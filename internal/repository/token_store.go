package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
	"github.com/hakken/hakken/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TokenStore struct {
	pool *pgxpool.Pool
}

func NewTokenStore(pool *pgxpool.Pool) *TokenStore {
	return &TokenStore{pool: pool}
}

// Generate creates a new token, persists its hash, and returns the raw token string.
// The raw token is only returned once and never stored.
func (s *TokenStore) Generate(ctx context.Context, userID uuid.UUID, name string) (rawToken string, tok *domain.APIToken, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate token bytes: %w", err)
	}
	rawToken = "hkn_" + base64.RawURLEncoding.EncodeToString(raw)
	hash := hashToken(rawToken)

	tok = &domain.APIToken{}
	err = s.pool.QueryRow(ctx,
		`INSERT INTO api_tokens (user_id, name, token_hash)
		 VALUES ($1, $2, $3)
		 RETURNING id, user_id, name, token_hash, created_at, last_used_at`,
		userID, name, hash,
	).Scan(&tok.ID, &tok.UserID, &tok.Name, &tok.TokenHash, &tok.CreatedAt, &tok.LastUsedAt)
	if err != nil {
		return "", nil, fmt.Errorf("insert api token: %w", err)
	}
	return rawToken, tok, nil
}

// List returns all tokens for a user (without the hash).
func (s *TokenStore) List(ctx context.Context, userID uuid.UUID) ([]*domain.APIToken, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, token_hash, created_at, last_used_at
		 FROM api_tokens WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*domain.APIToken
	for rows.Next() {
		t := &domain.APIToken{}
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.TokenHash, &t.CreatedAt, &t.LastUsedAt); err != nil {
			return nil, fmt.Errorf("scan api token: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// GetByRawToken looks up a token record by the raw token string.
func (s *TokenStore) GetByRawToken(ctx context.Context, rawToken string) (*domain.APIToken, error) {
	hash := hashToken(rawToken)
	t := &domain.APIToken{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, name, token_hash, created_at, last_used_at
		 FROM api_tokens WHERE token_hash = $1`,
		hash,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.TokenHash, &t.CreatedAt, &t.LastUsedAt)
	if err != nil {
		return nil, fmt.Errorf("get api token: %w", err)
	}
	return t, nil
}

// UpdateLastUsed records the current time as last_used_at for the given token.
func (s *TokenStore) UpdateLastUsed(ctx context.Context, tokenID uuid.UUID) {
	_, _ = s.pool.Exec(ctx,
		`UPDATE api_tokens SET last_used_at = now() WHERE id = $1`,
		tokenID,
	)
}

// Delete removes a token by ID, enforcing user ownership.
func (s *TokenStore) Delete(ctx context.Context, tokenID, userID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM api_tokens WHERE id = $1 AND user_id = $2`,
		tokenID, userID,
	)
	if err != nil {
		return fmt.Errorf("delete api token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("token not found")
	}
	return nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
