package images

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/xblackbytesx/tabidachi/internal/domain"
)

// TripStorer is the subset of repository.TripStore used by AutoFetch.
type TripStorer interface {
	GetByID(ctx context.Context, id, userID uuid.UUID) (*domain.Trip, error)
	UpdateTripImage(ctx context.Context, id, userID uuid.UUID, imageURL, credit string) error
	UpdateLegImage(ctx context.Context, id, userID uuid.UUID, legIdx int, imageURL, credit string) error
}

// AutoFetch fetches cover images for a trip and all its legs in the background.
// Call as: go images.AutoFetch(tripStore, imageService, tripID, userID)
func AutoFetch(trips TripStorer, svc *Service, tripID, userID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	trip, err := trips.GetByID(ctx, tripID, userID)
	if err != nil {
		slog.Error("autofetch: get trip", "tripID", tripID, "err", err)
		return
	}

	// Fetch trip cover image
	if trip.CoverImageURL == "" {
		imageURL, credit, err := svc.FetchAndStore(ctx, trip.Title)
		if err != nil {
			slog.Warn("autofetch: trip cover", "tripID", tripID, "err", err)
		} else {
			if err := trips.UpdateTripImage(ctx, tripID, userID, imageURL, credit); err != nil {
				slog.Error("autofetch: update trip image", "tripID", tripID, "err", err)
			} else {
				slog.Info("autofetch: trip cover fetched", "tripID", tripID, "url", imageURL)
			}
		}
	}

	// Fetch each leg cover image
	for i, leg := range trip.Data.Legs {
		if leg.CoverImageURL != "" {
			continue
		}
		query := leg.Destination
		if leg.Region != "" {
			query = leg.Destination + " " + leg.Region
		}
		imageURL, credit, err := svc.FetchAndStore(ctx, query)
		if err != nil {
			slog.Warn("autofetch: leg cover", "tripID", tripID, "leg", i, "err", err)
			continue
		}
		if err := trips.UpdateLegImage(ctx, tripID, userID, i, imageURL, credit); err != nil {
			slog.Error("autofetch: update leg image", "tripID", tripID, "leg", i, "err", err)
		} else {
			slog.Info("autofetch: leg cover fetched", "tripID", tripID, "leg", i, "url", imageURL)
		}
	}
}
