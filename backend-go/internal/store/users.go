package store

import (
	"context"
	"errors"

	"github.com/centraluniversity/researcher/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Users struct{ DB *pgxpool.Pool }

func (s Users) CreateUser(ctx context.Context, email, passwordHash string) (models.User, error) {
	var user models.User
	err := s.DB.QueryRow(ctx, `
		INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)
		RETURNING id, email, password_hash, created_at, updated_at`,
		uuid.New(), email, passwordHash,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

func (s Users) GetByEmail(ctx context.Context, email string) (models.User, error) {
	return s.scanUser(ctx, `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email = $1`, email)
}

func (s Users) GetByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	return s.scanUser(ctx, `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id = $1`, id)
}

func (s Users) scanUser(ctx context.Context, query string, arg any) (models.User, error) {
	var user models.User
	err := s.DB.QueryRow(ctx, query, arg).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, pgx.ErrNoRows
	}
	return user, err
}
