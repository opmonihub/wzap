package core

import (
	"net/http"
	"net/url"
)

// PathParam returns a decoded route segment, including escaped delimiters.
func PathParam(r *http.Request, name string) string {
	value := r.PathValue(name)
	if r.URL.RawPath != "" {
		if decoded, err := url.PathUnescape(value); err == nil {
			return decoded
		}
	}
	return value
}
