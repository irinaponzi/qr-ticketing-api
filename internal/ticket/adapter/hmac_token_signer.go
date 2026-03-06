package adapter

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// HMACTokenSigner implements TokenSigner using HMAC-SHA256.
// The signed token format is: "code.signature"
type HMACTokenSigner struct {
	secret []byte
}

// NewHMACTokenSigner creates a new HMACTokenSigner with the given secret key.
func NewHMACTokenSigner(secret string) *HMACTokenSigner {
	return &HMACTokenSigner{secret: []byte(secret)}
}

// Sign returns a token in the format "code.hmac_hex".
func (s *HMACTokenSigner) Sign(code string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(code))
	sig := hex.EncodeToString(mac.Sum(nil))

	return code + "." + sig
}

// Verify splits the token into code and signature, recomputes the HMAC,
// and returns the original code if valid.
func (s *HMACTokenSigner) Verify(token string) (string, bool) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", false
	}

	code := parts[0]
	sig := parts[1]

	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(code))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", false
	}

	return code, true
}
