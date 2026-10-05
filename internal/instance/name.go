package instance

import "wzap/internal/model"

// ValidateInstanceName checks the exact, case-sensitive URL-safe name contract.
// No normalization is applied; UUID-parsable names and stats are reserved.
func ValidateInstanceName(name string) error {
	if !model.IsValidInstanceName(name) {
		return ErrInvalidInstanceName
	}
	return nil
}
