package profile

import (
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/httpapi/core"
)

type Deps struct {
	Instances InstanceService
	Lookup    core.InstanceService
}

func Register(r chi.Router, cfg config.Config, log zerolog.Logger, deps Deps) {
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/profile", HandleGetProfile(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("PATCH", "/instances/{id}/profile", HandleUpdateProfile(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("PUT", "/instances/{id}/profile/photo", HandleSetProfilePhoto(deps.Instances, log, cfg.MaxMediaBytes))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/privacy", HandleGetPrivacy(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("PUT", "/instances/{id}/privacy", HandleSetPrivacy(deps.Instances, log))
}
