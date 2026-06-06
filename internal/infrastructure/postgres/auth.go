package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vishalss1/argus/internal/domain/auth"
)

// --- UserRepository Postgres Implementation ---

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

var _ auth.UserRepository = (*UserRepository)(nil)

func (r *UserRepository) Create(ctx context.Context, u *auth.User) error {
	query := `
		INSERT INTO users (id, email, password_hash, name, status, last_login_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(
		ctx, query,
		u.ID, u.Email, u.PasswordHash, u.Name, u.Status, u.LastLoginAt, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*auth.User, error) {
	query := `
		SELECT id, email, password_hash, name, status, last_login_at, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var u auth.User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Status, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*auth.User, error) {
	query := `
		SELECT id, email, password_hash, name, status, last_login_at, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var u auth.User
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Status, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) Update(ctx context.Context, u *auth.User) error {
	query := `
		UPDATE users
		SET email = $1, password_hash = $2, name = $3, status = $4, last_login_at = $5, updated_at = $6
		WHERE id = $7
	`
	_, err := r.db.ExecContext(
		ctx, query,
		u.Email, u.PasswordHash, u.Name, u.Status, u.LastLoginAt, u.UpdatedAt, u.ID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

func (r *UserRepository) AddWorkspaceMember(ctx context.Context, workspaceID, userID string) error {
	query := `
		INSERT INTO workspace_members (workspace_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (workspace_id, user_id) DO NOTHING
	`
	_, err := r.db.ExecContext(ctx, query, workspaceID, userID)
	if err != nil {
		return fmt.Errorf("add workspace member: %w", err)
	}
	return nil
}

func (r *UserRepository) RemoveWorkspaceMember(ctx context.Context, workspaceID, userID string) error {
	query := `
		DELETE FROM workspace_members
		WHERE workspace_id = $1 AND user_id = $2
	`
	_, err := r.db.ExecContext(ctx, query, workspaceID, userID)
	if err != nil {
		return fmt.Errorf("remove workspace member: %w", err)
	}
	return nil
}

func (r *UserRepository) ListWorkspacesForUser(ctx context.Context, userID string) ([]auth.WorkspaceInfo, error) {
	query := `
		SELECT w.id, w.name
		FROM workspaces w
		JOIN workspace_members wm ON w.id = wm.workspace_id
		WHERE wm.user_id = $1
		ORDER BY w.created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces for user: %w", err)
	}
	defer rows.Close()

	var list []auth.WorkspaceInfo
	for rows.Next() {
		var info auth.WorkspaceInfo
		if err := rows.Scan(&info.ID, &info.Name); err != nil {
			return nil, fmt.Errorf("scan workspace info: %w", err)
		}
		list = append(list, info)
	}
	return list, nil
}

func (r *UserRepository) CheckWorkspaceMembership(ctx context.Context, userID, workspaceID string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM workspace_members
			WHERE user_id = $1 AND workspace_id = $2
		)
	`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, userID, workspaceID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check workspace membership: %w", err)
	}
	return exists, nil
}

// --- RefreshTokenRepository Postgres Implementation ---

type RefreshTokenRepository struct {
	db *sql.DB
}

func NewRefreshTokenRepository(db *sql.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

var _ auth.RefreshTokenRepository = (*RefreshTokenRepository)(nil)

func (r *RefreshTokenRepository) Create(ctx context.Context, t *auth.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked, revoked_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(
		ctx, query,
		t.ID, t.UserID, t.TokenHash, t.ExpiresAt, t.Revoked, t.RevokedAt, t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) GetByHash(ctx context.Context, hash string) (*auth.RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, revoked, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var t auth.RefreshToken
	err := r.db.QueryRowContext(ctx, query, hash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.Revoked, &t.RevokedAt, &t.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get refresh token by hash: %w", err)
	}
	return &t, nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, id string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked = TRUE, revoked_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked = TRUE, revoked_at = NOW()
		WHERE user_id = $1
	`
	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("revoke all refresh tokens for user: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) DeleteExpiredOrRevoked(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM refresh_tokens
		WHERE expires_at < NOW() OR revoked = TRUE
	`
	res, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("delete expired/revoked refresh tokens: %w", err)
	}
	count, _ := res.RowsAffected()
	return count, nil
}

// --- AuditLogRepository Postgres Implementation ---

type AuditLogRepository struct {
	db *sql.DB
}

func NewAuditLogRepository(db *sql.DB) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

var _ auth.AuditLogRepository = (*AuditLogRepository)(nil)

func (r *AuditLogRepository) Create(ctx context.Context, l *auth.AuthAuditLog) error {
	query := `
		INSERT INTO auth_audit_logs (id, timestamp, user_id, event_type, ip_address, user_agent, success)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(
		ctx, query,
		l.ID, l.Timestamp, l.UserID, l.EventType, l.IPAddress, l.UserAgent, l.Success,
	)
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

func (r *AuditLogRepository) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	query := `
		DELETE FROM auth_audit_logs
		WHERE timestamp < $1
	`
	res, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete audit logs before cutoff: %w", err)
	}
	count, _ := res.RowsAffected()
	return count, nil
}
