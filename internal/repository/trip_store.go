package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/xblackbytesx/tabidachi/internal/domain"
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
		           data, created_at, updated_at,
		           COALESCE(cover_image_url,''), COALESCE(cover_image_credit,'')`,
		t.UserID, t.Title, t.StartDate, t.EndDate,
		nullableString(t.HomeLocation), t.Timezone, nullableString(t.CoverColor),
		dataJSON,
	).Scan(
		&result.ID, &result.UserID, &result.Title, &result.StartDate, &result.EndDate,
		&result.HomeLocation, &result.Timezone, &result.CoverColor,
		&dataRaw, &result.CreatedAt, &result.UpdatedAt,
		&result.CoverImageURL, &result.CoverImageCredit,
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
		        data, created_at, updated_at,
		        COALESCE(cover_image_url,''), COALESCE(cover_image_credit,'')
		 FROM trips WHERE id = $1 AND user_id = $2`,
		id, userID,
	).Scan(
		&result.ID, &result.UserID, &result.Title, &result.StartDate, &result.EndDate,
		&result.HomeLocation, &result.Timezone, &result.CoverColor,
		&dataRaw, &result.CreatedAt, &result.UpdatedAt,
		&result.CoverImageURL, &result.CoverImageCredit,
	)
	if err != nil {
		return nil, fmt.Errorf("get trip: %w", err)
	}
	if err := json.Unmarshal(dataRaw, &result.Data); err != nil {
		return nil, fmt.Errorf("unmarshal trip data: %w", err)
	}
	return result, nil
}

// GetByIDAnon fetches a trip by ID without a user ownership check.
// Only call this after a share token has been validated.
func (s *TripStore) GetByIDAnon(ctx context.Context, id uuid.UUID) (*domain.Trip, error) {
	result := &domain.Trip{}
	var dataRaw []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, title, start_date, end_date,
		        COALESCE(home_location,''), timezone, COALESCE(cover_color,''),
		        data, created_at, updated_at,
		        COALESCE(cover_image_url,''), COALESCE(cover_image_credit,'')
		 FROM trips WHERE id = $1`,
		id,
	).Scan(
		&result.ID, &result.UserID, &result.Title, &result.StartDate, &result.EndDate,
		&result.HomeLocation, &result.Timezone, &result.CoverColor,
		&dataRaw, &result.CreatedAt, &result.UpdatedAt,
		&result.CoverImageURL, &result.CoverImageCredit,
	)
	if err != nil {
		return nil, fmt.Errorf("get trip anon: %w", err)
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
		        data, created_at, updated_at,
		        COALESCE(cover_image_url,''), COALESCE(cover_image_credit,'')
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
			&t.CoverImageURL, &t.CoverImageCredit,
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

// UpdateTripImage sets the cover_image_url and cover_image_credit for a trip.
func (s *TripStore) UpdateTripImage(ctx context.Context, id, userID uuid.UUID, imageURL, credit string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE trips SET cover_image_url=$1, cover_image_credit=$2, updated_at=NOW()
		 WHERE id=$3 AND user_id=$4`,
		nullableString(imageURL), nullableString(credit), id, userID,
	)
	return err
}

// UpdateLegImage sets a leg's cover image URL and credit in the trip's JSONB data.
func (s *TripStore) UpdateLegImage(ctx context.Context, id, userID uuid.UUID, legIdx int, imageURL, credit string) error {
	trip, err := s.GetByID(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("update leg image: %w", err)
	}
	if legIdx < 0 || legIdx >= len(trip.Data.Legs) {
		return fmt.Errorf("update leg image: leg index %d out of range", legIdx)
	}
	trip.Data.Legs[legIdx].CoverImageURL = imageURL
	trip.Data.Legs[legIdx].CoverImageCredit = credit
	return s.Update(ctx, trip)
}

// UpdateEventImage updates the image fields on a specific event in the JSONB data.
func (s *TripStore) UpdateEventImage(ctx context.Context, id, userID uuid.UUID, legIdx, dayIdx, eventIdx int, imageURL, thumbURL, credit string) error {
	trip, err := s.GetByID(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("update event image: %w", err)
	}
	if legIdx < 0 || legIdx >= len(trip.Data.Legs) {
		return fmt.Errorf("update event image: leg index %d out of range", legIdx)
	}
	if dayIdx < 0 || dayIdx >= len(trip.Data.Legs[legIdx].Days) {
		return fmt.Errorf("update event image: day index %d out of range", dayIdx)
	}
	if eventIdx < 0 || eventIdx >= len(trip.Data.Legs[legIdx].Days[dayIdx].Events) {
		return fmt.Errorf("update event image: event index %d out of range", eventIdx)
	}
	trip.Data.Legs[legIdx].Days[dayIdx].Events[eventIdx].ImageURL = imageURL
	trip.Data.Legs[legIdx].Days[dayIdx].Events[eventIdx].ImageThumbURL = thumbURL
	trip.Data.Legs[legIdx].Days[dayIdx].Events[eventIdx].ImageCredit = credit
	return s.Update(ctx, trip)
}

// nullableString returns nil for empty strings so PostgreSQL stores NULL.
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
