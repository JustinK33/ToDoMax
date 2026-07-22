// Package auth verifies Supabase-issued JWTs. It fetches Supabase's JWKS
// once, then provides HTTP middleware that rejects requests without a valid
// bearer token and stashes the caller's user ID in the request context for
// handlers to read via UserID.
package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey int

const userIDKey contextKey = 0

// NewKeyfunc fetches and caches Supabase's JWKS (asymmetric ES256 signing
// keys, rotated by Supabase) for verifying tokens issued by Supabase Auth.
func NewKeyfunc(ctx context.Context, supabaseURL string) (keyfunc.Keyfunc, error) {
	return keyfunc.NewDefaultCtx(ctx, []string{supabaseURL + "/auth/v1/.well-known/jwks.json"})
}

// Middleware verifies a Supabase-issued JWT from the Authorization header
// against the given keyfunc and stores the subject (user id) in context.
func Middleware(kf keyfunc.Keyfunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			tokenStr, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || tokenStr == "" {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}

			claims := jwt.MapClaims{}
			// Explicit allow-list rather than relying on keyfunc/jwt-go's
			// incidental algorithm-confusion protection, and require exp so
			// a token with no expiration isn't implicitly treated as
			// non-expiring.
			token, err := jwt.ParseWithClaims(tokenStr, claims, kf.Keyfunc,
				jwt.WithValidMethods([]string{"ES256"}),
				jwt.WithExpirationRequired(),
			)
			if err != nil || !token.Valid {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			userID, ok := claims["sub"].(string)
			if !ok || userID == "" {
				http.Error(w, "invalid token subject", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserID returns the authenticated user's id from context, set by Middleware.
func UserID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}
