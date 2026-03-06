package auth

import (
	"context"
	"net/http"
	"strings"
)

// contextKey is an unexported type for context keys in this package.
// Using a custom type prevents collisions with keys from other packages.
type contextKey string

const claimsKey contextKey = "claims"

// Middleware wraps an HTTP handler and requires a valid JWT.
// It accepts the token from the Authorization header ("Bearer <token>")
// or from a "token" query parameter (used by WebSocket connections,
// since browsers can't set custom headers on WS upgrades).
func Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := tokenFromRequest(r)
		if token == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		claims, err := Decode(token)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// Store the claims in the request context so handlers can access them
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next(w, r.WithContext(ctx))
	}
}

// ClaimsFromContext retrieves the JWT claims stored by Middleware.
// Call this in any protected handler to get the current user's ID and username.
func ClaimsFromContext(ctx context.Context) *Claims {
	claims, _ := ctx.Value(claimsKey).(*Claims)
	return claims
}

func tokenFromRequest(r *http.Request) string {
	// Check Authorization header first
	if header := r.Header.Get("Authorization"); header != "" {
		if after, ok := strings.CutPrefix(header, "Bearer "); ok {
			return after
		}
	}
	// Fall back to query param (for WebSocket connections)
	return r.URL.Query().Get("token")
}
