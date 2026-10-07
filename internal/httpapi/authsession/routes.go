package authsession

import (
	"github.com/go-chi/chi/v5"

	"wzap/internal/storage"
)

func Register(r chi.Router, users storage.UserRepository, secret string, secure bool, limiter *LoginRateLimiter) {
	if limiter == nil {
		limiter = NewLoginRateLimiter(DefaultLoginLimit, 0)
	}
	r.Method("POST", "/auth/login", LoginLimit(limiter, HandleLogin(users, secret, secure)))
	r.MethodFunc("POST", "/auth/logout", HandleLogout(secure))
	r.MethodFunc("GET", "/auth/me", HandleMe(users, secret))
}
