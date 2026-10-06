package session

import "testing"

func TestNeedsFreshPairing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		reason string
		want   bool
	}{
		{"", false},
		{SessionRejectedReason, true},
		{"logged out: 401", true},
		{"stream replaced", true},
		{"temporary network blip", false},
	}
	for _, tc := range cases {
		if got := NeedsFreshPairing(tc.reason); got != tc.want {
			t.Errorf("NeedsFreshPairing(%q) = %v, want %v", tc.reason, got, tc.want)
		}
	}
}
