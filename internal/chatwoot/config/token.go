package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

// SealedTokenPrefix marks a stored Chatwoot token as AES-GCM ciphertext. The
// token column keeps holding text, so sealed values need no schema change;
// non-empty stored values always carry the prefix — plaintext storage has
// no compatibility mode.
const SealedTokenPrefix = "enc:v1:"

// SealToken encrypts a Chatwoot token with key (exactly 32 bytes, AES-256)
// and returns the prefixed base64(nonce|ciphertext) form for storage. Empty
// tokens pass through (disabled configs carry none), as do values that
// already carry the prefix, so Put-after-Get can never double-seal.
func SealToken(plaintext string, key []byte) (string, error) {
	if plaintext == "" || strings.HasPrefix(plaintext, SealedTokenPrefix) {
		return plaintext, nil
	}
	aead, err := tokenAEAD(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("seal chatwoot token: %w", err)
	}
	sealed := aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return SealedTokenPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// OpenToken recovers the plaintext of a stored Chatwoot token with key.
// Empty tokens read back empty (disabled configs carry none); every
// non-empty stored value must be the authenticated envelope and fails
// closed on tampering, a wrong key, or a legacy plaintext row.
func OpenToken(stored string, key []byte) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, SealedTokenPrefix) {
		return "", fmt.Errorf("open chatwoot token: legacy plaintext token without the sealed envelope")
	}
	aead, err := tokenAEAD(key)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, SealedTokenPrefix))
	if err != nil {
		return "", fmt.Errorf("open chatwoot token: %w", err)
	}
	nonceSize := aead.NonceSize()
	if len(raw) <= nonceSize {
		return "", fmt.Errorf("open chatwoot token: truncated ciphertext")
	}
	plaintext, err := aead.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("open chatwoot token: %w", err)
	}
	return string(plaintext), nil
}

// tokenAEAD builds the AES-256-GCM primitive over the 32-byte token key.
func tokenAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("chatwoot token key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("chatwoot token key: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("chatwoot token key: %w", err)
	}
	return aead, nil
}
