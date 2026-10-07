package core

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/auth"
	"wzap/internal/storage"
)

const RequestIDHeader = "X-Request-Id"

type ContextKey int

const RequestIDKey ContextKey = iota

// RequestIDFromContext returns the request id attached by RequestID, or an
// empty string when the request did not pass through it.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(RequestIDKey).(string)
	return id
}

// RequestID echoes a caller supplied X-Request-Id or generates one, stores it
// in the request context and returns it in the response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), RequestIDKey, id)))
	})
}

// Logging emits one structured log line per request, skipping probes and docs.
func Logging(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if SkipAccessLog(r) {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			rec := &StatusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.Info().
				Str("request_id", RequestIDFromContext(r.Context())).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", rec.status).
				Int64("duration_ms", time.Since(start).Milliseconds()).
				Msg("http request")
		})
	}
}
func SkipAccessLog(r *http.Request) bool {
	if (r.Method == http.MethodGet) && (r.URL.Path == "/healthz" || r.URL.Path == "/readyz") {
		return true
	}
	if strings.HasPrefix(r.URL.Path, "/swagger/") {
		return true
	}
	return r.URL.Path == "/manager" || strings.HasPrefix(r.URL.Path, "/manager/")
}

// StatusRecorder captures the status code written by the inner handler so
// Logging can report it.
type StatusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *StatusRecorder) WriteHeader(status int) {
	if r.wrote {
		return
	}
	r.wrote = true
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (r *StatusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// Recover converts a handler panic into a 500 error envelope. The panic value
// and stack are logged server-side and never written to the client.
func Recover(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}
				log.Error().
					Str("request_id", RequestIDFromContext(r.Context())).
					Any("panic", recovered).
					Str("stack", string(debug.Stack())).
					Msg("panic recovered")
				Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Authenticate guards handlers with one credential per request, resolved in a
// fixed order with first hit winning: the wzap_session cookie validated with
// auth.ParseToken, the apikey header equal to the global key, then the hex
// sha256 of the apikey header looked up in the key repository. An unusable
// cookie (absent, unparseable, expired, wrong secret) counts as absent and
// falls through to the apikey steps; only when no step resolves does the
// middleware fail with 401. Authorization: Bearer is never
// read. The resolved auth.Scope is injected in the request context for
// downstream handlers via auth.ContextWithScope.
func Authenticate(globalKey string, keys storage.APIKeyRepository, jwtSecret string) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
				if scope, err := auth.ParseToken(cookie.Value, jwtSecret); err == nil {
					next.ServeHTTP(w, r.WithContext(auth.ContextWithScope(r.Context(), scope)))
					return
				}
			}

			if presented := r.Header.Get("apikey"); presented != "" {
				if globalKey != "" && subtle.ConstantTimeCompare([]byte(presented), []byte(globalKey)) == 1 {
					next.ServeHTTP(w, r.WithContext(auth.ContextWithScope(r.Context(), auth.Scope{Kind: auth.ScopeGlobal})))
					return
				}
				if keys != nil {
					sum := sha256.Sum256([]byte(presented))
					id, err := keys.InstanceByHash(r.Context(), hex.EncodeToString(sum[:]))
					switch {
					case err == nil:
						next.ServeHTTP(w, r.WithContext(auth.ContextWithScope(r.Context(), auth.Scope{
							Kind:       auth.ScopeInstance,
							InstanceID: id,
						})))
						return
					case errors.Is(err, storage.ErrNotFound):

					default:
						Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
						return
					}
				}
			}

			Error(w, r, http.StatusUnauthorized, "unauthorized", "missing or invalid credential")
		})
	}
}
