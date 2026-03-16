package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hakken/hakken/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TripStore struct {
	pool *pgxpool.Pool
}

func NewTripStore(pool *pgxpool.Pool) *TripStore {
	return &TripStore{pool: pool}
}

func (s *TripStore) Create(ctx context.Context, t *domain.Trip) (*domain.Trip, error) {
	dataJSON, err := json.Marshal(t.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal trip data: %w", err)
	}

	result := &domain.Trip{}
	var dataRaw []byte
	err = s.pool.QueryRow(ctx,
		`INSERT INTO trips (user_id, title, start_date, end_date, home_location, timezone, cover_color, data)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, user_id, title, start_date, end_date,
		           COALESCE(home_location,''), timezone, COALESCE(cover_color,''),
		           data, created_at, updated_at`,
		t.UserID, t.Title, t.StartDate, t.EndDate,
		nullableString(t.HomeLocation), t.Timezone, nullableString(t.CoverColor),
		dataJSON,
	).Scan(
		&result.ID, &result.UserID, &result.Title, &result.StartDate, &result.EndDate,
		&result.HomeLocation, &result.Timezone, &result.CoverColor,
		&dataRaw, &result.CreatedAt, &result.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create trip: %w", err)
	}
	if err := json.Unmarshal(dataRaw, &result.Data); err != nil {
		return nil, fmt.Errorf("unmarshal trip data: %w", err)
	}
	return result, nil
}

func (s *TripStore) GetByID(ctx context.Context, id, userID uuid.UUID) (*domain.Trip, error) {
	result := &domain.Trip{}
	var dataRaw []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, title, start_date, end_date,
		        COALESCE(home_location,''), timezone, COALESCE(cover_color,''),
		        data, created_at, updated_at
		 FROM trips WHERE id = $1 AND user_id = $2`,
		id, userID,
	).Scan(
		&result.ID, &result.UserID, &result.Title, &result.StartDate, &result.EndDate,
		&result.HomeLocation, &result.Timezone, &result.CoverColor,
		&dataRaw, &result.CreatedAt, &result.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get trip: %w", err)
	}
	if err := json.Unmarshal(dataRaw, &result.Data); err != nil {
		return nil, fmt.Errorf("unmarshal trip data: %w", err)
	}
	return result, nil
}

func (s *TripStore) List(ctx context.Context, userID uuid.UUID) ([]*domain.Trip, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, title, start_date, end_date,
		        COALESCE(home_location,''), timezone, COALESCE(cover_color,''),
		        data, created_at, updated_at
		 FROM trips WHERE user_id = $1
		 ORDER BY start_date DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list trips: %w", err)
	}
	defer rows.Close()

	var trips []*domain.Trip
	for rows.Next() {
		t := &domain.Trip{}
		var dataRaw []byte
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.Title, &t.StartDate, &t.EndDate,
			&t.HomeLocation, &t.Timezone, &t.CoverColor,
			&dataRaw, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan trip: %w", err)
		}
		if err := json.Unmarshal(dataRaw, &t.Data); err != nil {
			return nil, fmt.Errorf("unmarshal trip data: %w", err)
		}
		trips = append(trips, t)
	}
	return trips, rows.Err()
}

func (s *TripStore) Update(ctx context.Context, t *domain.Trip) error {
	dataJSON, err := json.Marshal(t.Data)
	if err != nil {
		return fmt.Errorf("marshal trip data: %w", err)
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE trips SET title=$1, start_date=$2, end_date=$3, home_location=$4,
		  timezone=$5, cover_color=$6, data=$7, updated_at=NOW()
		 WHERE id=$8 AND user_id=$9`,
		t.Title, t.StartDate, t.EndDate, nullableString(t.HomeLocation),
		t.Timezone, nullableString(t.CoverColor), dataJSON, t.ID, t.UserID,
	)
	return err
}

func (s *TripStore) Delete(ctx context.Context, id, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM trips WHERE id=$1 AND user_id=$2`,
		id, userID,
	)
	return err
}

// nullableString returns nil for empty strings so PostgreSQL stores NULL.
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
