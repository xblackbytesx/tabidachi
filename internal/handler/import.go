package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/xblackbytesx/tabidachi/internal/domain"
	"github.com/xblackbytesx/tabidachi/internal/images"
	"github.com/xblackbytesx/tabidachi/internal/repository"
	"github.com/xblackbytesx/tabidachi/web/templates/pages"
	"github.com/labstack/echo/v4"
)

type ImportHandler struct {
	trips    *repository.TripStore
	imageSvc *images.Service
}

func NewImportHandler(trips *repository.TripStore, imageSvc *images.Service) *ImportHandler {
	return &ImportHandler{trips: trips, imageSvc: imageSvc}
}

func (h *ImportHandler) Get(c echo.Context) error {
	return render(c, http.StatusOK, pages.TripImport(csrfToken(c), ""))
}

func (h *ImportHandler) Post(c echo.Context) error {
	uid, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}

	raw := c.FormValue("json_data")
	if raw == "" {
		return render(c, http.StatusOK, pages.TripImport(csrfToken(c), "JSON data is required."))
	}

	if len(raw) > 2*1024*1024 { // 2 MB max
		return render(c, http.StatusOK, pages.TripImport(csrfToken(c), "JSON data is too large (max 2 MB)."))
	}

	var data domain.TripData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return render(c, http.StatusOK, pages.TripImport(csrfToken(c), "Invalid JSON: "+err.Error()))
	}

	if err := validateTripData(&data); err != nil {
		return render(c, http.StatusOK, pages.TripImport(csrfToken(c), err.Error()))
	}

	if data.Title == "" {
		return render(c, http.StatusOK, pages.TripImport(csrfToken(c), "Trip title is required in JSON."))
	}
	if data.StartDate == "" || data.EndDate == "" {
		return render(c, http.StatusOK, pages.TripImport(csrfToken(c), "startDate and endDate are required in JSON."))
	}

	startDate, err := time.Parse("2006-01-02", data.StartDate)
	if err != nil {
		return render(c, http.StatusOK, pages.TripImport(csrfToken(c), "Invalid startDate format. Use YYYY-MM-DD."))
	}
	endDate, err := time.Parse("2006-01-02", data.EndDate)
	if err != nil {
		return render(c, http.StatusOK, pages.TripImport(csrfToken(c), "Invalid endDate format. Use YYYY-MM-DD."))
	}

	if data.SchemaVersion == "" {
		data.SchemaVersion = "1.1"
	}
	if data.Timezone == "" {
		data.Timezone = "UTC"
	}

	trip := &domain.Trip{
		UserID:       uid,
		Title:        data.Title,
		StartDate:    startDate,
		EndDate:      endDate,
		HomeLocation: data.HomeLocation,
		Timezone:     data.Timezone,
		Data:         data,
	}

	created, err := h.trips.Create(c.Request().Context(), trip)
	if err != nil {
		slog.Error("import: create trip", "err", err)
		return render(c, http.StatusOK, pages.TripImport(csrfToken(c), "Failed to save trip. Please try again."))
	}

	go images.AutoFetch(h.trips, h.imageSvc, created.ID, created.UserID)

	if isHTMX(c) {
		c.Response().Header().Set("HX-Redirect", "/trips/"+created.ID.String())
		return c.NoContent(http.StatusNoContent)
	}
	return redirect(c, "/trips/"+created.ID.String())
}

// validateTripData enforces structural limits on imported trip data to prevent
// abuse or accidental resource exhaustion.
func validateTripData(data *domain.TripData) error {
	const (
		maxLegs        = 50
		maxDaysPerLeg  = 60
		maxEventsPerDay = 50
		maxStringLen   = 1000
	)

	if len(data.Title) > maxStringLen {
		return fmt.Errorf("Title is too long (max %d characters).", maxStringLen)
	}
	if len(data.HomeLocation) > maxStringLen {
		return fmt.Errorf("Home location is too long (max %d characters).", maxStringLen)
	}
	if len(data.Legs) > maxLegs {
		return fmt.Errorf("Too many legs (max %d).", maxLegs)
	}

	validDayTypes := map[string]bool{
		"normal": true, "arrival": true, "departure": true,
		"travel": true, "rest": true, "flexible": true, "": true,
	}
	validEventTypes := map[string]bool{
		"activity": true, "transit": true, "accommodation": true,
	}
	validStatuses := map[string]bool{
		"confirmed": true, "tentative": true, "cancelled": true, "": true,
	}

	for i, leg := range data.Legs {
		if len(leg.Destination) > maxStringLen {
			return fmt.Errorf("Leg %d destination is too long.", i+1)
		}
		if len(leg.Notes) > 5000 {
			return fmt.Errorf("Leg %d notes are too long (max 5000 characters).", i+1)
		}
		if leg.StartDate != "" {
			if _, err := time.Parse("2006-01-02", leg.StartDate); err != nil {
				return fmt.Errorf("Leg %d has invalid startDate format (use YYYY-MM-DD).", i+1)
			}
		}
		if leg.EndDate != "" {
			if _, err := time.Parse("2006-01-02", leg.EndDate); err != nil {
				return fmt.Errorf("Leg %d has invalid endDate format (use YYYY-MM-DD).", i+1)
			}
		}
		if len(leg.Days) > maxDaysPerLeg {
			return fmt.Errorf("Leg %d has too many days (max %d).", i+1, maxDaysPerLeg)
		}
		for j, day := range leg.Days {
			if !validDayTypes[day.Type] {
				return fmt.Errorf("Leg %d, day %d has invalid type %q.", i+1, j+1, day.Type)
			}
			if day.Date != "" {
				if _, err := time.Parse("2006-01-02", day.Date); err != nil {
					return fmt.Errorf("Leg %d, day %d has invalid date format (use YYYY-MM-DD).", i+1, j+1)
				}
			}
			if len(day.Notes) > 5000 {
				return fmt.Errorf("Leg %d, day %d notes are too long (max 5000 characters).", i+1, j+1)
			}
			if len(day.Events) > maxEventsPerDay {
				return fmt.Errorf("Leg %d, day %d has too many events (max %d).", i+1, j+1, maxEventsPerDay)
			}
			for k, event := range day.Events {
				if !validEventTypes[event.Type] {
					return fmt.Errorf("Leg %d, day %d, event %d has invalid type %q.", i+1, j+1, k+1, event.Type)
				}
				if len(event.Title) > maxStringLen {
					return fmt.Errorf("Leg %d, day %d, event %d title is too long.", i+1, j+1, k+1)
				}
				if len(event.Notes) > 5000 {
					return fmt.Errorf("Leg %d, day %d, event %d notes are too long (max 5000 characters).", i+1, j+1, k+1)
				}
				if len(event.URL) > 2000 {
					return fmt.Errorf("Leg %d, day %d, event %d URL is too long (max 2000 characters).", i+1, j+1, k+1)
				}
				if !validStatuses[event.Status] {
					return fmt.Errorf("Leg %d, day %d, event %d has invalid status %q.", i+1, j+1, k+1, event.Status)
				}
				if event.Latitude < -90 || event.Latitude > 90 || event.Longitude < -180 || event.Longitude > 180 {
					if event.Latitude != 0 || event.Longitude != 0 {
						return fmt.Errorf("Leg %d, day %d, event %d has invalid coordinates.", i+1, j+1, k+1)
					}
				}
			}
		}
	}
	return nil
}
