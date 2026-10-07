package core

import (
	"net/http"

	"github.com/google/uuid"

	"wzap/internal/auth"
	"wzap/internal/instance"
	"wzap/internal/model"
)

func ResolveInstanceReference(r *http.Request, instances InstanceService, reference string, authenticated bool) (*model.Instance, error) {
	if id, err := uuid.Parse(reference); err == nil {
		if authenticated {
			if scope, ok := auth.ScopeFromContext(r.Context()); ok && scope.Kind == auth.ScopeInstance && scope.InstanceID != id {
				return nil, auth.ErrForbidden
			}
		}
		return LoadInstance(r, instances, id)
	}
	if err := instance.ValidateInstanceName(reference); err != nil {
		return nil, instance.ErrNotFound
	}
	if authenticated {
		if scope, ok := auth.ScopeFromContext(r.Context()); ok && scope.Kind == auth.ScopeInstance {

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
