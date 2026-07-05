package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

const testKID = "test-kid"

// newTestKeyfunc generates an ES256 key pair, wraps its public key in a JWK
// Set (as Supabase's JWKS endpoint would serve), and returns both the
// keyfunc.Keyfunc used for verification and the private key used to sign
// test tokens - all offline, no network calls.
func newTestKeyfunc(t *testing.T) (keyfunc.Keyfunc, *ecdsa.PrivateKey) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	jwk, err := jwkset.NewJWKFromKey(priv.Public(), jwkset.JWKOptions{
		Metadata: jwkset.JWKMetadataOptions{KID: testKID, ALG: jwkset.AlgES256},
	})
	if err != nil {
		t.Fatalf("failed to build jwk: %v", err)
	}

	raw, err := json.Marshal(jwkset.JWKSMarshal{Keys: []jwkset.JWKMarshal{jwk.Marshal()}})
	if err != nil {
		t.Fatalf("failed to marshal jwks: %v", err)
	}

	kf, err := keyfunc.NewJWKSetJSON(raw)
	if err != nil {
		t.Fatalf("failed to build keyfunc: %v", err)
	}

	return kf, priv
}

func signToken(t *testing.T, priv *ecdsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = testKID
	s, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func newProtectedHandler(kf keyfunc.Keyfunc) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserID(r.Context())
		if !ok {
			http.Error(w, "no user id in context", http.StatusInternalServerError)
			return
		}
		w.Write([]byte(userID))
	})
	return Middleware(kf)(inner)
}

func TestMiddlewareValidToken(t *testing.T) {
	kf, priv := newTestKeyfunc(t)
	tok := signToken(t, priv, jwt.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	newProtectedHandler(kf).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "user-123" {
		t.Fatalf("expected user-123, got %q", rec.Body.String())
	}
}

func TestMiddlewareMissingHeader(t *testing.T) {
	kf, _ := newTestKeyfunc(t)
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()

	newProtectedHandler(kf).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMiddlewareBadSignature(t *testing.T) {
	kf, _ := newTestKeyfunc(t)
	_, otherKey := newTestKeyfunc(t) // different key than the one in kf's set

	tok := signToken(t, otherKey, jwt.MapClaims{"sub": "user-123"})

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	newProtectedHandler(kf).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMiddlewareExpiredToken(t *testing.T) {
	kf, priv := newTestKeyfunc(t)
	tok := signToken(t, priv, jwt.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	newProtectedHandler(kf).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
