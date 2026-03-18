package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/xblackbytesx/tabidachi/internal/domain"
	"github.com/xblackbytesx/tabidachi/internal/images"
	"github.com/xblackbytesx/tabidachi/internal/repository"
	"github.com/xblackbytesx/tabidachi/web/templates/pages"
	"github.com/labstack/echo/v4"
)

type TripHandler struct {
	trips    *repository.TripStore
	imageSvc *images.Service
}

func NewTripHandler(trips *repository.TripStore, imageSvc *images.Service) *TripHandler {
	return &TripHandler{trips: trips, imageSvc: imageSvc}
}

func (h *TripHandler) NewMethod(c echo.Context) error {
	return render(c, http.StatusOK, pages.TripNew(csrfToken(c)))
}

func (h *TripHandler) NewScratch(c echo.Context) error {
	return render(c, http.StatusOK, pages.TripScratch(csrfToken(c)))
}

func (h *TripHandler) Create(c echo.Context) error {
	uid, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}

	title := c.FormValue("title")
	startStr := c.FormValue("start_date")
	endStr := c.FormValue("end_date")
	homeLocation := c.FormValue("home_location")
	timezone := c.FormValue("timezone")
	if timezone == "" {
		timezone = "UTC"
	}
	coverColor := c.FormValue("cover_color")

	if title == "" || startStr == "" || endStr == "" {
		return render(c, http.StatusOK, pages.TripScratch(csrfToken(c)))
	}

	startDate, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		return render(c, http.StatusOK, pages.TripScratch(csrfToken(c)))
	}
	endDate, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		return render(c, http.StatusOK, pages.TripScratch(csrfToken(c)))
	}

	trip := &domain.Trip{
		UserID:       uid,
		Title:        title,
		StartDate:    startDate,
		EndDate:      endDate,
		HomeLocation: homeLocation,
		Timezone:     timezone,
		CoverColor:   coverColor,
		Data: domain.TripData{
			SchemaVersion: "1.0",
			Title:         title,
			StartDate:     startStr,
			EndDate:       endStr,
			HomeLocation:  homeLocation,
			Timezone:      timezone,
			Legs:          []domain.Leg{},
		},
	}

	created, err := h.trips.Create(c.Request().Context(), trip)
	if err != nil {
		slog.Error("trip create", "err", err)
		return render(c, http.StatusOK, pages.TripScratch(csrfToken(c)))
	}

	go images.AutoFetch(h.trips, h.imageSvc, created.ID, created.UserID)

	return redirect(c, "/trips/"+created.ID.String()+"/edit")
}

func (h *TripHandler) View(c echo.Context) error {
	uid, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}

	tripID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	trip, err := h.trips.GetByID(c.Request().Context(), tripID, uid)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	return render(c, http.StatusOK, pages.TripView(csrfToken(c), trip))
}

func (h *TripHandler) Edit(c echo.Context) error {
	uid, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}

	tripID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	trip, err := h.trips.GetByID(c.Request().Context(), tripID, uid)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	return render(c, http.StatusOK, pages.TripEdit(csrfToken(c), trip))
}

func (h *TripHandler) Update(c echo.Context) error {
	uid, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}

	tripID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	trip, err := h.trips.GetByID(c.Request().Context(), tripID, uid)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	title := c.FormValue("title")
	startStr := c.FormValue("start_date")
	endStr := c.FormValue("end_date")

	if title == "" || startStr == "" || endStr == "" {
		return render(c, http.StatusOK, pages.TripEdit(csrfToken(c), trip))
	}

	startDate, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		return render(c, http.StatusOK, pages.TripEdit(csrfToken(c), trip))
	}
	endDate, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		return render(c, http.StatusOK, pages.TripEdit(csrfToken(c), trip))
	}

	trip.Title = title
	trip.StartDate = startDate
	trip.EndDate = endDate
	trip.HomeLocation = c.FormValue("home_location")
	trip.Timezone = c.FormValue("timezone")
	if trip.Timezone == "" {
		trip.Timezone = "UTC"
	}
	trip.CoverColor = c.FormValue("cover_color")

	// Update metadata fields in data too
	trip.Data.Title = title
	trip.Data.StartDate = startStr
	trip.Data.EndDate = endStr
	trip.Data.HomeLocation = trip.HomeLocation
	trip.Data.Timezone = trip.Timezone

	if err := h.trips.Update(c.Request().Context(), trip); err != nil {
		slog.Error("trip update", "err", err)
		return render(c, http.StatusOK, pages.TripEdit(csrfToken(c), trip))
	}

	return redirect(c, "/trips/"+trip.ID.String()+"/edit")
}

func (h *TripHandler) Export(c echo.Context) error {
	uid, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}

	tripID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	trip, err := h.trips.GetByID(c.Request().Context(), tripID, uid)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	out, err := json.MarshalIndent(trip.Data, "", "  ")
	if err != nil {
		return c.String(http.StatusInternalServerError, "export failed")
	}

	filename := fmt.Sprintf("%s.json", slugify(trip.Title))
	c.Response().Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	return c.Blob(http.StatusOK, "application/json", out)
}

func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) || r == '-' {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func (h *TripHandler) Delete(c echo.Context) error {
	uid, err := parseUserID(c)
	if err != nil {
		return c.String(http.StatusUnauthorized, "unauthorized")
	}

	tripID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.String(http.StatusNotFound, "not found")
	}

	if err := h.trips.Delete(c.Request().Context(), tripID, uid); err != nil {
		slog.Error("trip delete", "err", err)
		return c.String(http.StatusInternalServerError, "error")
	}

	if isHTMX(c) {
		c.Response().Header().Set("HX-Redirect", "/")
		return c.String(http.StatusOK, "")
	}
	return redirect(c, "/")
}
