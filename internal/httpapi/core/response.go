package core

import (
	"encoding/json"
	"net/http"
)

// Envelope is the success response shape for standard REST JSON responses.
type Envelope struct {
	Data any `json:"data" binding:"required"`
}

// ErrorEnvelope is the failure response shape for standard REST JSON errors.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error" binding:"required"`
}
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSON writes data as a JSON envelope with the given status, echoing the
// request id like Error so every success carries X-Request-Id even outside
// the RequestID middleware. No-content responses are written without a body.
func JSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	if id := RequestIDFromContext(r.Context()); id != "" {
		w.Header().Set(RequestIDHeader, id)
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Data: data})
}

// Error writes an error envelope with the given status and code, echoing the
// request id when the request carries one.
func Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if id := RequestIDFromContext(r.Context()); id != "" {
		w.Header().Set(RequestIDHeader, id)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorEnvelope{Error: ErrorBody{Code: code, Message: message}})
}
