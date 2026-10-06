package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"wzap/internal/auth"
	"wzap/internal/model"
	"wzap/internal/storage"
)

// userResponse is the public representation of a manager user (matrix §5):
// identity, role, the per-user instance limit (0 = unlimited), the
// backend-computed count of owned instances and the timestamps. Password
// hashes never leave the storage boundary.
type userResponse struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	Role          string    `json:"role"`
	InstanceLimit int       `json:"instance_limit"`
	InstancesUsed int       `json:"instances_used"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// userResponseEnvelope wraps a user under data.user (matrix §1) and nests
// each item of the /users collection under its own key.
type userResponseEnvelope struct {
	User userResponse `json:"user"`
}

// userListResponse is the authorized user collection.
type userListResponse struct {
	Items []userResponseEnvelope `json:"items"`
}

// newUserResponse maps a stored user to its public DTO with the instances
// count supplied by the caller.
func newUserResponse(user *model.User, instancesUsed int) userResponse {
	return userResponse{
		ID:            user.ID.String(),
		Email:         user.Email,
		Role:          user.Role,
		InstanceLimit: user.InstanceQuota,
		InstancesUsed: instancesUsed,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}
}

// optionalQuota captures the presence of the instance_limit field
// independently of its JSON type: a *json.RawMessage cannot tell an absent
// field from an explicit null (both decode to nil), while this value type
// records every occurrence — including null — via UnmarshalJSON, so an
// explicit null can be rejected with 422 instead of taking the default.
type optionalQuota struct {
	Present bool
	Raw     json.RawMessage
}

// UnmarshalJSON marks the field present and keeps its raw bytes for the
// handler to validate as an integer quota.
func (o *optionalQuota) UnmarshalJSON(data []byte) error {
	o.Present = true
	o.Raw = append(o.Raw[:0], data...)
	return nil
}

// createUserRequest is the POST /users payload. The limit uses optionalQuota
// so an absent field (creation-time default) stays distinct from an explicit
// value, and a non-integer value can be rejected with 422 instead of the
// generic 400 of a body decoding failure.
type createUserRequest struct {
	Email         string        `json:"email"`
	Password      string        `json:"password"`
	Role          string        `json:"role"`
	InstanceLimit optionalQuota `json:"instance_limit" swaggertype:"integer" minimum:"0"`
}

// handleCreateUser registers a manager user and answers 201 with the user.
// Only the global scope and admin sessions may create; any other scope
// answers 403 before the body is read. There is no public registration: an
// unauthenticated request never reaches this handler (the middleware answers
// 401). An empty email or password, a role outside admin|user, or a missing,
// non-integer or negative quota answers 422; a malformed body answers 400. A
// missing quota applies defaultQuota (WZAP_DEFAULT_USER_INSTANCE_QUOTA); an
// explicit value, including 0 (unlimited), is used verbatim. A duplicate
// email answers 409.
//
// @Summary Create a user
// @Tags users
// @Accept json
// @Produce json
// @Security apikey
// @Param request body createUserRequest true "User payload"
// @Success 201 {object} envelope{data=userResponseEnvelope} "Created user, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Requires global or admin scope"
// @Failure 409 {object} errorEnvelope "Email already taken"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid email, password, role or quota"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /users [post]
func handleCreateUser(users storage.UserRepository, defaultQuota int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := requireAdminScope(r); err != nil {
			writeForbidden(w, r)
			return
		}

		var request createUserRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		email := strings.TrimSpace(request.Email)
		if email == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid email")
			return
		}
		if request.Password == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid password")
			return
		}
		if request.Role != "admin" && request.Role != "user" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid role")
			return
		}
		limit := defaultQuota
		if request.InstanceLimit.Present {
			var parsed *int
			if err := json.Unmarshal(request.InstanceLimit.Raw, &parsed); err != nil || parsed == nil || *parsed < 0 {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid instance_limit")
				return
			}
			limit = *parsed
		}

		if users == nil {
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		hash, err := auth.HashPassword(request.Password)
		if err != nil {
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		created, err := users.Create(r.Context(), model.User{
			ID:            uuid.New(),
			Email:         email,
			PasswordHash:  hash,
			Role:          request.Role,
			InstanceQuota: limit,
		})
		if err != nil {
			if errors.Is(err, storage.ErrEmailTaken) {
				Error(w, r, http.StatusConflict, "conflict", "email already taken")
				return
			}
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		JSON(w, r, http.StatusCreated, userResponseEnvelope{User: newUserResponse(created, 0)})
	}
}

// handleListUsers answers every user in created_at order. Only the global
// scope and admin sessions may list; any other scope answers 403.
//
// @Summary List users
// @Tags users
// @Produce json
// @Security apikey
// @Success 200 {object} envelope{data=userListResponse} "Users, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Requires global or admin scope"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /users [get]
func handleListUsers(users storage.UserRepository, keys storage.APIKeyRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := requireAdminScope(r); err != nil {
			writeForbidden(w, r)
			return
		}

		if users == nil || keys == nil {
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		stored, err := users.List(r.Context())
		if err != nil {
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		items := make([]userResponseEnvelope, 0, len(stored))
		for i := range stored {
			used, err := keys.CountByOwner(r.Context(), stored[i].ID)
			if err != nil {
				Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
				return
			}
			items = append(items, userResponseEnvelope{User: newUserResponse(&stored[i], used)})
		}
		JSON(w, r, http.StatusOK, userListResponse{Items: items})
	}
}

// handleGetUser answers one user by id. Only the global scope and admin
// sessions may read; any other scope answers 403 before the target is read,
// so non-admin callers cannot probe user ids. A malformed id and an unknown
// user answer 404.
//
// @Summary Get a user
// @Tags users
// @Produce json
// @Security apikey
// @Param id path string true "User ID (UUID)"
// @Success 200 {object} envelope{data=userResponseEnvelope} "User, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Requires global or admin scope"
// @Failure 404 {object} errorEnvelope "User not found"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /users/{id} [get]
func handleGetUser(users storage.UserRepository, keys storage.APIKeyRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := requireAdminScope(r); err != nil {
			writeForbidden(w, r)
			return
		}

		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			Error(w, r, http.StatusNotFound, "not_found", "user not found")
			return
		}

		if users == nil || keys == nil {
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		stored, err := users.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				Error(w, r, http.StatusNotFound, "not_found", "user not found")
				return
			}
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		used, err := keys.CountByOwner(r.Context(), stored.ID)
		if err != nil {
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		JSON(w, r, http.StatusOK, userResponseEnvelope{User: newUserResponse(stored, used)})
	}
}

// handleDeleteUser removes a user and answers 204. Only the global scope
// and admin sessions may delete; any other scope answers 403 before the
// target is read. A malformed id and an unknown user answer 404. An owner
// that still owns instances answers 409 and nothing is removed, via two
// layers: a deterministic CountByOwner pre-check before the delete, and a
// foreign-key race cover mapping a 23503 violation from Delete to 409 as
// well. There is no transfer and no cascade, by omission.
//
// @Summary Delete a user
// @Tags users
// @Produce json
// @Security apikey
// @Param id path string true "User ID (UUID)"
// @Success 204 "Deleted, no body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Requires global or admin scope"
// @Failure 404 {object} errorEnvelope "User not found"
// @Failure 409 {object} errorEnvelope "User still owns instances"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /users/{id} [delete]
func handleDeleteUser(users storage.UserRepository, keys storage.APIKeyRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := requireAdminScope(r); err != nil {
			writeForbidden(w, r)
			return
		}

		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			Error(w, r, http.StatusNotFound, "not_found", "user not found")
			return
		}

		if users == nil || keys == nil {
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		owned, err := keys.CountByOwner(r.Context(), id)
		if err != nil {
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if owned > 0 {
			Error(w, r, http.StatusConflict, "conflict", "user still owns instances")
			return
		}

		if err := users.Delete(r.Context(), id); err != nil {
			switch {
			case errors.Is(err, storage.ErrNotFound):
				Error(w, r, http.StatusNotFound, "not_found", "user not found")
			case isForeignKeyViolation(err):
				Error(w, r, http.StatusConflict, "conflict", "user still owns instances")
			default:
				Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}
		JSON(w, r, http.StatusNoContent, nil)
	}
}

// isForeignKeyViolation reports whether err wraps a Postgres foreign-key
// violation (SQLSTATE 23503), covering the check-then-delete race where an
// instance row lands after the CountByOwner pre-check.
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// patchQuotaRequest is the PATCH /users/{id} payload. The limit arrives as a
// raw message so a non-integer value (string, float, boolean, null) can be
// rejected with 422 instead of the generic 400 of a body decoding failure.
type patchQuotaRequest struct {
	InstanceLimit *json.RawMessage `json:"instance_limit" swaggertype:"integer" minimum:"0"`
}

// handleUpdateUserQuota edits the per-user instance quota and answers 200
// with the updated user. Only the global scope and admin sessions may edit;
// any other scope answers 403 before the target is read, so non-admin callers
// cannot probe user ids. A malformed id and an unknown user answer 404; a
// missing, non-integer or negative quota answers 422. Unknown JSON fields are
// ignored, matching the shared body decoder.
//
// @Summary Update a user quota
// @Tags users
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "User ID (UUID)"
// @Param request body patchQuotaRequest true "Quota payload"
// @Success 200 {object} envelope{data=userResponseEnvelope} "Updated user, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Requires global or admin scope"
// @Failure 404 {object} errorEnvelope "User not found"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid instance quota"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /users/{id} [patch]
func handleUpdateUserQuota(users storage.UserRepository, keys storage.APIKeyRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := requireAdminScope(r); err != nil {
			writeForbidden(w, r)
			return
		}

		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			Error(w, r, http.StatusNotFound, "not_found", "user not found")
			return
		}

		var request patchQuotaRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		if request.InstanceLimit == nil {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid instance_limit")
			return
		}
		var limit *int
		if err := json.Unmarshal(*request.InstanceLimit, &limit); err != nil || limit == nil || *limit < 0 {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid instance_limit")
			return
		}

		if users == nil || keys == nil {
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if err := users.UpdateQuota(r.Context(), id, *limit); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				Error(w, r, http.StatusNotFound, "not_found", "user not found")
				return
			}
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		updated, err := users.GetByID(r.Context(), id)
		if err != nil {
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		used, err := keys.CountByOwner(r.Context(), updated.ID)
		if err != nil {
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		JSON(w, r, http.StatusOK, userResponseEnvelope{User: newUserResponse(updated, used)})
	}
}
