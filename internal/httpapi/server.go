package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	httpSwagger "github.com/swaggo/http-swagger"

	_ "wzap/docs"
	"wzap/internal/config"
	"wzap/internal/httpapi/authsession"
	"wzap/internal/httpapi/channels"
	"wzap/internal/httpapi/chats"
	"wzap/internal/httpapi/chatwoot"
	"wzap/internal/httpapi/contacts"
	"wzap/internal/httpapi/core"
	"wzap/internal/httpapi/groups"
	"wzap/internal/httpapi/instances"
	"wzap/internal/httpapi/media"
	"wzap/internal/httpapi/messages"
	"wzap/internal/httpapi/profile"
	"wzap/internal/httpapi/statuses"
	"wzap/internal/httpapi/users"
	"wzap/internal/storage"
	"wzap/manager"
)

type ReadyChecker interface{ Check(context.Context) error }

type InstanceService interface {
	instances.InstanceService
	contacts.InstanceService
	messages.InstanceService
	groups.InstanceService
	channels.InstanceService
	statuses.InstanceService
	chats.InstanceService
	profile.InstanceService
	media.InstanceService
	chatwoot.InstanceService
	core.InstanceService
}
type Deps struct {
	ReadyChecker           ReadyChecker
	Instances              InstanceService
	Numbers                contacts.NumberResolver
	Messages               messages.MessageService
	Idempotency            storage.IdempotencyRepository
	Media                  media.MediaStore
	Users                  storage.UserRepository
	Keys                   storage.APIKeyRepository
	JWTSecret              string
	ChatwootConfigs        chatwoot.ChatwootConfigStore
	Chatwoot               config.Chatwoot
	PublicURL              string
	ChatwootInbound        chatwoot.ChatwootInbound
	ChatwootClientFor      chatwoot.ChatwootClientFor
	ChatwootImporter       chatwoot.ChatwootImporter
	ChatwootWebhookLimiter *chatwoot.ChatwootRateLimiter
	LoginLimiter           *authsession.LoginRateLimiter
}

func New(cfg config.Config, log zerolog.Logger, deps Deps) *http.Server {
	r := newRouter(cfg, log, deps)
	return &http.Server{Addr: cfg.HTTPAddr, Handler: r, ReadHeaderTimeout: 10 * time.Second}
}
func newRouter(cfg config.Config, log zerolog.Logger, deps Deps) *chi.Mux {
	r := chi.NewRouter()
	r.Use(core.RequestID, core.Logging(log), core.Recover(log), chimiddleware.GetHead)
	authenticate := core.Authenticate(cfg.APIKey, deps.Keys, deps.JWTSecret)
	r.Use(func(next http.Handler) http.Handler {
		protected := authenticate(next)
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			for _, prefix := range []string{"/instances", "/users", "/media"} {
				if req.URL.Path == prefix || strings.HasPrefix(req.URL.Path, prefix+"/") {
					protected.ServeHTTP(w, req)
					return
				}
			}
			next.ServeHTTP(w, req)
		})
	})
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		core.Error(w, req, 404, "not_found", "route not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		allowed := []string{}
		path := req.URL.Path
		if req.URL.RawPath != "" {
			path = req.URL.RawPath
		}
		for _, method := range []string{"DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"} {
			if r.Match(chi.NewRouteContext(), method, path) || (method == http.MethodHead && r.Match(chi.NewRouteContext(), http.MethodGet, path)) {
				allowed = append(allowed, method)
			}
		}
		if len(allowed) > 0 {
			w.Header().Set("Allow", strings.Join(allowed, ", "))
		}
		core.Error(w, req, 405, "method_not_allowed", "method not allowed")
	})
	r.Get("/healthz", handleHealthz)
	r.Get("/readyz", handleReadyz(deps.ReadyChecker, log))
	r.Handle("/swagger/*", httpSwagger.WrapHandler)
	r.Handle("/manager/*", manager.Handler())
	r.Get("/manager", manager.Handler().ServeHTTP)
	authsession.Register(r, deps.Users, deps.JWTSecret, authsession.SecureCookies(cfg.PublicURL), deps.LoginLimiter)
	publicURL := deps.PublicURL
	if publicURL == "" {
		publicURL = cfg.PublicURL
	}
	instances.Register(r, cfg, log, instances.Deps{ChatwootConfigs: deps.ChatwootConfigs, Instances: deps.Instances, Keys: deps.Keys, PublicURL: publicURL, Users: deps.Users, Lookup: deps.Instances})
	contacts.Register(r, cfg, log, contacts.Deps{Idempotency: deps.Idempotency, Instances: deps.Instances, Numbers: deps.Numbers, Lookup: deps.Instances})
	messages.Register(r, cfg, log, messages.Deps{Idempotency: deps.Idempotency, Instances: deps.Instances, Media: deps.Media, Messages: deps.Messages, Lookup: deps.Instances})
	groups.Register(r, cfg, log, groups.Deps{Idempotency: deps.Idempotency, Instances: deps.Instances, Lookup: deps.Instances})
	channels.Register(r, cfg, log, channels.Deps{Idempotency: deps.Idempotency, Instances: deps.Instances, Lookup: deps.Instances})
	statuses.Register(r, cfg, log, statuses.Deps{Idempotency: deps.Idempotency, Instances: deps.Instances, Lookup: deps.Instances})
	chats.Register(r, cfg, log, chats.Deps{Idempotency: deps.Idempotency, Instances: deps.Instances, Lookup: deps.Instances})
	profile.Register(r, cfg, log, profile.Deps{Instances: deps.Instances, Lookup: deps.Instances})
	media.Register(r, cfg, log, media.Deps{Instances: deps.Instances, Media: deps.Media, Lookup: deps.Instances})
	users.Register(r, cfg, log, users.Deps{Keys: deps.Keys, Users: deps.Users, Lookup: deps.Instances})
	chatwoot.Register(r, cfg, log, chatwoot.Deps{PublicURL: publicURL, Chatwoot: deps.Chatwoot, ChatwootClientFor: deps.ChatwootClientFor, ChatwootConfigs: deps.ChatwootConfigs, ChatwootImporter: deps.ChatwootImporter, ChatwootInbound: deps.ChatwootInbound, Instances: deps.Instances, Lookup: deps.Instances})
	limiter := deps.ChatwootWebhookLimiter
	if limiter == nil {
		limiter = chatwoot.NewChatwootRateLimiter(120, 0)
	}
	r.With(core.ResolveInstance(deps.Instances, false, "id")).Post("/chatwoot/webhook/{id}", chatwoot.HandleChatwootWebhook(deps.Instances, deps.ChatwootInbound, deps.Chatwoot, limiter))
	return r
}
