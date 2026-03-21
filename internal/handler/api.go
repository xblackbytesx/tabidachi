package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xblackbytesx/tabidachi/internal/domain"
	"github.com/xblackbytesx/tabidachi/internal/repository"
	"github.com/labstack/echo/v4"
)

// APIHandler serves the JSON API used by external clients (e.g. Android app).
type APIHandler struct {
	trips   *repository.TripStore
	baseURL string // e.g. "https://tabidachi.example.com" — used to make image URLs absolute
}

func NewAPIHandler(trips *repository.TripStore, baseURL string) *APIHandler {
	return &APIHandler{trips: trips, baseURL: baseURL}
}

// tripSummary is the lightweight representation returned by GET /api/v1/trips.
type tripSummary struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	StartDate        string    `json:"startDate"`
	EndDate          string    `json:"endDate"`
	HomeLocation     string    `json:"homeLocation,omitempty"`
	Timezone         string    `json:"timezone,omitempty"`
	CoverColor       string    `json:"coverColor,omitempty"`
	CoverImageURL    string    `json:"coverImageUrl,omitempty"`
	CoverImageCredit string    `json:"coverImageCredit,omitempty"`
	LegCount         int       `json:"legCount"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// tripDetail is the full representation returned by GET /api/v1/trips/:id.
type tripDetail struct {
	tripSummary
	Data domain.TripData `json:"data"`
}

// ListTrips handles GET /api/v1/trips
func (h *APIHandler) ListTrips(c echo.Context) error {
	userID, err := parseUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	trips, err := h.trips.List(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}

	summaries := make([]tripSummary, 0, len(trips))
	for _, t := range trips {
		summaries = append(summaries, h.toSummary(t))
	}
	return c.JSON(http.StatusOK, summaries)
}

// GetTrip handles GET /api/v1/trips/:id
func (h *APIHandler) GetTrip(c echo.Context) error {
	userID, err := parseUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	tripID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}

	trip, err := h.trips.GetByID(c.Request().Context(), tripID, userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}

	detail := tripDetail{
		tripSummary: h.toSummary(trip),
		Data:        trip.Data,
	}
	// Make all image URLs absolute and rewrite /uploads/ for Bearer token auth.
	for i, leg := range detail.Data.Legs {
		detail.Data.Legs[i].CoverImageURL = h.absImageURL(leg.CoverImageURL)
		for j, day := range leg.Days {
			for k, ev := range day.Events {
				detail.Data.Legs[i].Days[j].Events[k].ImageURL = h.absImageURL(ev.ImageURL)
				detail.Data.Legs[i].Days[j].Events[k].ImageThumbURL = h.absImageURL(ev.ImageThumbURL)
			}
		}
	}
	return c.JSON(http.StatusOK, detail)
}

// absImageURL makes a relative image path into an absolute URL suitable for
// API clients.  It also rewrites /uploads/ to /api/v1/uploads/ so that Bearer
// token auth works (the session-protected /uploads/ route does not accept API
// tokens).
func (h *APIHandler) absImageURL(path string) string {
	if path == "" {
		return ""
	}
	url := path
	if url[0] == '/' {
		url = h.baseURL + url
	}
	url = strings.Replace(url, "/uploads/", "/api/v1/uploads/", 1)
	return url
}

func (h *APIHandler) toSummary(t *domain.Trip) tripSummary {
	return tripSummary{
		ID:               t.ID.String(),
		Title:            t.Title,
		StartDate:        t.StartDate.Format("2006-01-02"),
		EndDate:          t.EndDate.Format("2006-01-02"),
		HomeLocation:     t.HomeLocation,
		Timezone:         t.Timezone,
		CoverColor:       t.CoverColor,
		CoverImageURL:    h.absImageURL(t.CoverImageURL),
		CoverImageCredit: t.CoverImageCredit,
		LegCount:         len(t.Data.Legs),
		UpdatedAt:        t.UpdatedAt,
	}
}
