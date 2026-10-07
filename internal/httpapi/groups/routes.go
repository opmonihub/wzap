package groups

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
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/groups", HandleCreateGroup(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/groups/{group_id}", HandleGetGroup(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("PATCH", "/instances/{id}/groups/{group_id}", HandleUpdateGroup(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("PUT", "/instances/{id}/groups/{group_id}/photo", HandleSetGroupPhoto(deps.Instances, log, cfg.MaxMediaBytes))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/groups/{group_id}/participants", HandleUpdateGroupParticipants(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/groups/{group_id}/invite", HandleGetGroupInvite(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/groups/{group_id}/invite/reset", HandleResetGroupInvite(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/groups/join", HandleJoinGroup(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/groups/{group_id}/leave", HandleLeaveGroup(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/groups", HandleListJoinedGroups(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/groups/invite-preview", HandleInvitePreview(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/groups/{group_id}/requests", HandleGroupRequests(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).Method("POST", "/instances/{id}/groups/{group_id}/requests", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandleUpdateGroupRequests(deps.Instances, log)))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).Method("PATCH", "/instances/{id}/groups/{group_id}/settings", core.Idempotency(deps.Idempotency, log, cfg.MaxMediaBytes)(HandleUpdateGroupSettings(deps.Instances, log)))
}
