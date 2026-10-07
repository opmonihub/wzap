package contacts

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
	Numbers     NumberResolver
	Lookup      core.InstanceService
}

func Register(r chi.Router, cfg config.Config, log zerolog.Logger, deps Deps) {
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/numbers/check", HandleCheckNumber(deps.Instances, deps.Numbers))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/contacts/check", HandleCheckContacts(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/contacts/{jid}/devices", HandleContactDevices(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/contacts/{jid}/photo", HandleContactPhoto(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/contacts/{jid}/business", HandleContactBusiness(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/contacts/{jid}/subscribe", HandleSubscribePresence(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/contact-link", HandleContactLink(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/blocklist", HandleGetBlocklist(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).Method("POST", "/instances/{id}/blocklist", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandleUpdateBlocklist(deps.Instances, log)))
}
