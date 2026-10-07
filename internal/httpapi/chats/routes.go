package chats

import (
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/httpapi/core"
	"wzap/internal/storage"
)

type Deps struct {
	Idempotency storage.IdempotencyRepository
	Instances   InstanceService
	Lookup      core.InstanceService
}

func Register(r chi.Router, cfg config.Config, log zerolog.Logger, deps Deps) {
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/chats/mark-read", HandleMarkRead(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/chats/{chat}/disappearing", HandleGetDisappearing(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).Method("PUT", "/instances/{id}/chats/{chat}/disappearing", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandleSetDisappearing(deps.Instances, log)))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).Method("PUT", "/instances/{id}/chats/default-disappearing", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandleSetDefaultDisappearing(deps.Instances, log)))
}
