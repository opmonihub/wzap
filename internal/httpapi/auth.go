package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"wzap/internal/auth"
	"wzap/internal/model"
	"wzap/internal/storage"
)

// loginRequest is the POST /auth/login payload.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// identityResponse is the JSON representation of the authenticated manager
// user, shared by login and me.
type identityResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// logoutResponse is the POST /auth/logout payload.
type logoutResponse struct {
	Status string `json:"status"`
}

// meEnvelope nests the authenticated identity under data.me (response matrix
// §1): the session identity is not a resource keyed by the request, so it
// travels under its own named key instead of a flat data object.
type meEnvelope struct {
	Me identityResponse `json:"me"`
}

// dummyPasswordHash is a valid bcrypt hash burned on unknown emails so the
// 401 path costs one comparison like a wrong password, closing the
// timing oracle between "unknown email" (immediate) and "wrong password"
// (~60ms of bcrypt). The value is a hash of an undisclosed password and
// never matches a real login.
const dummyPasswordHash = "$2a$10$Q2IcRv3W7hJBMOYy5JOOU.3qrWu7zjQhOp9LqZzrTVLIZvVToCr/C"

// handleLogin verifies the credentials and answers 200 with the identity plus
// the session cookie. An unknown email and a wrong password share one 401
// response so neither field is revealed.
//
// @Summary Log in to the manager session
// @Description Verifies the email/password credentials and mints the wzap_session cookie. Unknown email and wrong password share one 401 response.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body loginRequest true "Credentials"
// @Success 200 {object} envelope{data=meEnvelope} "Identity, wrapped in the data envelope; the wzap_session cookie is set"
// @Header 200 {string} Set-Cookie "Sets the httpOnly wzap_session cookie"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Invalid credentials"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 429 {object} errorEnvelope "Login rate limit exceeded"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /auth/login [post]
func handleLogin(users storage.UserRepository, jwtSecret string, secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request loginRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}

		user, err := users.GetByEmail(r.Context(), request.Email)
		switch {
		case errors.Is(err, storage.ErrNotFound):
			// Burn one bcrypt comparison so unknown emails cost the same as
			// wrong passwords (see dummyPasswordHash).
			_ = auth.CheckPassword(dummyPasswordHash, request.Password)
			Error(w, r, http.StatusUnauthorized, "unauthorized", "invalid credentials")
			return
		case err != nil:
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		if err := auth.CheckPassword(user.PasswordHash, request.Password); err != nil {
			Error(w, r, http.StatusUnauthorized, "unauthorized", "invalid credentials")
			return
		}

		token, err := auth.MintToken(user.ID, user.Role, jwtSecret)
		if err != nil {
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		setSessionCookie(w, token, secure)
		JSON(w, r, http.StatusOK, meEnvelope{Me: newIdentityResponse(user)})
	}
}

// handleLogout clears the session cookie and answers 200. There are no
// server-side sessions, so revocation is the cleared cookie alone.
//
// @Summary Log out of the manager session
// @Description Clears the wzap_session cookie. There are no server-side sessions.
// @Tags auth
// @Produce json
// @Success 200 {object} envelope{data=logoutResponse} "Status, wrapped in the data envelope"
// @Header 200 {string} Set-Cookie "Clears the wzap_session cookie"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /auth/logout [post]
func handleLogout(secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clearSessionCookie(w, secure)
		JSON(w, r, http.StatusOK, logoutResponse{Status: "ok"})
	}
}

// handleMe answers 200 with the identity of the session cookie holder, 401
// without a valid session.
//
// @Summary Show the current manager session
// @Description Answers the identity of the wzap_session cookie holder, 401 without a valid session.
// @Tags auth
// @Produce json
// @Success 200 {object} envelope{data=meEnvelope} "Identity, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid session"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /auth/me [get]
func handleMe(users storage.UserRepository, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.SessionCookieName)
		if err != nil {
			Error(w, r, http.StatusUnauthorized, "unauthorized", "missing or invalid session")
			return
		}
		scope, err := auth.ParseToken(cookie.Value, jwtSecret)
		if err != nil {
			Error(w, r, http.StatusUnauthorized, "unauthorized", "missing or invalid session")
			return
		}

		user, err := users.GetByID(r.Context(), scope.UserID)
		switch {
		case errors.Is(err, storage.ErrNotFound):
			Error(w, r, http.StatusUnauthorized, "unauthorized", "missing or invalid session")
			return
		case err != nil:
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		JSON(w, r, http.StatusOK, meEnvelope{Me: newIdentityResponse(user)})
	}
}

// newIdentityResponse maps a stored user to its JSON representation. The
// password hash never leaves the storage boundary.
func newIdentityResponse(user *model.User) identityResponse {
	return identityResponse{ID: user.ID.String(), Email: user.Email, Role: user.Role}
}

// setSessionCookie stores the session token with the attributes the session
// contract requires: Path /, HttpOnly, SameSite Lax, a MaxAge matching the
// token lifetime, and Secure exactly when the public URL serves https.
func setSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.TokenLifetime.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie expires the session cookie with the same attributes, so
// the client drops the session it holds.
func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// secureCookies reports whether session cookies must carry the Secure flag:
// exactly when PublicURL serves https.
func secureCookies(publicURL string) bool {
	return strings.HasPrefix(strings.ToLower(publicURL), "https://")
}
