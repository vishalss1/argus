package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/vishalss1/argus/core/internal/domain/auth"
	"github.com/vishalss1/argus/core/internal/domain/device"
	"github.com/vishalss1/argus/shared/common"
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
			ctx = common.WithWorkspaceID(ctx, workspaceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// DeviceAuth middleware validates X-Device-API-Key header and injects device context
func DeviceAuth(deviceRepo device.Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mtlsDeviceID, ok := r.Context().Value("mtls_device_id").(string)
			if !ok || mtlsDeviceID == "" {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: mTLS authentication required")
				return
			}

			apiKey := r.Header.Get("X-Device-API-Key")
			if apiKey == "" {
				apiKey = r.URL.Query().Get("api_key")
			}

			if apiKey == "" {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: device API key missing")
				return
			}

			if len(apiKey) < 8 {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: invalid device API key format")
				return
			}

			prefix := apiKey[:8]
			dev, err := deviceRepo.GetByAPIKeyPrefix(r.Context(), prefix)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: invalid device API key")
				return
			}

			hash := sha256.Sum256([]byte(apiKey))
			if subtle.ConstantTimeCompare(hash[:], dev.APIKeyHash) != 1 {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: invalid device API key")
				return
			}

			if dev.ID != mtlsDeviceID {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: client certificate device ID does not match API key device ID")
				return
			}

			deviceID := chi.URLParam(r, "deviceID")
			if deviceID != "" && deviceID != dev.ID {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: device ID mismatch")
				return
			}

			ctx := r.Context()
			if dev.WorkspaceID != nil {
				ctx = auth.WithWorkspaceID(ctx, *dev.WorkspaceID)
				ctx = common.WithWorkspaceID(ctx, *dev.WorkspaceID)
			}
			ctx = context.WithValue(ctx, "device_id", dev.ID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

