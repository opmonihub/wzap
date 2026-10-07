package media

import (
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/httpapi/core"
)

type Deps struct {
	Instances InstanceService
	Media     MediaStore
	Lookup    core.InstanceService
}

func Register(r chi.Router, cfg config.Config, log zerolog.Logger, deps Deps) {
	r.MethodFunc("GET", "/media/{id}", HandleGetMedia(deps.Instances, deps.Media))
}
