package messages

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
	Media       MediaStore
	Messages    MessageService
	Lookup      core.InstanceService
}

func Register(r chi.Router, cfg config.Config, log zerolog.Logger, deps Deps) {
	r.With(core.ResolveInstance(deps.Lookup, true, "instance")).Method("POST", "/instances/{instance}/messages/text", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandleSendText(deps.Instances, deps.Messages)))
	r.With(core.ResolveInstance(deps.Lookup, true, "instance")).Method("POST", "/instances/{instance}/messages/location", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandleSendLocation(deps.Instances, deps.Messages)))
	r.With(core.ResolveInstance(deps.Lookup, true, "instance")).Method("POST", "/instances/{instance}/messages/contact", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandleSendContact(deps.Instances, deps.Messages)))
	r.With(core.ResolveInstance(deps.Lookup, true, "instance")).Method("POST", "/instances/{instance}/messages/media", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandleSendMedia(deps.Instances, deps.Messages, deps.Media, cfg.MaxMediaBytes)))
	r.With(core.ResolveInstance(deps.Lookup, true, "instance")).Method("POST", "/instances/{instance}/messages", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandleSendMessage(deps.Instances, deps.Messages)))
	r.With(core.ResolveInstance(deps.Lookup, true, "instance")).MethodFunc("GET", "/instances/{instance}/messages", HandleListMessages(deps.Instances, deps.Messages))
	r.With(core.ResolveInstance(deps.Lookup, true, "instance")).MethodFunc("GET", "/instances/{instance}/messages/{id}", HandleGetMessage(deps.Instances, deps.Messages))
	r.With(core.ResolveInstance(deps.Lookup, true, "instance")).MethodFunc("POST", "/instances/{instance}/messages/revoke", HandleRevokeMessage(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "instance")).Method("POST", "/instances/{instance}/messages/edit", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandleEditMessage(deps.Instances, log)))
}
