package middleware

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- test helpers ---

func generateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	return k
}

func bigToBase64URL(n *big.Int) string {
	return base64.RawURLEncoding.EncodeToString(n.Bytes())
}

func intToBase64URL(e int) string {
	return base64.RawURLEncoding.EncodeToString(big.NewInt(int64(e)).Bytes())
}

// jwksServer starts an httptest server serving the public key as JWKS.
func jwksServer(t *testing.T, kid string, pub *rsa.PublicKey) *httptest.Server {
	t.Helper()
	data := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kid": kid, "kty": "RSA", "alg": "RS256", "use": "sig",
				"n": bigToBase64URL(pub.N), "e": intToBase64URL(pub.E),
			},
		},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(data)
	}))
}

type cognitoClaims struct {
	Iss      string   `json:"iss"`
	Sub      string   `json:"sub"`
	Username string   `json:"cognito:username"`
	Groups   []string `json:"cognito:groups"`
	TokenUse string   `json:"token_use"`
	Exp      int64    `json:"exp"`
}

// buildJWT creates a signed RS256 JWT for testing.
func buildJWT(t *testing.T, kid string, key *rsa.PrivateKey, claims cognitoClaims) string {
	t.Helper()
	hdr, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": kid})
	pld, _ := json.Marshal(claims)

	h64 := base64.RawURLEncoding.EncodeToString(hdr)
	p64 := base64.RawURLEncoding.EncodeToString(pld)
	digest := sha256.Sum256([]byte(h64 + "." + p64))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("rsa.SignPKCS1v15: %v", err)
	}
	return h64 + "." + p64 + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func newCognitoValidatorFor(issuer, jwksURL string) *CognitoValidator {
	return &CognitoValidator{
		issuer: issuer,
		cache:  &keyCache{url: jwksURL, keys: make(map[string]*rsa.PublicKey)},
	}
}

// --- AuthClaims tests ---

func TestAuthClaims_HasRole(t *testing.T) {
	c := &AuthClaims{Groups: []string{"admin", "editor"}}
	if !c.HasRole("admin") {
		t.Error("HasRole(admin) = false, want true")
	}
	if c.HasRole("user") {
		t.Error("HasRole(user) = true, want false")
	}
}

func TestClaimsFromContext(t *testing.T) {
	want := &AuthClaims{Username: "alice", Groups: []string{"admin"}}
	ctx := context.WithValue(context.Background(), authClaimsKey, want)

	got, ok := ClaimsFromContext(ctx)
	if !ok || got != want {
		t.Errorf("ClaimsFromContext: got (%v, %v), want (%v, true)", got, ok, want)
	}

	_, ok = ClaimsFromContext(context.Background())
	if ok {
		t.Error("ClaimsFromContext on empty context: want false")
	}
}

// --- Middleware tests ---

type mockValidator struct {
	claims *AuthClaims
	err    error
}

func (m *mockValidator) Validate(_ context.Context, _ string) (*AuthClaims, error) {
	return m.claims, m.err
}

func newRequest(authHeader string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/test", nil)
	if authHeader != "" {
		r.Header.Set("Authorization", authHeader)
	}
	return r
}

func TestRequireAuth(t *testing.T) {
	validClaims := &AuthClaims{Username: "alice", Groups: []string{"user"}}

	tests := []struct {
		name       string
		header     string
		mock       *mockValidator
		wantStatus int
	}{
		{"missing header", "", &mockValidator{}, http.StatusUnauthorized},
		{"bad scheme", "Token abc", &mockValidator{claims: validClaims}, http.StatusUnauthorized},
		{"invalid token", "Bearer bad", &mockValidator{err: errors.New("bad token")}, http.StatusUnauthorized},
		{"valid", "Bearer good", &mockValidator{claims: validClaims}, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			w := httptest.NewRecorder()
			RequireAuth(tt.mock)(next).ServeHTTP(w, newRequest(tt.header))
			if w.Code != tt.wantStatus {
				t.Errorf("status: got %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestRequireAuth_ClaimsStoredInContext(t *testing.T) {
	want := &AuthClaims{Username: "bob", Groups: []string{"admin"}}
	mock := &mockValidator{claims: want}

	var got *AuthClaims
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	RequireAuth(mock)(next).ServeHTTP(w, newRequest("Bearer sometoken"))

	if got != want {
		t.Errorf("claims in context: got %v, want %v", got, want)
	}
}

func TestRequireRole(t *testing.T) {
	adminClaims := &AuthClaims{Username: "admin-user", Groups: []string{"admin"}}
	userClaims := &AuthClaims{Username: "regular", Groups: []string{"user"}}

	tests := []struct {
		name       string
		mock       *mockValidator
		role       string
		wantStatus int
	}{
		{"no auth header", &mockValidator{}, RoleAdmin, http.StatusUnauthorized},
		{"wrong role", &mockValidator{claims: userClaims}, RoleAdmin, http.StatusForbidden},
		{"correct role", &mockValidator{claims: adminClaims}, RoleAdmin, http.StatusOK},
		{"user role match", &mockValidator{claims: userClaims}, RoleUser, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var header string
			if tt.mock.claims != nil || tt.mock.err != nil {
				header = "Bearer token"
			}
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			w := httptest.NewRecorder()
			RequireRole(tt.mock, tt.role)(next).ServeHTTP(w, newRequest(header))
			if w.Code != tt.wantStatus {
				t.Errorf("status: got %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

// --- CognitoValidator tests ---

func TestCognitoValidator_ValidAccessToken(t *testing.T) {
	kid := "key-1"
	key := generateKey(t)
	srv := jwksServer(t, kid, &key.PublicKey)
	defer srv.Close()

	issuer := "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_Test"
	v := newCognitoValidatorFor(issuer, srv.URL)

	token := buildJWT(t, kid, key, cognitoClaims{
		Iss:      issuer,
		Sub:      "uuid-123",
		Username: "alice",
		Groups:   []string{"admin"},
		TokenUse: "access",
		Exp:      time.Now().Add(time.Hour).Unix(),
	})

	claims, err := v.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Username != "alice" {
		t.Errorf("username: got %q, want %q", claims.Username, "alice")
	}
	if !claims.HasRole("admin") {
		t.Error("expected admin role")
	}
}

func TestCognitoValidator_IDToken(t *testing.T) {
	kid := "key-1"
	key := generateKey(t)
	srv := jwksServer(t, kid, &key.PublicKey)
	defer srv.Close()

	issuer := "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_Test"
	v := newCognitoValidatorFor(issuer, srv.URL)

	token := buildJWT(t, kid, key, cognitoClaims{
		Iss:      issuer,
		Sub:      "uuid-456",
		Username: "bob",
		Groups:   []string{"user"},
		TokenUse: "id",
		Exp:      time.Now().Add(time.Hour).Unix(),
	})

	claims, err := v.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error for id token: %v", err)
	}
	if claims.Username != "bob" {
		t.Errorf("username: got %q, want %q", claims.Username, "bob")
	}
}

func TestCognitoValidator_FallsBackToSubWhenNoUsername(t *testing.T) {
	kid := "key-1"
	key := generateKey(t)
	srv := jwksServer(t, kid, &key.PublicKey)
	defer srv.Close()

	issuer := "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_Test"
	v := newCognitoValidatorFor(issuer, srv.URL)

	token := buildJWT(t, kid, key, cognitoClaims{
		Iss:      issuer,
		Sub:      "fallback-sub",
		TokenUse: "access",
		Exp:      time.Now().Add(time.Hour).Unix(),
	})

	claims, err := v.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Username != "fallback-sub" {
		t.Errorf("username: got %q, want %q", claims.Username, "fallback-sub")
	}
}

func TestCognitoValidator_ExpiredToken(t *testing.T) {
	kid := "key-1"
	key := generateKey(t)
	srv := jwksServer(t, kid, &key.PublicKey)
	defer srv.Close()

	issuer := "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_Test"
	v := newCognitoValidatorFor(issuer, srv.URL)

	token := buildJWT(t, kid, key, cognitoClaims{
		Iss:      issuer,
		Sub:      "uuid",
		TokenUse: "access",
		Exp:      time.Now().Add(-time.Minute).Unix(),
	})

	_, err := v.Validate(context.Background(), token)
	if err == nil || err.Error() != "token expired" {
		t.Errorf("expected 'token expired', got %v", err)
	}
}

func TestCognitoValidator_WrongIssuer(t *testing.T) {
	kid := "key-1"
	key := generateKey(t)
	srv := jwksServer(t, kid, &key.PublicKey)
	defer srv.Close()

	issuer := "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_Test"
	v := newCognitoValidatorFor(issuer, srv.URL)

	token := buildJWT(t, kid, key, cognitoClaims{
		Iss:      "https://evil.example.com/pool",
		Sub:      "uuid",
		TokenUse: "access",
		Exp:      time.Now().Add(time.Hour).Unix(),
	})

	_, err := v.Validate(context.Background(), token)
	if err == nil || err.Error() != "token issuer mismatch" {
		t.Errorf("expected issuer mismatch error, got %v", err)
	}
}

func TestCognitoValidator_TamperedSignature(t *testing.T) {
	kid := "key-1"
	key := generateKey(t)
	srv := jwksServer(t, kid, &key.PublicKey)
	defer srv.Close()

	issuer := "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_Test"
	v := newCognitoValidatorFor(issuer, srv.URL)

	token := buildJWT(t, kid, key, cognitoClaims{
		Iss:      issuer,
		Sub:      "uuid",
		TokenUse: "access",
		Exp:      time.Now().Add(time.Hour).Unix(),
	})
	// corrupt last character of signature
	token = token[:len(token)-1] + "X"

	_, err := v.Validate(context.Background(), token)
	if err == nil {
		t.Error("expected signature error, got nil")
	}
}

func TestCognitoValidator_MalformedToken(t *testing.T) {
	issuer := "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_Test"
	v := newCognitoValidatorFor(issuer, "http://unused")

	_, err := v.Validate(context.Background(), "not.a.valid.jwt.at.all")
	if err == nil {
		t.Error("expected error for malformed token")
	}

	_, err = v.Validate(context.Background(), "onlytwoparts")
	if err == nil || err.Error() != "invalid token format" {
		t.Errorf("expected 'invalid token format', got %v", err)
	}
}

func TestCognitoValidator_UnknownKid(t *testing.T) {
	key := generateKey(t)
	// Server serves a key with kid "real-key", token uses "unknown-key"
	srv := jwksServer(t, "real-key", &key.PublicKey)
	defer srv.Close()

	issuer := "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_Test"
	v := newCognitoValidatorFor(issuer, srv.URL)

	token := buildJWT(t, "unknown-key", key, cognitoClaims{
		Iss:      issuer,
		Sub:      "uuid",
		TokenUse: "access",
		Exp:      time.Now().Add(time.Hour).Unix(),
	})

	_, err := v.Validate(context.Background(), token)
	if err == nil {
		t.Error("expected error for unknown kid")
	}
}
