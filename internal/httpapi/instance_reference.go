package httpapi

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"wzap/internal/auth"
	"wzap/internal/instance"
	"wzap/internal/model"
)

// instanceMux resolves aliases after ServeMux has matched and populated path
// values. Only instance registrations are wrapped; static and other IDs retain
// their existing handlers and validation order.
type instanceMux struct {
	*http.ServeMux
	instances InstanceService
}

func (m *instanceMux) Handle(pattern string, handler http.Handler) {
	if strings.Contains(pattern, "/instances/{id}") {
		handler = resolveInstancePath(m.instances, handler, true, true)
	}
	m.ServeMux.Handle(pattern, handler)
}

func (m *instanceMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	h := http.Handler(http.HandlerFunc(handler))
	if strings.Contains(pattern, "/instances/{id}") {
		h = resolveInstancePath(m.instances, h, true, false)
	}
	m.ServeMux.Handle(pattern, h)
}

// resolveInstancePath keeps ordinary UUID operations lazy. Keyed POSTs on
// idempotent registrations must authorize the current row before the cache
// can acquire or replay. Resolution changes PathValue only, preserving URL
// and route pattern for fingerprinting and transport logging.
func resolveInstancePath(instances InstanceService, next http.Handler, authenticated, idempotent bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reference := r.PathValue("id")
		id, err := uuid.Parse(reference)
		keyedReplay := idempotent && r.Method == http.MethodPost && strings.TrimSpace(r.Header.Get(idempotencyKeyHeader)) != ""
		if err == nil && !keyedReplay {
			r.SetPathValue("id", id.String())
			next.ServeHTTP(w, r)
			return
		}
		target, err := resolveInstanceReference(r, instances, reference, authenticated)
		if err != nil {
			writeInstanceError(w, r, err)
			return
		}
		if authenticated {
			if err := authorizeInstance(r, target); err != nil {
				writeForbidden(w, r)
				return
			}
		}
		r.SetPathValue("id", target.ID.String())
		next.ServeHTTP(w, r)
	})
}

func resolveInstanceReference(r *http.Request, instances InstanceService, reference string, authenticated bool) (*model.Instance, error) {
	if id, err := uuid.Parse(reference); err == nil {
		if authenticated {
			if scope, ok := auth.ScopeFromContext(r.Context()); ok && scope.Kind == auth.ScopeInstance && scope.InstanceID != id {
				return nil, auth.ErrForbidden
			}
		}
		return instances.Get(r.Context(), id)
	}
	if err := instance.ValidateInstanceName(reference); err != nil {
		return nil, instance.ErrNotFound
	}
	if authenticated {
		if scope, ok := auth.ScopeFromContext(r.Context()); ok && scope.Kind == auth.ScopeInstance {
			// An instance key resolves its own row, then compares the exact current
			// name. It never queries a foreign name's existence or ambiguity.
			own, err := instances.Get(r.Context(), scope.InstanceID)
			if err != nil {
				return nil, err
			}
			if own.Name != reference {
				return nil, auth.ErrForbidden
			}
			return own, nil
		}
	}
	return instances.GetByName(r.Context(), reference)
}
