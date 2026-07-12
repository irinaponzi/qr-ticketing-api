package middleware

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

type contextKey string

const authClaimsKey contextKey = "auth_claims"

// AuthClaims holds the identity and group membership extracted from a validated JWT.
type AuthClaims struct {
	Username string
	Groups   []string
}

// HasRole reports whether the caller belongs to the given Cognito group.
func (c *AuthClaims) HasRole(role string) bool {
	for _, g := range c.Groups {
		if g == role {
			return true
		}
	}
	return false
}

// ClaimsFromContext retrieves the AuthClaims stored by the auth middleware.
func ClaimsFromContext(ctx context.Context) (*AuthClaims, bool) {
	c, ok := ctx.Value(authClaimsKey).(*AuthClaims)
	return c, ok
}

// TokenValidator validates a raw JWT string and returns the extracted claims.
type TokenValidator interface {
	Validate(ctx context.Context, token string) (*AuthClaims, error)
}

// RequireAuth is middleware that accepts any authenticated caller regardless of role.
func RequireAuth(v TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := extractBearer(r, v)
			if err != nil {
				respondAuthErr(w, http.StatusUnauthorized, err.Error())
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authClaimsKey, claims)))
		})
	}
}

// RequireRole is middleware that validates the JWT and enforces Cognito group membership.
func RequireRole(v TokenValidator, role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := extractBearer(r, v)
			if err != nil {
				respondAuthErr(w, http.StatusUnauthorized, err.Error())
				return
			}
			if !claims.HasRole(role) {
				respondAuthErr(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authClaimsKey, claims)))
		})
	}
}

func extractBearer(r *http.Request, v TokenValidator) (*AuthClaims, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return nil, errors.New("authorization header required")
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return nil, errors.New("authorization header must be Bearer <token>")
	}
	return v.Validate(r.Context(), parts[1])
}

func respondAuthErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// --- Cognito JWT validator ---

// jwksHTTPClient is the HTTP client used to fetch JWKS from Cognito.
// Exposed as a variable so tests can replace it with a test-server-aware client.
var jwksHTTPClient = &http.Client{Timeout: 5 * time.Second}

type jwksKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type keyCache struct {
	mu   sync.RWMutex
	keys map[string]*rsa.PublicKey
	url  string
}

func (c *keyCache) get(kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	key, ok := c.keys[kid]
	c.mu.RUnlock()
	if ok {
		return key, nil
	}
	return c.refresh(kid)
}

func (c *keyCache) refresh(kid string) (*rsa.PublicKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	resp, err := jwksHTTPClient.Get(c.url)
	if err != nil {
		return nil, fmt.Errorf("fetching JWKS: %w", err)
	}
	defer resp.Body.Close()

	var jwks struct {
		Keys []jwksKey `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("decoding JWKS: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}
		pub, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		newKeys[k.Kid] = pub
	}
	c.keys = newKeys

	key, ok := newKeys[kid]
	if !ok {
		return nil, fmt.Errorf("kid %q not found in JWKS", kid)
	}
	return key, nil
}

func parseRSAPublicKey(n, e string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, fmt.Errorf("decoding n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, fmt.Errorf("decoding e: %w", err)
	}
	nBig := new(big.Int).SetBytes(nBytes)
	eBig := new(big.Int).SetBytes(eBytes)
	return &rsa.PublicKey{N: nBig, E: int(eBig.Int64())}, nil
}

// CognitoValidator validates RS256 JWTs issued by an AWS Cognito User Pool.
// JWKS public keys are fetched lazily on first use and cached in memory.
// On key rotation, an unknown kid triggers a single JWKS refresh.
type CognitoValidator struct {
	issuer string
	cache  *keyCache
}

// NewCognitoValidator creates a validator targeting the given Cognito User Pool.
func NewCognitoValidator(region, userPoolID string) *CognitoValidator {
	issuer := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, userPoolID)
	return &CognitoValidator{
		issuer: issuer,
		cache: &keyCache{
			url:  issuer + "/.well-known/jwks.json",
			keys: make(map[string]*rsa.PublicKey),
		},
	}
}

// Validate parses and verifies a Cognito-issued access or ID token.
func (v *CognitoValidator) Validate(_ context.Context, tokenString string) (*AuthClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("invalid token header")
	}
	var header struct {
		Kid string `json:"kid"`
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, errors.New("invalid token header")
	}
	if header.Alg != "RS256" {
		return nil, fmt.Errorf("unsupported algorithm %q: only RS256 accepted", header.Alg)
	}

	pubKey, err := v.cache.get(header.Kid)
	if err != nil {
		return nil, fmt.Errorf("resolving signing key: %w", err)
	}

	sigInput := parts[0] + "." + parts[1]
	digest := sha256.Sum256([]byte(sigInput))
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("invalid token signature encoding")
	}
	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, digest[:], sigBytes); err != nil {
		return nil, errors.New("token signature verification failed")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid token payload")
	}
	var payload struct {
		Iss      string   `json:"iss"`
		Sub      string   `json:"sub"`
		Username string   `json:"cognito:username"`
		Groups   []string `json:"cognito:groups"`
		TokenUse string   `json:"token_use"`
		Exp      int64    `json:"exp"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, errors.New("invalid token payload")
	}

	if payload.Iss != v.issuer {
		return nil, errors.New("token issuer mismatch")
	}
	if time.Now().Unix() > payload.Exp {
		return nil, errors.New("token expired")
	}
	if payload.TokenUse != "access" && payload.TokenUse != "id" {
		return nil, fmt.Errorf("unexpected token_use %q", payload.TokenUse)
	}

	username := payload.Username
	if username == "" {
		username = payload.Sub
	}

	return &AuthClaims{Username: username, Groups: payload.Groups}, nil
}
