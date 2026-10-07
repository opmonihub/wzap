package core

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"wzap/internal/model"
)

type targetKey struct{}

func TargetInstance(r *http.Request) (*model.Instance, bool) {
	value, ok := r.Context().Value(targetKey{}).(*model.Instance)
	return value, ok
}
func LoadInstance(r *http.Request, reader InstanceReader, id uuid.UUID) (*model.Instance, error) {
	if target, ok := TargetInstance(r); ok && target.ID == id {
		return target, nil
	}
	return reader.Get(r.Context(), id)
}
func ResolveInstance(instances InstanceService, authenticated bool, parameter string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reference := PathParam(r, parameter)
			target, err := ResolveInstanceReference(r, instances, reference, authenticated)
			if err != nil {
				WriteInstanceError(w, r, err)
				return
			}
			if authenticated {
				if err := AuthorizeInstance(r, target); err != nil {
					WriteForbidden(w, r)
					return
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), targetKey{}, target)))
		})
	}
}
