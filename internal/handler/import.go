package handler

import (
	"encoding/json"
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

	var data domain.TripData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return render(c, http.StatusOK, pages.TripImport(csrfToken(c), "Invalid JSON: "+err.Error()))
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
		data.SchemaVersion = "1.0"
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
