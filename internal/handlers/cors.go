package handlers

import "net/http"

// CORS wraps a handler and adds cross-origin headers.
// The allowed origin is configurable so you can lock it down per environment.
func CORS(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		// Required for browsers to send/receive cookies cross-origin
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Preflight request — browsers send OPTIONS first to check permissions
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
