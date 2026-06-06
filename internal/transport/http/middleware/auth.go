package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/vishalss1/argus/internal/domain/auth"
)

// Helper to write JSON error responses
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// Auth middleware validates the JWT access token and injects the user context
func Auth(authService *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenStr string
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
					tokenStr = parts[1]
				}
			}

			// Fallback to query parameter (e.g. for WebSocket connections)
			if tokenStr == "" {
				tokenStr = r.URL.Query().Get("token")
			}

			if tokenStr == "" {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: authorization token missing")
				return
			}

			claims, err := authService.ValidateAccessToken(tokenStr)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: invalid or expired access token")
				return
			}

			// Validate status by fetching the user profile
			user, _, err := authService.GetMe(r.Context(), claims.UserID)
			if err != nil || user == nil || user.Status != "active" {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: user account is inactive or not found")
				return
			}

			// Inject user info into context
			ctx := auth.WithUserID(r.Context(), claims.UserID)
			ctx = context.WithValue(ctx, "email", claims.Email) // secondary lookup value if needed
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WorkspaceAuth middleware validates X-Workspace-ID header and memberships
func WorkspaceAuth(authService *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := auth.GetUserID(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: user context missing")
				return
			}

			workspaceID := r.Header.Get("X-Workspace-ID")
			if workspaceID == "" {
				workspaceID = r.URL.Query().Get("workspace_id")
			}

			if workspaceID == "" {
				writeJSONError(w, http.StatusBadRequest, "workspace ID (X-Workspace-ID header or workspace_id query parameter) is required")
				return
			}

			isMember, err := authService.CheckWorkspaceMembership(r.Context(), userID, workspaceID)
			if err != nil || !isMember {
				// Get client IP and User Agent for audit logging
				ip := r.Header.Get("X-Forwarded-For")
				if ip == "" {
					ip = r.RemoteAddr
					if idx := strings.LastIndex(ip, ":"); idx != -1 {
						ip = ip[:idx]
					}
				}
				ua := r.UserAgent()
				authService.LogWorkspaceDenial(r.Context(), userID, ip, ua)

				writeJSONError(w, http.StatusForbidden, "forbidden: not a member of this workspace")
				return
			}

			// Inject workspace_id into context
			ctx := auth.WithWorkspaceID(r.Context(), workspaceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
