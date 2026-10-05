package model

import "github.com/google/uuid"

// IsValidInstanceName checks the exact, case-sensitive URL-safe name contract.
// No normalization is applied; UUID-parsable names and stats are reserved.
func IsValidInstanceName(name string) bool {
	if len(name) < 1 || len(name) > 64 || name == "stats" {
		return false
	}
	if _, err := uuid.Parse(name); err == nil {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		alnum := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
		if !alnum && (i == 0 || i == len(name)-1 || c != '_' && c != '-') {
			return false
		}
	}
	return true
}
