package repository

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/xblackbytesx/tabidachi/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ShareStore struct {
	pool *pgxpool.Pool
}

func NewShareStore(pool *pgxpool.Pool) *ShareStore {
	return &ShareStore{pool: pool}
}

// Generate creates a new share token for a trip and returns the raw token (only returned once).
func (s *ShareStore) Generate(ctx context.Context, tripID, userID uuid.UUID, name string) (rawToken string, share *domain.TripShare, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate share token bytes: %w", err)
	}
	rawToken = "tbd_share_" + base64.RawURLEncoding.EncodeToString(raw)
	hash := hashToken(rawToken)

	share = &domain.TripShare{}
	err = s.pool.QueryRow(ctx,
		`INSERT INTO trip_shares (trip_id, user_id, name, token_hash)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, trip_id, user_id, name, token_hash, expires_at, created_at, last_used_at`,
		tripID, userID, name, hash,
	).Scan(
		&share.ID, &share.TripID, &share.UserID, &share.Name, &share.TokenHash,
		&share.ExpiresAt, &share.CreatedAt, &share.LastUsedAt,
	)
	if err != nil {
		return "", nil, fmt.Errorf("insert trip share: %w", err)
	}
	return rawToken, share, nil
}

// GetByRawToken looks up a share by raw token string, validating it belongs to a specific trip.
// Returns an error if the token is not found or has expired.
func (s *ShareStore) GetByRawToken(ctx context.Context, rawToken string) (*domain.TripShare, error) {
	hash := hashToken(rawToken)
	share := &domain.TripShare{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, trip_id, user_id, name, token_hash, expires_at, created_at, last_used_at
		 FROM trip_shares WHERE token_hash = $1`,
		hash,
	).Scan(
		&share.ID, &share.TripID, &share.UserID, &share.Name, &share.TokenHash,
		&share.ExpiresAt, &share.CreatedAt, &share.LastUsedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get trip share: %w", err)
	}
	if share.IsExpired() {
		return nil, fmt.Errorf("share token expired")
	}
	return share, nil
}

// ListByTrip returns all share links for a trip, enforcing user ownership.
func (s *ShareStore) ListByTrip(ctx context.Context, tripID, userID uuid.UUID) ([]*domain.TripShare, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, trip_id, user_id, name, token_hash, expires_at, created_at, last_used_at
		 FROM trip_shares WHERE trip_id = $1 AND user_id = $2 ORDER BY created_at DESC`,
		tripID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list trip shares: %w", err)
	}
	defer rows.Close()

	var shares []*domain.TripShare
	for rows.Next() {
		sh := &domain.TripShare{}
		if err := rows.Scan(
			&sh.ID, &sh.TripID, &sh.UserID, &sh.Name, &sh.TokenHash,
			&sh.ExpiresAt, &sh.CreatedAt, &sh.LastUsedAt,
		); err != nil {
			return nil, fmt.Errorf("scan trip share: %w", err)
		}
		shares = append(shares, sh)
	}
	return shares, rows.Err()
}

// Delete removes a share link by ID, enforcing both trip and user ownership.
func (s *ShareStore) Delete(ctx context.Context, shareID, tripID, userID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM trip_shares WHERE id = $1 AND trip_id = $2 AND user_id = $3`,
		shareID, tripID, userID,
	)
	if err != nil {
		return fmt.Errorf("delete trip share: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("share not found")
	}
	return nil
}

// UpdateLastUsed records the current time as last_used_at.
func (s *ShareStore) UpdateLastUsed(ctx context.Context, shareID uuid.UUID) {
	if _, err := s.pool.Exec(ctx,
		`UPDATE trip_shares SET last_used_at = now() WHERE id = $1`,
		shareID,
	); err != nil {
		slog.Warn("update share last_used_at", "err", err)
	}
}
