package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testToken = "test-service-token"

func decodeJSON(t *testing.T, b []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(b, target); err != nil {
		t.Fatal(err)
	}
}
func serve(t *testing.T, srv *http.Server, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("apikey", token)
	}
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	return rec
}
func dataField(t *testing.T, b []byte, field string) string {
	t.Helper()
	var data struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	decodeJSON(t, b, &data)
	var value string
	decodeJSON(t, data.Data[field], &value)
	return value
}
