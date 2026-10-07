package instances

import (
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/httpapi/core"
	"wzap/internal/storage"
)

type Deps struct {
	ChatwootConfigs ChatwootConfigStore
	Instances       InstanceService
	Keys            storage.APIKeyRepository
	PublicURL       string
	Users           storage.UserRepository
	Lookup          core.InstanceService
}

func Register(r chi.Router, cfg config.Config, log zerolog.Logger, deps Deps) {
	instanceAgg := InstanceConfigAggregator{instances: deps.Instances, configs: deps.ChatwootConfigs, publicURL: deps.PublicURL, log: log}
	r.MethodFunc("POST", "/instances", HandleCreateInstance(instanceAgg, deps.Users, deps.Keys, cfg.MaxInstances))
	r.MethodFunc("GET", "/instances/stats", HandleInstanceStats(deps.Instances))
	r.MethodFunc("GET", "/instances", HandleListInstances(instanceAgg))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}", HandleGetInstance(instanceAgg))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("PATCH", "/instances/{id}", HandleUpdateInstance(instanceAgg))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("DELETE", "/instances/{id}", HandleDeleteInstance(deps.Instances))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/apikey/rotate", HandleRotateAPIKey(deps.Instances, deps.Keys))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("DELETE", "/instances/{id}/apikey", HandleRevokeAPIKey(deps.Instances, deps.Keys))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/connect", HandleConnectInstance(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/disconnect", HandleDisconnectInstance(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/qr", HandleQRInstance(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/status", HandleInstanceStatus(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/presence", HandleSendPresence(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/pair-phone", HandlePairPhone(deps.Instances, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/calls/reject", HandleRejectCall(deps.Instances, log))
}
