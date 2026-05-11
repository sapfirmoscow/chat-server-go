package main

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const userIDKey contextKey = "userID"

func AuthMiddleware(jwtManager *JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				writeError(w, http.StatusUnauthorized, "invalid authorization header format")
				return
			}

			tokenStr := parts[1]

			userID, err := jwtManager.Verify(tokenStr)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
