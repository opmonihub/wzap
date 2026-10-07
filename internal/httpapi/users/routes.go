package users

import (
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/httpapi/core"
	"wzap/internal/storage"
)

type Deps struct {
	Keys   storage.APIKeyRepository
	Users  storage.UserRepository
	Lookup core.InstanceService
}

func Register(r chi.Router, cfg config.Config, log zerolog.Logger, deps Deps) {
	r.MethodFunc("POST", "/users", HandleCreateUser(deps.Users, cfg.DefaultUserQuota))
	r.MethodFunc("GET", "/users", HandleListUsers(deps.Users, deps.Keys))
	r.MethodFunc("GET", "/users/{id}", HandleGetUser(deps.Users, deps.Keys))
	r.MethodFunc("DELETE", "/users/{id}", HandleDeleteUser(deps.Users, deps.Keys))
	r.MethodFunc("PATCH", "/users/{id}", HandleUpdateUserQuota(deps.Users, deps.Keys))
}
