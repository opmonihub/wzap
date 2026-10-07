package config

import (
	"strings"
	"testing"
)

func testTokenKey(t *testing.T) []byte {
	t.Helper()
	key := []byte("0123456789abcdef0123456789abcdef")
	if len(key) != 32 {
		t.Fatalf("test key length = %d, want 32", len(key))
	}
	return key
}

func TestSealTokenRoundtrip(t *testing.T) {
	key := testTokenKey(t)
	sealed, err := SealToken("chatwoot-secret-token", key)
	if err != nil {
		t.Fatalf("SealToken: %v", err)
	}
	if !strings.HasPrefix(sealed, SealedTokenPrefix) {
		t.Errorf("sealed = %q, want prefix %q", sealed, SealedTokenPrefix)
	}
	if strings.Contains(sealed, "chatwoot-secret-token") {
		t.Errorf("sealed %q contains the plaintext", sealed)
	}
	opened, err := OpenToken(sealed, key)
	if err != nil {
		t.Fatalf("OpenToken: %v", err)
	}
	if opened != "chatwoot-secret-token" {
		t.Errorf("opened = %q, want the plaintext", opened)
	}
}

func TestSealTokenRandomizesNonce(t *testing.T) {
	key := testTokenKey(t)
	first, err := SealToken("same-token", key)
	if err != nil {
		t.Fatalf("SealToken: %v", err)
	}
	second, err := SealToken("same-token", key)
	if err != nil {
		t.Fatalf("SealToken: %v", err)
	}
	if first == second {
		t.Error("two seals are identical, want a fresh random nonce per seal")
	}
}

func TestOpenTokenRejectsWrongKey(t *testing.T) {
	sealed, err := SealToken("chatwoot-secret-token", testTokenKey(t))
	if err != nil {
		t.Fatalf("SealToken: %v", err)
	}
	if _, err := OpenToken(sealed, []byte("fedcba9876543210fedcba9876543210")); err == nil {
		t.Error("OpenToken with wrong key = nil, want authentication failure")
	}
}

func TestOpenTokenRejectsTamperedCiphertext(t *testing.T) {
	sealed, err := SealToken("chatwoot-secret-token", testTokenKey(t))
	if err != nil {
		t.Fatalf("SealToken: %v", err)
	}
	tampered := sealed[:len(sealed)-2] + "AA"
	if _, err := OpenToken(tampered, testTokenKey(t)); err == nil {
		t.Error("OpenToken tampered = nil, want authentication failure")
	}
}

// TestOpenTokenRejectsLegacyPlaintext pins the mandatory-cipher contract:
// empty tokens read back as empty (disabled configs carry none) and any
// non-empty stored value must carry the sealed envelope — legacy plaintext
// rows no longer read through.
func TestOpenTokenRejectsLegacyPlaintext(t *testing.T) {
	key := testTokenKey(t)
	got, err := OpenToken("", key)
	if err != nil {
		t.Fatalf("OpenToken(\"\") error = %v, want empty passthrough", err)
	}
	if got != "" {
		t.Errorf("OpenToken(\"\") = %q, want empty", got)
	}
	if _, err := OpenToken("legacy-plaintext-token", key); err == nil {
		t.Error("OpenToken(legacy plaintext) = nil, want failure (plaintext storage removed)")
	}
	if _, err := OpenToken("legacy-plaintext-token", nil); err == nil {
		t.Error("OpenToken(legacy plaintext, nil key) = nil, want failure (plaintext storage removed)")
	}
}

func TestSealTokenPassesEmptyThrough(t *testing.T) {
	got, err := SealToken("", testTokenKey(t))
	if err != nil {
		t.Fatalf("SealToken empty: %v", err)
	}
	if got != "" {
		t.Errorf("SealToken empty = %q, want empty (disabled configs carry no token)", got)
	}
}

func TestSealTokenNeverDoubleSeals(t *testing.T) {
	key := testTokenKey(t)
	sealed, err := SealToken("chatwoot-secret-token", key)
	if err != nil {
		t.Fatalf("SealToken: %v", err)
	}
	again, err := SealToken(sealed, key)
	if err != nil {
		t.Fatalf("SealToken sealed: %v", err)
	}
	if again != sealed {
		t.Error("SealToken sealed the sealed value again, want idempotent passthrough")
	}
}

func TestSealTokenRejectsBadKey(t *testing.T) {
	for _, key := range [][]byte{nil, {}, []byte("short")} {
		if _, err := SealToken("token", key); err == nil {
			t.Errorf("SealToken key len %d = nil, want key size error", len(key))
		}
		if _, err := OpenToken(SealedTokenPrefix+"AAAA", key); err == nil {
			t.Errorf("OpenToken key len %d = nil, want key size error", len(key))
		}
	}
}
