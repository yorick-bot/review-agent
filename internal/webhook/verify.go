package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

var ErrInvalidSignature = errors.New("invalid signature")

// Verify checks the X-Hub-Signature-256 header against the request body
// using constant-time comparison.
func Verify(secret []byte, sigHeader string, body []byte) error {
	if !strings.HasPrefix(sigHeader, "sha256=") {
		return ErrInvalidSignature
	}
	given, err := hex.DecodeString(strings.TrimPrefix(sigHeader, "sha256="))
	if err != nil {
		return ErrInvalidSignature
	}
	m := hmac.New(sha256.New, secret)
	m.Write(body)
	if !hmac.Equal(given, m.Sum(nil)) {
		return ErrInvalidSignature
	}
	return nil
}
