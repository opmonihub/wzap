package chatwoot

import (
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/httpapi/core"
)

type Deps struct {
	PublicURL         string
	Chatwoot          config.Chatwoot
	ChatwootClientFor ChatwootClientFor
	ChatwootConfigs   ChatwootConfigStore
	ChatwootImporter  ChatwootImporter
	ChatwootInbound   ChatwootInbound
	Instances         InstanceService
	Lookup            core.InstanceService
}

func Register(r chi.Router, cfg config.Config, log zerolog.Logger, deps Deps) {
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("PUT", "/instances/{id}/chatwoot", HandleChatwootSet(deps.Instances, deps.ChatwootConfigs, deps.Chatwoot, deps.PublicURL, deps.ChatwootClientFor, log))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("GET", "/instances/{id}/chatwoot", HandleChatwootGet(deps.Instances, deps.ChatwootConfigs, deps.Chatwoot, deps.PublicURL))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/chatwoot/import", HandleChatwootImport(deps.Instances, deps.ChatwootConfigs, deps.Chatwoot, deps.ChatwootImporter))
	r.With(core.ResolveInstance(deps.Lookup, true, "id")).MethodFunc("POST", "/instances/{id}/chatwoot/command", HandleChatwootCommand(deps.Instances, deps.ChatwootConfigs, deps.Chatwoot, deps.ChatwootInbound))
}
