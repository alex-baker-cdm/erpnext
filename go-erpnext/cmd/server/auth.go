package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

type contextKey string

const userContextKey contextKey = "user"

// getUserFromContext extracts the authenticated user from the request context.
func getUserFromContext(r *http.Request) string {
	if user, ok := r.Context().Value(userContextKey).(string); ok {
		return user
	}
	return ""
}

// authMiddleware returns HTTP middleware that authenticates requests via
// Frappe session cookies or API key/secret token headers.
//
// Cookie auth: reads the "sid" cookie → queries tabSessions.
// Token auth: reads "Authorization: token api_key:api_secret" header → queries tabUser.
//
// On success the authenticated user string is stored in the request context.
// On failure a 403 with {"exc_type": "AuthenticationError"} is returned.
func authMiddleware(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := authenticateRequest(database, r)
			if err != nil || user == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{
					"exc_type": "AuthenticationError",
				})
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// authenticateRequest tries cookie auth first, then token auth.
func authenticateRequest(database *db.DB, r *http.Request) (string, error) {
	// 1. Try cookie-based auth (sid)
	if cookie, err := r.Cookie("sid"); err == nil && cookie.Value != "" {
		user, err := authenticateSession(database, cookie.Value)
		if err == nil && user != "" {
			return user, nil
		}
	}

	// 2. Try token-based auth (Authorization: token api_key:api_secret)
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "token ") {
		token := strings.TrimPrefix(authHeader, "token ")
		parts := strings.SplitN(token, ":", 2)
		if len(parts) == 2 {
			user, err := authenticateToken(database, parts[0], parts[1])
			if err == nil && user != "" {
				return user, nil
			}
		}
	}

	return "", nil
}

// authenticateSession validates a session ID against tabSessions.
func authenticateSession(database *db.DB, sid string) (string, error) {
	row, err := database.RawQueryRow(
		"SELECT user FROM `tabSessions` WHERE sid = ? AND TIMESTAMPDIFF(HOUR, lastupdate, NOW()) < 6 LIMIT 1",
		sid,
	)
	if err != nil {
		return "", err
	}
	if row == nil {
		return "", nil
	}
	if user, ok := row["user"].(string); ok {
		return user, nil
	}
	if user, ok := row["user"].([]byte); ok {
		return string(user), nil
	}
	return "", nil
}

// authenticateToken validates API key/secret against tabUser.
func authenticateToken(database *db.DB, apiKey, apiSecret string) (string, error) {
	row, err := database.RawQueryRow(
		"SELECT name FROM `tabUser` WHERE api_key = ? AND api_secret = ?",
		apiKey, apiSecret,
	)
	if err != nil {
		return "", err
	}
	if row == nil {
		return "", nil
	}
	if name, ok := row["name"].(string); ok {
		return name, nil
	}
	if name, ok := row["name"].([]byte); ok {
		return string(name), nil
	}
	return "", nil
}
