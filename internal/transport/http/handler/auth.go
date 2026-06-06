package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/vishalss1/argus/internal/domain/auth"
	"github.com/vishalss1/argus/internal/infrastructure/redis"
)

type AuthHandler struct {
	authService *auth.Service
	redisClient *redis.Client
}

func NewAuthHandler(authService *auth.Service, redisClient *redis.Client) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		redisClient: redisClient,
	}
}

// Helper for writing generic/anti-enumeration responses
func (h *AuthHandler) writeGenericError(w http.ResponseWriter, status int, logErr error, clientMsg string) {
	writeError(w, status, clientMsg)
}

// Rate limiter helper for auth endpoints (5 requests per minute per IP)
func (h *AuthHandler) isRateLimited(r *http.Request, endpoint string) bool {
	if h.redisClient == nil {
		return false
	}
	ctx := r.Context()
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
		if idx := strings.LastIndex(ip, ":"); idx != -1 {
			ip = ip[:idx]
		}
	}
	key := fmt.Sprintf("rate_limit:auth:%s:%s", endpoint, ip)
	count, err := h.redisClient.Client().Incr(ctx, key).Result()
	if err == nil {
		if count == 1 {
			h.redisClient.Client().Expire(ctx, key, 1*time.Minute)
		}
		if count > 5 { // 5 requests per minute limit
			return true
		}
	}
	return false
}

// POST /auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 4 KB
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ip, ua := getIPAndUA(r)
	user, err := h.authService.Register(r.Context(), req.Email, req.Password, req.Name, ip, ua)
	if err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			// Generic validation error to prevent account enumeration
			h.writeGenericError(w, http.StatusBadRequest, err, "invalid email or registration credentials")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if h.isRateLimited(r, "login") {
		writeError(w, http.StatusTooManyRequests, "Too many login attempts. Try again in a minute.")
		return
	}

	// Limit request body to 4 KB
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ip, ua := getIPAndUA(r)
	user, accessToken, refreshToken, err := h.authService.Login(r.Context(), req.Email, req.Password, ip, ua)
	if err != nil {
		if errors.Is(err, auth.ErrAccountDisabled) {
			writeError(w, http.StatusForbidden, "account is disabled")
			return
		}
		// Generic invalid credentials message to prevent user enumeration
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":          user,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// POST /auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if h.isRateLimited(r, "refresh") {
		writeError(w, http.StatusTooManyRequests, "Too many refresh attempts. Try again in a minute.")
		return
	}

	// Limit request body to 4 KB
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh token is required")
		return
	}

	ip, ua := getIPAndUA(r)
	accessToken, newRefreshToken, err := h.authService.Refresh(r.Context(), req.RefreshToken, ip, ua)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
	})
}

// POST /auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 4 KB
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RefreshToken != "" {
		ip, ua := getIPAndUA(r)
		_ = h.authService.Logout(r.Context(), req.RefreshToken, ip, ua)
	}

	w.WriteHeader(http.StatusNoContent)
}


// GET /auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized: user context missing")
		return
	}

	user, workspaces, err := h.authService.GetMe(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized: user not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":       user,
		"workspaces": workspaces,
	})
}

// Helper to get client IP and User Agent
func getIPAndUA(r *http.Request) (string, string) {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
		if idx := strings.LastIndex(ip, ":"); idx != -1 {
			ip = ip[:idx]
		}
	}
	return ip, r.UserAgent()
}
