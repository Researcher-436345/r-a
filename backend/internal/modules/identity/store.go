package identity

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ DB *pgxpool.Pool }

// SessionStore abstracts identity persistence so handlers can be tested
// against an in-memory fake. *Store (with a real pool) implements it.
type SessionStore interface {
	CreateUser(ctx context.Context, email, passwordHash string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByID(ctx context.Context, id uuid.UUID) (User, error)
	MarkVerified(ctx context.Context, userID uuid.UUID) error
	UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error
	CreateSession(ctx context.Context, userID uuid.UUID, tokenHash, userAgent, ip string, expiresAt time.Time) (Session, error)
	GetSessionByToken(ctx context.Context, tokenHash string) (Session, error)
	RotateSession(ctx context.Context, old Session, tokenHash string, expiresAt time.Time) (Session, error)
	RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) (bool, error)
	RevokeAllSessions(ctx context.Context, userID, exceptID uuid.UUID) error
	ListActiveSessions(ctx context.Context, userID uuid.UUID) ([]Session, error)
	CreateAuthToken(ctx context.Context, userID uuid.UUID, typ, tokenHash string, ttl time.Duration) error
	ConsumeAuthToken(ctx context.Context, typ, tokenHash string) (uuid.UUID, error)
}

const userCols = `id, email, password_hash, email_verified_at, created_at, updated_at`

func (s Store) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	var user User
	err := s.DB.QueryRow(ctx, `
		INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)
		RETURNING `+userCols,
		uuid.New(), email, passwordHash,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.EmailVerifiedAt, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

func (s Store) GetByEmail(ctx context.Context, email string) (User, error) {
	return s.scanUser(ctx, `SELECT `+userCols+` FROM users WHERE email = $1`, email)
}

func (s Store) GetByID(ctx context.Context, id uuid.UUID) (User, error) {
	return s.scanUser(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id)
}

func (s Store) scanUser(ctx context.Context, query string, arg any) (User, error) {
	var user User
	err := s.DB.QueryRow(ctx, query, arg).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.EmailVerifiedAt, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, pgx.ErrNoRows
	}
	return user, err
}

func (s Store) MarkVerified(ctx context.Context, userID uuid.UUID) error {
	_, err := s.DB.Exec(ctx, `UPDATE users SET email_verified_at = now(), updated_at = now() WHERE id = $1`, userID)
	return err
}

func (s Store) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	_, err := s.DB.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, userID, passwordHash)
	return err
}

// --- auth_sessions (refresh) ---

func (s Store) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash, userAgent, ip string, expiresAt time.Time) (Session, error) {
	var sess Session
	err := s.DB.QueryRow(ctx, `
		INSERT INTO auth_sessions (user_id, token_hash, user_agent, ip, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, user_agent, ip, created_at, last_used_at, expires_at, revoked_at, replaced_by`,
		userID, tokenHash, userAgent, ip, expiresAt,
	).Scan(&sess.ID, &sess.UserID, &sess.UserAgent, &sess.IP, &sess.CreatedAt, &sess.LastUsedAt, &sess.ExpiresAt, &sess.RevokedAt, &sess.ReplacedBy)
	return sess, err
}

// GetSessionByToken returns the session for the given token hash, if it exists.
func (s Store) GetSessionByToken(ctx context.Context, tokenHash string) (Session, error) {
	var sess Session
	err := s.DB.QueryRow(ctx, `
		SELECT id, user_id, user_agent, ip, created_at, last_used_at, expires_at, revoked_at, replaced_by
		FROM auth_sessions WHERE token_hash = $1`, tokenHash,
	).Scan(&sess.ID, &sess.UserID, &sess.UserAgent, &sess.IP, &sess.CreatedAt, &sess.LastUsedAt, &sess.ExpiresAt, &sess.RevokedAt, &sess.ReplacedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, pgx.ErrNoRows
	}
	return sess, err
}

// RotateSession revokes the old session and creates the replacement atomically.
func (s Store) RotateSession(ctx context.Context, old Session, tokenHash string, expiresAt time.Time) (Session, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)

	var sess Session
	err = tx.QueryRow(ctx, `
		INSERT INTO auth_sessions (user_id, token_hash, user_agent, ip, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, user_agent, ip, created_at, last_used_at, expires_at, revoked_at, replaced_by`,
		old.UserID, tokenHash, old.UserAgent, old.IP, expiresAt,
	).Scan(&sess.ID, &sess.UserID, &sess.UserAgent, &sess.IP, &sess.CreatedAt, &sess.LastUsedAt, &sess.ExpiresAt, &sess.RevokedAt, &sess.ReplacedBy)
	if err != nil {
		return Session{}, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE auth_sessions SET revoked_at = now(), replaced_by = $2 WHERE id = $1`,
		old.ID, sess.ID)
	if err != nil {
		return Session{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	return sess, nil
}

// RevokeSession revokes one active session owned by userID; reports whether
// a row matched (false → unknown, foreign or already revoked).
func (s Store) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) (bool, error) {
	tag, err := s.DB.Exec(ctx,
		`UPDATE auth_sessions SET revoked_at = now() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`,
		sessionID, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// RevokeAllSessions revokes every active session of the user. exceptID may be
// uuid.Nil to revoke absolutely everything (e.g. after a password reset).
func (s Store) RevokeAllSessions(ctx context.Context, userID, exceptID uuid.UUID) error {
	_, err := s.DB.Exec(ctx, `
		UPDATE auth_sessions SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL AND ($2::uuid IS NULL OR id <> $2)`,
		userID, nullableUUID(exceptID))
	return err
}

func (s Store) ListActiveSessions(ctx context.Context, userID uuid.UUID) ([]Session, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, user_id, user_agent, ip, created_at, last_used_at, expires_at, revoked_at, replaced_by
		FROM auth_sessions
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()
		ORDER BY last_used_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.UserID, &sess.UserAgent, &sess.IP, &sess.CreatedAt, &sess.LastUsedAt, &sess.ExpiresAt, &sess.RevokedAt, &sess.ReplacedBy); err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// --- auth_tokens (one-time: verify_email / reset_password) ---

func (s Store) CreateAuthToken(ctx context.Context, userID uuid.UUID, typ, tokenHash string, ttl time.Duration) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO auth_tokens (user_id, type, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`, userID, typ, tokenHash, time.Now().Add(ttl))
	return err
}

// ConsumeAuthToken atomically marks a one-time token used and returns its owner.
// Returns pgx.ErrNoRows when the token is unknown, already used or expired.
func (s Store) ConsumeAuthToken(ctx context.Context, typ, tokenHash string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := s.DB.QueryRow(ctx, `
		UPDATE auth_tokens SET used_at = now()
		WHERE token_hash = $1 AND type = $2 AND used_at IS NULL AND expires_at > now()
		RETURNING user_id`, tokenHash, typ).Scan(&userID)
	return userID, err
}

func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
