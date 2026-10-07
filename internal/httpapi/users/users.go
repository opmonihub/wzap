package users

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"wzap/internal/auth"
	"wzap/internal/httpapi/core"
	"wzap/internal/model"
	"wzap/internal/storage"
)

// UserResponse is the public representation of a manager user (matrix §5):
// identity, role, the per-user instance limit (0 = unlimited), the
// backend-computed count of owned instances and the timestamps. Password
// hashes never leave the storage boundary.
type UserResponse struct {
	ID            string    `json:"id" binding:"required"`
	Email         string    `json:"email" binding:"required"`
	Role          string    `json:"role" binding:"required"`
	InstanceLimit int       `json:"instance_limit" binding:"required"`
	InstancesUsed int       `json:"instances_used" binding:"required"`
	CreatedAt     time.Time `json:"created_at" binding:"required"`
	UpdatedAt     time.Time `json:"updated_at" binding:"required"`
}

// UserResponseEnvelope wraps a user under data.user (matrix §1) and nests
// each item of the /users collection under its own key.
type UserResponseEnvelope struct {
	User UserResponse `json:"user" binding:"required"`
}

// UserListResponse is the authorized user collection.
type UserListResponse struct {
	Users []UserResponse `json:"users" binding:"required"`
}

// NewUserResponse maps a stored user to its public DTO with the instances
// count supplied by the caller.
func NewUserResponse(user *model.User, instancesUsed int) UserResponse {
	return UserResponse{
		ID:            user.ID.String(),
		Email:         user.Email,
		Role:          user.Role,
		InstanceLimit: user.InstanceQuota,
		InstancesUsed: instancesUsed,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}
}

// OptionalQuota captures the presence of the instance_limit field
// independently of its JSON type: a *json.RawMessage cannot tell an absent
// field from an explicit null (both decode to nil), while this value type
// records every occurrence — including null — via UnmarshalJSON, so an
// explicit null can be rejected with 422 instead of taking the default.
type OptionalQuota struct {
	Present bool
	Raw     json.RawMessage
}

// UnmarshalJSON marks the field present and keeps its raw bytes for the
// handler to validate as an integer quota.
func (o *OptionalQuota) UnmarshalJSON(data []byte) error {
	o.Present = true
	o.Raw = append(o.Raw[:0], data...)
	return nil
}

// CreateUserRequest is the POST /users payload. The limit uses optionalQuota
// so an absent field (creation-time default) stays distinct from an explicit
// value, and a non-integer value can be rejected with 422 instead of the
// generic 400 of a body decoding failure.
type CreateUserRequest struct {
	Email         string        `json:"email"`
	Password      string        `json:"password"`
	Role          string        `json:"role"`
	InstanceLimit OptionalQuota `json:"instance_limit" swaggertype:"integer" minimum:"0"`
}

// HandleCreateUser registers a manager user and answers 201 with the user.
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
// @Param request body CreateUserRequest true "User payload"
// @Success 201 {object} core.Envelope{data=UserResponseEnvelope} "Created user, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Requires global or admin scope"
// @Failure 409 {object} core.ErrorEnvelope "Email already taken"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid email, password, role or quota"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /users [post]
func HandleCreateUser(users storage.UserRepository, defaultQuota int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := core.RequireAdminScope(r); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		var request CreateUserRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		email := strings.TrimSpace(request.Email)
		if email == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid email")
			return
		}
		if request.Password == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid password")
			return
		}
		if request.Role != "admin" && request.Role != "user" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid role")
			return
		}
		limit := defaultQuota
		if request.InstanceLimit.Present {
			var parsed *int
			if err := json.Unmarshal(request.InstanceLimit.Raw, &parsed); err != nil || parsed == nil || *parsed < 0 {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid instance_limit")
				return
			}
			limit = *parsed
		}

		if users == nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		hash, err := auth.HashPassword(request.Password)
		if err != nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
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
				core.Error(w, r, http.StatusConflict, "conflict", "email already taken")
				return
			}
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		core.JSON(w, r, http.StatusCreated, UserResponseEnvelope{User: NewUserResponse(created, 0)})
	}
}

// HandleListUsers answers every user in created_at order. Only the global
// scope and admin sessions may list; any other scope answers 403.
//
// @Summary List users
// @Tags users
// @Produce json
// @Security apikey
// @Success 200 {object} core.Envelope{data=UserListResponse} "Users, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Requires global or admin scope"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /users [get]
func HandleListUsers(users storage.UserRepository, keys storage.APIKeyRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := core.RequireAdminScope(r); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		if users == nil || keys == nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		stored, err := users.List(r.Context())
		if err != nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		items := make([]UserResponse, 0, len(stored))
		for i := range stored {
			used, err := keys.CountByOwner(r.Context(), stored[i].ID)
			if err != nil {
				core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
				return
			}
			items = append(items, NewUserResponse(&stored[i], used))
		}
		core.JSON(w, r, http.StatusOK, UserListResponse{Users: items})
	}
}

// HandleGetUser answers one user by id. Only the global scope and admin
// sessions may read; any other scope answers 403 before the target is read,
// so non-admin callers cannot probe user ids. A malformed id and an unknown
// user answer 404.
//
// @Summary Get a user
// @Tags users
// @Produce json
// @Security apikey
// @Param id path string true "User ID (UUID)"
// @Success 200 {object} core.Envelope{data=UserResponseEnvelope} "User, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Requires global or admin scope"
// @Failure 404 {object} core.ErrorEnvelope "User not found"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /users/{id} [get]
func HandleGetUser(users storage.UserRepository, keys storage.APIKeyRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := core.RequireAdminScope(r); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		id, err := uuid.Parse(core.PathParam(r, "id"))
		if err != nil {
			core.Error(w, r, http.StatusNotFound, "not_found", "user not found")
			return
		}

		if users == nil || keys == nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		stored, err := users.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				core.Error(w, r, http.StatusNotFound, "not_found", "user not found")
				return
			}
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		used, err := keys.CountByOwner(r.Context(), stored.ID)
		if err != nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		core.JSON(w, r, http.StatusOK, UserResponseEnvelope{User: NewUserResponse(stored, used)})
	}
}

// HandleDeleteUser removes a user and answers 204. Only the global scope
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
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Requires global or admin scope"
// @Failure 404 {object} core.ErrorEnvelope "User not found"
// @Failure 409 {object} core.ErrorEnvelope "User still owns instances"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /users/{id} [delete]
func HandleDeleteUser(users storage.UserRepository, keys storage.APIKeyRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := core.RequireAdminScope(r); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		id, err := uuid.Parse(core.PathParam(r, "id"))
		if err != nil {
			core.Error(w, r, http.StatusNotFound, "not_found", "user not found")
			return
		}

		if users == nil || keys == nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		owned, err := keys.CountByOwner(r.Context(), id)
		if err != nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if owned > 0 {
			core.Error(w, r, http.StatusConflict, "conflict", "user still owns instances")
			return
		}

		if err := users.Delete(r.Context(), id); err != nil {
			switch {
			case errors.Is(err, storage.ErrNotFound):
				core.Error(w, r, http.StatusNotFound, "not_found", "user not found")
			case IsForeignKeyViolation(err):
				core.Error(w, r, http.StatusConflict, "conflict", "user still owns instances")
			default:
				core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}
		core.JSON(w, r, http.StatusNoContent, nil)
	}
}

// IsForeignKeyViolation reports whether err wraps a Postgres foreign-key
// violation (SQLSTATE 23503), covering the check-then-delete race where an
// instance row lands after the CountByOwner pre-check.
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// PatchQuotaRequest distinguishes an omitted limit from an explicit null,
// which clears the limit to unlimited. Other non-integer values answer 422.
type PatchQuotaRequest struct {
	InstanceLimit OptionalQuota `json:"instance_limit" swaggertype:"integer" minimum:"0"`
}

// HandleUpdateUserQuota edits the per-user instance quota and answers 200
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
// @Param request body PatchQuotaRequest true "Quota payload"
// @Success 200 {object} core.Envelope{data=UserResponseEnvelope} "Updated user, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Requires global or admin scope"
// @Failure 404 {object} core.ErrorEnvelope "User not found"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid instance quota"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /users/{id} [patch]
func HandleUpdateUserQuota(users storage.UserRepository, keys storage.APIKeyRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := core.RequireAdminScope(r); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		id, err := uuid.Parse(core.PathParam(r, "id"))
		if err != nil {
			core.Error(w, r, http.StatusNotFound, "not_found", "user not found")
			return
		}

		var request PatchQuotaRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		if !request.InstanceLimit.Present {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid instance_limit")
			return
		}
		var limit *int
		if err := json.Unmarshal(request.InstanceLimit.Raw, &limit); err != nil || (limit != nil && *limit < 0) {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid instance_limit")
			return
		}

		if users == nil || keys == nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		quota := 0
		if limit != nil {
			quota = *limit
		}
		if err := users.UpdateQuota(r.Context(), id, quota); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				core.Error(w, r, http.StatusNotFound, "not_found", "user not found")
				return
			}
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		updated, err := users.GetByID(r.Context(), id)
		if err != nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		used, err := keys.CountByOwner(r.Context(), updated.ID)
		if err != nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		core.JSON(w, r, http.StatusOK, UserResponseEnvelope{User: NewUserResponse(updated, used)})
	}
}
