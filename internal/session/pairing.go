package session

import "strings"

// SessionRejectedReason is the actionable last_error recorded when WhatsApp
// opens the socket but rejects the linked device before login completes (stale
// device logged out on the phone, or another client took over the session).
const SessionRejectedReason = "connection dropped by WhatsApp before login completed (socket opened but the session was rejected): the device is stale (the phone logged out this device) or another client took over the session; pair again with POST /instances/{id}/connect"

// NeedsFreshPairing reports whether lastError means stored credentials cannot
// be reused and the instance must pair again (reset device store + clear bind).
func NeedsFreshPairing(lastError string) bool {
	if lastError == "" {
		return false
	}
	if strings.Contains(lastError, SessionRejectedReason) {
		return true
	}
	switch {
	case strings.Contains(lastError, "logged out:"),
		strings.Contains(lastError, "stream replaced"),
		strings.Contains(lastError, "device jid mismatch"),
		strings.Contains(lastError, "device jid already bound"):
		return true
	}
	return false
}
