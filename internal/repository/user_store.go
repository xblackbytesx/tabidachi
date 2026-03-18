package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/xblackbytesx/tabidachi/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserStore struct {
	pool *pgxpool.Pool
}

func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool: pool}
}

func (s *UserStore) Create(ctx context.Context, email, displayName, passwordHash string) (*domain.User, error) {
	u := &domain.User{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, display_name, password_hash)
		 VALUES ($1, $2, $3)
		 RETURNING id, email, display_name, password_hash, date_format, created_at`,
		email, displayName, passwordHash,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.DateFormat, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (s *UserStore) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	u := &domain.User{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, display_name, password_hash, date_format, created_at FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.DateFormat, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

func (s *UserStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u := &domain.User{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, display_name, password_hash, date_format, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.DateFormat, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// Delete permanently removes a user and all associated data (trips, tokens cascade via FK).
func (s *UserStore) Delete(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func (s *UserStore) UpdateDateFormat(ctx context.Context, userID uuid.UUID, pref string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET date_format = $1 WHERE id = $2`,
		pref, userID,
	)
	if err != nil {
		return fmt.Errorf("update date format: %w", err)
	}
	return nil
}
