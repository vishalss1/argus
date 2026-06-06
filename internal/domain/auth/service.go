package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserExists       = errors.New("user with this email already exists")
	ErrInvalidCreds     = errors.New("invalid email or password")
	ErrAccountDisabled  = errors.New("account is disabled")
	ErrInvalidToken     = errors.New("invalid or expired refresh token")
)

type Service struct {
	userRepo          UserRepository
	tokenRepo         RefreshTokenRepository
	auditRepo         AuditLogRepository
	jwtSecret         []byte
	accessExpiration  time.Duration
	refreshExpiration time.Duration
}

func NewService(
	userRepo UserRepository,
	tokenRepo RefreshTokenRepository,
	auditRepo AuditLogRepository,
	jwtSecret string,
	accessExpiration time.Duration,
	refreshExpiration time.Duration,
) *Service {
	return &Service{
		userRepo:          userRepo,
		tokenRepo:         tokenRepo,
		auditRepo:         auditRepo,
		jwtSecret:         []byte(jwtSecret),
		accessExpiration:  accessExpiration,
		refreshExpiration: refreshExpiration,
	}
}

type TokenClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func (s *Service) Register(ctx context.Context, email, password, name string, ipAddress, userAgent string) (*User, error) {
	// Clean inputs
	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)

	// Validate password policy
	if len(password) < 8 || len(password) > 128 {
		return nil, fmt.Errorf("password must be between 8 and 128 characters")
	}

	existing, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		s.logAudit(ctx, nil, "registration", ipAddress, userAgent, false)
		return nil, ErrUserExists
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	hashStr := string(hashBytes)

	now := time.Now().UTC()
	user := &User{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: &hashStr,
		Name:         name,
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logAudit(ctx, nil, "registration", ipAddress, userAgent, false)
		return nil, err
	}

	s.logAudit(ctx, &user.ID, "registration", ipAddress, userAgent, true)
	return user, nil
}

func (s *Service) Login(ctx context.Context, email, password string, ipAddress, userAgent string) (*User, string, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		s.logAudit(ctx, nil, "login_failure", ipAddress, userAgent, false)
		return nil, "", "", err
	}
	if user == nil {
		s.logAudit(ctx, nil, "login_failure", ipAddress, userAgent, false)
		return nil, "", "", ErrInvalidCreds
	}

	if user.Status == "disabled" {
		s.logAudit(ctx, &user.ID, "login_failure", ipAddress, userAgent, false)
		return nil, "", "", ErrAccountDisabled
	}

	if user.PasswordHash == nil {
		// User has no local password hash set
		s.logAudit(ctx, &user.ID, "login_failure", ipAddress, userAgent, false)
		return nil, "", "", ErrInvalidCreds
	}

	err = bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password))
	if err != nil {
		s.logAudit(ctx, &user.ID, "login_failure", ipAddress, userAgent, false)
		return nil, "", "", ErrInvalidCreds
	}

	// Password valid
	now := time.Now().UTC()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	_ = s.userRepo.Update(ctx, user)

	accessToken, refreshToken, err := s.generateTokens(ctx, user)
	if err != nil {
		s.logAudit(ctx, &user.ID, "login_failure", ipAddress, userAgent, false)
		return nil, "", "", err
	}

	s.logAudit(ctx, &user.ID, "login_success", ipAddress, userAgent, true)
	return user, accessToken, refreshToken, nil
}

func (s *Service) Refresh(ctx context.Context, rawRefreshToken string, ipAddress, userAgent string) (string, string, error) {
	tokenHash := s.hashToken(rawRefreshToken)

	tokenRecord, err := s.tokenRepo.GetByHash(ctx, tokenHash)
	if err != nil {
		s.logAudit(ctx, nil, "token_refresh", ipAddress, userAgent, false)
		return "", "", err
	}

	if tokenRecord == nil || tokenRecord.Revoked || time.Now().UTC().After(tokenRecord.ExpiresAt) {
		s.logAudit(ctx, nil, "token_refresh", ipAddress, userAgent, false)
		return "", "", ErrInvalidToken
	}

	user, err := s.userRepo.GetByID(ctx, tokenRecord.UserID)
	if err != nil {
		s.logAudit(ctx, nil, "token_refresh", ipAddress, userAgent, false)
		return "", "", err
	}
	if user == nil || user.Status == "disabled" {
		s.logAudit(ctx, nil, "token_refresh", ipAddress, userAgent, false)
		return "", "", ErrAccountDisabled
	}

	// Revoke current token immediately (Rotation)
	if err := s.tokenRepo.Revoke(ctx, tokenRecord.ID); err != nil {
		s.logAudit(ctx, &user.ID, "token_refresh", ipAddress, userAgent, false)
		return "", "", err
	}

	// Issue new tokens
	accessToken, newRefreshToken, err := s.generateTokens(ctx, user)
	if err != nil {
		s.logAudit(ctx, &user.ID, "token_refresh", ipAddress, userAgent, false)
		return "", "", err
	}

	s.logAudit(ctx, &user.ID, "token_refresh", ipAddress, userAgent, true)
	return accessToken, newRefreshToken, nil
}

func (s *Service) Logout(ctx context.Context, rawRefreshToken string, ipAddress, userAgent string) error {
	tokenHash := s.hashToken(rawRefreshToken)

	tokenRecord, err := s.tokenRepo.GetByHash(ctx, tokenHash)
	if err != nil {
		s.logAudit(ctx, nil, "logout", ipAddress, userAgent, false)
		return err
	}

	if tokenRecord != nil {
		_ = s.tokenRepo.Revoke(ctx, tokenRecord.ID)
		s.logAudit(ctx, &tokenRecord.UserID, "logout", ipAddress, userAgent, true)
	} else {
		s.logAudit(ctx, nil, "logout", ipAddress, userAgent, false)
	}

	return nil
}



func (s *Service) GetMe(ctx context.Context, userID string) (*User, []WorkspaceInfo, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		return nil, nil, fmt.Errorf("user not found")
	}

	workspaces, err := s.userRepo.ListWorkspacesForUser(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	return user, workspaces, nil
}

// CheckWorkspaceMembership validates whether a user belongs to a workspace
func (s *Service) CheckWorkspaceMembership(ctx context.Context, userID, workspaceID string) (bool, error) {
	return s.userRepo.CheckWorkspaceMembership(ctx, userID, workspaceID)
}

func (s *Service) LogWorkspaceDenial(ctx context.Context, userID string, ipAddress, userAgent string) {
	s.logAudit(ctx, &userID, "workspace_access_denial", ipAddress, userAgent, false)
}

// --- Internal Helpers ---

func (s *Service) generateTokens(ctx context.Context, u *User) (string, string, error) {
	// Fallbacks if unset
	accessExp := s.accessExpiration
	if accessExp == 0 {
		accessExp = 15 * time.Minute
	}
	refreshExp := s.refreshExpiration
	if refreshExp == 0 {
		refreshExp = 30 * 24 * time.Hour
	}

	// 1. Access Token
	accessExpiry := time.Now().Add(accessExp)
	claims := TokenClaims{
		UserID: u.ID,
		Email:  u.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   u.ID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", "", fmt.Errorf("sign access token: %w", err)
	}

	// 2. Refresh Token
	rawRefreshTokenBytes := make([]byte, 32)
	if _, err := rand.Read(rawRefreshTokenBytes); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}
	rawRefreshToken := hex.EncodeToString(rawRefreshTokenBytes)

	tokenHash := s.hashToken(rawRefreshToken)
	expiresAt := time.Now().Add(refreshExp)

	dbToken := &RefreshToken{
		ID:        uuid.New().String(),
		UserID:    u.ID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		Revoked:   false,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.tokenRepo.Create(ctx, dbToken); err != nil {
		return "", "", fmt.Errorf("persist refresh token: %w", err)
	}

	return accessToken, rawRefreshToken, nil
}

func (s *Service) hashToken(token string) string {
	hasher := sha256.New()
	hasher.Write([]byte(token))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (s *Service) logAudit(ctx context.Context, userID *string, eventType, ipAddress, userAgent string, success bool) {
	// Limit string lengths to prevent logging overflow
	if len(userAgent) > 500 {
		userAgent = userAgent[:500]
	}
	if ipAddress == "" {
		ipAddress = "unknown"
	}
	audit := &AuthAuditLog{
		ID:        uuid.New().String(),
		Timestamp: time.Now().UTC(),
		UserID:    userID,
		EventType: eventType,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   success,
	}
	_ = s.auditRepo.Create(ctx, audit)
}

// ValidateAccessToken verifies a signed JWT token and returns claims
func (s *Service) ValidateAccessToken(tokenStr string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
