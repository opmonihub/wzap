package channels

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
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/newsletters/follow", HandleFollowNewsletter(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/newsletters/unfollow", HandleUnfollowNewsletter(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/newsletters/{channel}", HandleGetNewsletter(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/newsletters", HandleListNewsletters(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/newsletters/{channel}/messages", HandleGetNewsletterMessages(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/newsletters/{channel}/updates", HandleGetNewsletterUpdates(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).Method("POST", "/instances/{id}/newsletters", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandleCreateNewsletter(deps.Instances, log)))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).Method("POST", "/instances/{id}/newsletters/{channel}/mute", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandleMuteNewsletter(deps.Instances, log)))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).Method("POST", "/instances/{id}/newsletters/{channel}/viewed", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandleMarkNewsletterViewed(deps.Instances, log)))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).Method("POST", "/instances/{id}/newsletters/{channel}/reactions", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandleReactNewsletter(deps.Instances, log)))
}
