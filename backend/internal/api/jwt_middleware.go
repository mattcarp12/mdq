package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// Define a custom type for our context key to prevent collisions
type contextKey string

const UserIDKey contextKey = "user_id"

// AuthMiddleware wraps an HTTP handler and enforces JWT authentication.
func (s *Server) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			slog.Error("Missing or invalid Authorization header")
			http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Parse and validate the token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			// Validate the signing algorithm
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, http.ErrAbortHandler
			}
			return s.jwtSecret, nil
		})

		if err != nil || !token.Valid {
			slog.Error("Invalid token", "error", err.Error())
			http.Error(w, `{"error": "invalid token"}`, http.StatusUnauthorized)
			return
		}

		// Extract claims (the payload payload)
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			slog.Error("Invalid token claims")
			http.Error(w, `{"error": "invalid claims"}`, http.StatusUnauthorized)
			return
		}

		// Push the user_id into the request context
		userID := claims["sub"].(string)
		ctx := context.WithValue(r.Context(), UserIDKey, userID)

		// Call the next handler with the new context
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
