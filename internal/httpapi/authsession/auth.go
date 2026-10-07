package authsession

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"wzap/internal/auth"
	"wzap/internal/httpapi/core"
	"wzap/internal/model"
	"wzap/internal/storage"
)

// LoginRequest is the POST /auth/login payload.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// IdentityResponse is the JSON representation of the authenticated manager
// user, shared by login and me.
type IdentityResponse struct {
	ID    string `json:"id" binding:"required"`
	Email string `json:"email" binding:"required"`
	Role  string `json:"role" binding:"required"`
}

// LogoutResponse is the POST /auth/logout payload.
type LogoutResponse struct {
	Status string `json:"status" binding:"required"`
}

// MeEnvelope nests the authenticated identity under data.me (response matrix
// §1): the session identity is not a resource keyed by the request, so it
// travels under its own named key instead of a flat data object.
type MeEnvelope struct {
	Me IdentityResponse `json:"me" binding:"required"`
}

// DummyPasswordHash is a valid bcrypt hash burned on unknown emails so the
// 401 path costs one comparison like a wrong password, closing the
// timing oracle between "unknown email" (immediate) and "wrong password"
// (~60ms of bcrypt). The value is a hash of an undisclosed password and
// never matches a real login.
const DummyPasswordHash = "$2a$10$Q2IcRv3W7hJBMOYy5JOOU.3qrWu7zjQhOp9LqZzrTVLIZvVToCr/C"

// HandleLogin verifies the credentials and answers 200 with the identity plus
// the session cookie. An unknown email and a wrong password share one 401
// response so neither field is revealed.
//
// @Summary Log in to the manager session
// @Description Verifies the email/password credentials and mints the wzap_session cookie. Unknown email and wrong password share one 401 response.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Credentials"
// @Success 200 {object} core.Envelope{data=MeEnvelope} "Identity, wrapped in the data envelope; the wzap_session cookie is set"
// @Header 200 {string} Set-Cookie "Sets the httpOnly wzap_session cookie"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Invalid credentials"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 429 {object} core.ErrorEnvelope "Login rate limit exceeded"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /auth/login [post]
func HandleLogin(users storage.UserRepository, jwtSecret string, secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request LoginRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}

		user, err := users.GetByEmail(r.Context(), request.Email)
		switch {
		case errors.Is(err, storage.ErrNotFound):

			_ = auth.CheckPassword(DummyPasswordHash, request.Password)
			core.Error(w, r, http.StatusUnauthorized, "unauthorized", "invalid credentials")
			return
		case err != nil:
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		if err := auth.CheckPassword(user.PasswordHash, request.Password); err != nil {
			core.Error(w, r, http.StatusUnauthorized, "unauthorized", "invalid credentials")
			return
		}

		token, err := auth.MintToken(user.ID, user.Role, jwtSecret)
		if err != nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		SetSessionCookie(w, token, secure)
		core.JSON(w, r, http.StatusOK, MeEnvelope{Me: NewIdentityResponse(user)})
	}
}

// HandleLogout clears the session cookie and answers 200. There are no
// server-side sessions, so revocation is the cleared cookie alone.
//
// @Summary Log out of the manager session
// @Description Clears the wzap_session cookie. There are no server-side sessions.
// @Tags auth
// @Produce json
// @Success 200 {object} core.Envelope{data=LogoutResponse} "Status, wrapped in the data envelope"
// @Header 200 {string} Set-Cookie "Clears the wzap_session cookie"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /auth/logout [post]
func HandleLogout(secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ClearSessionCookie(w, secure)
		core.JSON(w, r, http.StatusOK, LogoutResponse{Status: "ok"})
	}
}

// HandleMe answers 200 with the identity of the session cookie holder, 401
// without a valid session.
//
// @Summary Show the current manager session
// @Description Answers the identity of the wzap_session cookie holder, 401 without a valid session.
// @Tags auth
// @Produce json
// @Success 200 {object} core.Envelope{data=MeEnvelope} "Identity, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid session"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /auth/me [get]
func HandleMe(users storage.UserRepository, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.SessionCookieName)
		if err != nil {
			core.Error(w, r, http.StatusUnauthorized, "unauthorized", "missing or invalid session")
			return
		}
		scope, err := auth.ParseToken(cookie.Value, jwtSecret)
		if err != nil {
			core.Error(w, r, http.StatusUnauthorized, "unauthorized", "missing or invalid session")
			return
		}

		user, err := users.GetByID(r.Context(), scope.UserID)
		switch {
		case errors.Is(err, storage.ErrNotFound):
			core.Error(w, r, http.StatusUnauthorized, "unauthorized", "missing or invalid session")
			return
		case err != nil:
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		core.JSON(w, r, http.StatusOK, MeEnvelope{Me: NewIdentityResponse(user)})
	}
}

// NewIdentityResponse maps a stored user to its JSON representation. The
// password hash never leaves the storage boundary.
func NewIdentityResponse(user *model.User) IdentityResponse {
	return IdentityResponse{ID: user.ID.String(), Email: user.Email, Role: user.Role}
}

// SetSessionCookie stores the session token with the attributes the session
// contract requires: Path /, HttpOnly, SameSite Lax, a MaxAge matching the
// token lifetime, and Secure exactly when the public URL serves https.
func SetSessionCookie(w http.ResponseWriter, token string, secure bool) {
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

// ClearSessionCookie expires the session cookie with the same attributes, so
// the client drops the session it holds.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
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

// SecureCookies reports whether session cookies must carry the Secure flag:
// exactly when PublicURL serves https.
func SecureCookies(publicURL string) bool {
	return strings.HasPrefix(strings.ToLower(publicURL), "https://")
}
