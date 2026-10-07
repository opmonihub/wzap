package core

import (
	"net/http/httptest"
	"testing"
)

func TestPathParamDecodesOnce(t *testing.T) {
	for _, tc := range []struct{ path, value, want string }{{"/x/a%40b", "a%40b", "a@b"}, {"/x/a%252Fb", "a%2Fb", "a%2Fb"}, {"/x/name", "name", "name"}} {
		r := httptest.NewRequest("GET", tc.path, nil)
		r.SetPathValue("segment", tc.value)
		if got := PathParam(r, "segment"); got != tc.want {
			t.Errorf("%s = %q want %q", tc.path, got, tc.want)
		}
	}
}
