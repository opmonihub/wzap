package statuses

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
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/status/privacy", HandleGetStatusPrivacy(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).Method("POST", "/instances/{id}/status/updates", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandlePublishStatus(deps.Instances, log)))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).Method("POST", "/instances/{id}/status/updates/media", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandlePublishStatusMedia(deps.Instances, log, cfg.MaxMediaBytes)))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/status/updates", HandleListStatuses(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("DELETE", "/instances/{id}/status/updates/{status_id}", HandleDeleteStatus(deps.Instances, log))
}
