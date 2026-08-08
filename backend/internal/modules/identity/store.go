package identity

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ DB *pgxpool.Pool }

func (s Store) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	var user User
	err := s.DB.QueryRow(ctx, `
		INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)
		RETURNING id, email, password_hash, created_at, updated_at`,
		uuid.New(), email, passwordHash,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

func (s Store) GetByEmail(ctx context.Context, email string) (User, error) {
	return s.scanUser(ctx, `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email = $1`, email)
}

func (s Store) GetByID(ctx context.Context, id uuid.UUID) (User, error) {
	return s.scanUser(ctx, `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id = $1`, id)
}

func (s Store) scanUser(ctx context.Context, query string, arg any) (User, error) {
	var user User
	err := s.DB.QueryRow(ctx, query, arg).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, pgx.ErrNoRows
	}
	return user, err
}
