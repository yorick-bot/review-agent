package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

func sign(secret, body []byte) string {
	m := hmac.New(sha256.New, secret)
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func TestVerify(t *testing.T) {
	secret := []byte("s3cret")
	body := []byte(`{"hello":"world"}`)
	good := sign(secret, body)

	cases := []struct {
		name string
		sig  string
		body []byte
		want error
	}{
		{"valid", good, body, nil},
		{"missing header", "", body, ErrInvalidSignature},
		{"wrong prefix", "sha1=deadbeef", body, ErrInvalidSignature},
		{"malformed hex", "sha256=zz", body, ErrInvalidSignature},
		{"tampered body", good, []byte(`{"hello":"WORLD"}`), ErrInvalidSignature},
		{"wrong secret", sign([]byte("other"), body), body, ErrInvalidSignature},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Verify(secret, tc.sig, tc.body)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}
