package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xblackbytesx/tabidachi/internal/domain"
	"github.com/xblackbytesx/tabidachi/internal/repository"
	"github.com/xblackbytesx/tabidachi/web/templates/pages"
	"github.com/labstack/echo/v4"
)

type BuilderHandler struct {
	trips *repository.TripStore
}

func NewBuilderHandler(trips *repository.TripStore) *BuilderHandler {
	return &BuilderHandler{trips: trips}
}

func (h *BuilderHandler) loadTrip(c echo.Context) (*domain.Trip, uuid.UUID, error) {
	uid, err := parseUserID(c)
	if err != nil {
		return nil, uuid.Nil, err
	}
	tripID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return nil, uuid.Nil, err
	}
	trip, err := h.trips.GetByID(c.Request().Context(), tripID, uid)
	if err != nil {
		return nil, uuid.Nil, err
	}
	return trip, uid, nil
}

func (h *BuilderHandler) saveAndRedirect(c echo.Context, trip *domain.Trip) error {
	if err := h.trips.Update(c.Request().Context(), trip); err != nil {
		slog.Error("builder: update trip", "err", err)
		return c.String(http.StatusInternalServerError, "error saving trip")
	}
	if isHTMX(c) {
		c.Response().Header().Set("HX-Redirect", "/trips/"+trip.ID.String()+"/edit")
		return c.String(http.StatusOK, "")
	}
	return redirect(c, "/trips/"+trip.ID.String()+"/edit")
}

// saveAndRenderDay persists the trip then either returns the re-rendered
// DayBuilder fragment (for fetch-based JS callers) or falls back to a redirect.
func (h *BuilderHandler) saveAndRenderDay(c echo.Context, trip *domain.Trip, legIdx, dayIdx int) error {
	if err := h.trips.Update(c.Request().Context(), trip); err != nil {
		slog.Error("builder: update trip", "err", err)
		return c.String(http.StatusInternalServerError, "error saving trip")
	}
	// Requests from the async event JS handler include X-Day-Refresh.
	if c.Request().Header.Get("X-Day-Refresh") == "true" {
		return render(c, http.StatusOK, pages.DayBuilder(
			csrfToken(c),
			trip.ID.String(),
			trip.Data.Legs[legIdx],
			legIdx,
			trip.Data.Legs[legIdx].Days[dayIdx],
			dayIdx,
		))
	}
	// Fallback: redirect (works for non-JS and HTMX form submissions).
	return h.saveAndRedirect(c, trip)
}

// parseDate parses an ISO date string, returning zero time on failure.
func parseDate(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", s)
	return t, err == nil
}

// parseDateTimeLoose parses an ISO 8601 datetime, trying several common formats.
func parseDateTimeLoose(s string) (time.Time, bool) {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// validateTimeFormat checks that a time string is HH:MM or empty.
func validateTimeFormat(s string) bool {
	if s == "" {
		return true
	}
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	h, err1 := strconv.Atoi(s[:2])
	m, err2 := strconv.Atoi(s[3:])
	return err1 == nil && err2 == nil && h >= 0 && h <= 23 && m >= 0 && m <= 59
}

// AddLeg adds a new leg to the trip.
func (h *BuilderHandler) AddLeg(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	destination := c.FormValue("destination")
	region := c.FormValue("region")
	startDate := c.FormValue("start_date")
	endDate := c.FormValue("end_date")
	notes := c.FormValue("notes")

	if destination == "" || startDate == "" || endDate == "" {
		return c.String(http.StatusBadRequest, "destination, start_date and end_date are required")
	}
	if len(destination) > 500 || len(region) > 500 || len(notes) > 5000 {
		return c.String(http.StatusBadRequest, "field value too long")
	}

	start, okS := parseDate(startDate)
	end, okE := parseDate(endDate)
	if !okS || !okE {
		return c.String(http.StatusBadRequest, "invalid date format (expected YYYY-MM-DD)")
	}
	if end.Before(start) {
		return c.String(http.StatusBadRequest, "end date must be on or after start date")
	}

	tripStart := trip.StartDate.Truncate(24 * time.Hour)
	tripEnd := trip.EndDate.Truncate(24 * time.Hour)
	if !tripStart.IsZero() && !tripEnd.IsZero() {
		if start.Before(tripStart) || end.After(tripEnd) {
			return c.String(http.StatusBadRequest, "leg dates must be within the trip dates ("+tripStart.Format("2006-01-02")+" to "+tripEnd.Format("2006-01-02")+")")
		}
	}

	leg := domain.Leg{
		Sequence:    len(trip.Data.Legs) + 1,
		Destination: destination,
		Region:      region,
		StartDate:   startDate,
		EndDate:     endDate,
		Notes:       notes,
		Days:        []domain.Day{},
	}
	trip.Data.Legs = append(trip.Data.Legs, leg)
	return h.saveAndRedirect(c, trip)
}

// DeleteLeg removes a leg by index.
func (h *BuilderHandler) DeleteLeg(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	idx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || idx < 0 || idx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}

	trip.Data.Legs = append(trip.Data.Legs[:idx], trip.Data.Legs[idx+1:]...)
	// Re-sequence
	for i := range trip.Data.Legs {
		trip.Data.Legs[i].Sequence = i + 1
	}
	return h.saveAndRedirect(c, trip)
}

// UpdateAccommodation sets or updates the accommodation for a leg.
func (h *BuilderHandler) UpdateAccommodation(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	idx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || idx < 0 || idx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}

	name := c.FormValue("name")
	if name == "" {
		trip.Data.Legs[idx].Accommodation = nil
	} else {
		checkIn := c.FormValue("check_in")
		checkOut := c.FormValue("check_out")

		// #5: validate check-in is before check-out when both are provided
		if checkIn != "" && checkOut != "" {
			ciTime, okCI := parseDateTimeLoose(checkIn)
			coTime, okCO := parseDateTimeLoose(checkOut)
			if okCI && okCO && !coTime.After(ciTime) {
				return c.String(http.StatusBadRequest, "check-out must be after check-in")
			}
		}

		trip.Data.Legs[idx].Accommodation = &domain.Accommodation{
			Name:             name,
			Neighborhood:     c.FormValue("neighborhood"),
			Address:          c.FormValue("address"),
			CheckIn:          checkIn,
			CheckOut:         checkOut,
			BookingReference: c.FormValue("booking_reference"),
		}
	}
	return h.saveAndRedirect(c, trip)
}

// AddDay adds a day to a leg.
func (h *BuilderHandler) AddDay(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	idx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || idx < 0 || idx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}

	date := c.FormValue("date")
	if date == "" {
		return c.String(http.StatusBadRequest, "date is required")
	}
	if _, ok := parseDate(date); !ok {
		return c.String(http.StatusBadRequest, "invalid date format (expected YYYY-MM-DD)")
	}

	// #7: prevent duplicate day dates within this leg
	leg := trip.Data.Legs[idx]
	for _, existing := range leg.Days {
		if existing.Date == date {
			return c.String(http.StatusBadRequest, "a day with date "+date+" already exists in this leg")
		}
	}

	dayType := c.FormValue("type")
	if dayType == "" {
		dayType = "normal"
	}

	day := domain.Day{
		Date:   date,
		Label:  c.FormValue("label"),
		Type:   dayType,
		Notes:  c.FormValue("notes"),
		Events: []domain.Event{},
	}
	trip.Data.Legs[idx].Days = append(trip.Data.Legs[idx].Days, day)
	return h.saveAndRedirect(c, trip)
}

// DeleteDay removes a day from a leg by index.
func (h *BuilderHandler) DeleteDay(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	legIdx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || legIdx < 0 || legIdx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}
	dayIdx, err := strconv.Atoi(c.Param("dayIdx"))
	if err != nil || dayIdx < 0 || dayIdx >= len(trip.Data.Legs[legIdx].Days) {
		return c.String(http.StatusBadRequest, "invalid day index")
	}

	days := trip.Data.Legs[legIdx].Days
	trip.Data.Legs[legIdx].Days = append(days[:dayIdx], days[dayIdx+1:]...)
	return h.saveAndRedirect(c, trip)
}

// AddEvent adds an event to a day.
func (h *BuilderHandler) AddEvent(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	legIdx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || legIdx < 0 || legIdx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}
	dayIdx, err := strconv.Atoi(c.Param("dayIdx"))
	if err != nil || dayIdx < 0 || dayIdx >= len(trip.Data.Legs[legIdx].Days) {
		return c.String(http.StatusBadRequest, "invalid day index")
	}

	eventType := c.FormValue("event_type")
	title := c.FormValue("title")
	if title == "" || eventType == "" {
		return c.String(http.StatusBadRequest, "event_type and title are required")
	}
	if len(title) > 500 || len(c.FormValue("notes")) > 5000 || len(c.FormValue("location")) > 500 {
		return c.String(http.StatusBadRequest, "field value too long")
	}

	// #6: validate time format (HH:MM)
	startTime := c.FormValue("start_time")
	endTime := c.FormValue("end_time")
	if !validateTimeFormat(startTime) {
		return c.String(http.StatusBadRequest, "invalid start time format (expected HH:MM)")
	}
	if !validateTimeFormat(endTime) {
		return c.String(http.StatusBadRequest, "invalid end time format (expected HH:MM)")
	}

	day := &trip.Data.Legs[legIdx].Days[dayIdx]
	event := domain.Event{
		Sequence:  len(day.Events) + 1,
		Type:      eventType,
		Title:     title,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  c.FormValue("duration"),
		Notes:     c.FormValue("notes"),
		Optional:  c.FormValue("optional") == "on",
		URL:       c.FormValue("url"),
	}

	switch eventType {
	case "activity":
		event.Location = c.FormValue("location")
		event.Address = c.FormValue("address")
		event.Latitude = c.FormValue("latitude")
		event.Longitude = c.FormValue("longitude")
		event.TicketRequired = c.FormValue("ticket_required") == "on"
		event.BookingReference = c.FormValue("booking_reference")
	case "transit":
		event.TransportMode = c.FormValue("transport_mode")
		event.Carrier = c.FormValue("carrier")
		event.FlightNumber = c.FormValue("flight_number")
		event.TrackFlight = c.FormValue("track_flight") == "on"
		depLoc := c.FormValue("departure_location")
		depCode := c.FormValue("departure_code")
		if depLoc != "" {
			event.Departure = &domain.TransitPoint{
				Location:  depLoc,
				Code:      depCode,
				Latitude:  c.FormValue("departure_latitude"),
				Longitude: c.FormValue("departure_longitude"),
			}
		}
		arrLoc := c.FormValue("arrival_location")
		arrCode := c.FormValue("arrival_code")
		if arrLoc != "" {
			event.Arrival = &domain.TransitPoint{
				Location:  arrLoc,
				Code:      arrCode,
				Latitude:  c.FormValue("arrival_latitude"),
				Longitude: c.FormValue("arrival_longitude"),
			}
		}
	case "accommodation":
		event.CheckIn = c.FormValue("check_in") == "on"
		event.CheckOut = c.FormValue("check_out") == "on"
	}

	day.Events = append(day.Events, event)
	return h.saveAndRenderDay(c, trip, legIdx, dayIdx)
}

// UpdateDay updates a day's label, type, and notes.
func (h *BuilderHandler) UpdateDay(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	legIdx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || legIdx < 0 || legIdx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}
	dayIdx, err := strconv.Atoi(c.Param("dayIdx"))
	if err != nil || dayIdx < 0 || dayIdx >= len(trip.Data.Legs[legIdx].Days) {
		return c.String(http.StatusBadRequest, "invalid day index")
	}

	dayType := c.FormValue("type")
	if dayType == "" {
		dayType = "normal"
	}
	if len(c.FormValue("label")) > 500 || len(c.FormValue("notes")) > 5000 {
		return c.String(http.StatusBadRequest, "field value too long")
	}

	trip.Data.Legs[legIdx].Days[dayIdx].Label = c.FormValue("label")
	trip.Data.Legs[legIdx].Days[dayIdx].Type = dayType
	trip.Data.Legs[legIdx].Days[dayIdx].Notes = c.FormValue("notes")

	if err := h.trips.Update(c.Request().Context(), trip); err != nil {
		slog.Error("builder: update day", "err", err)
		return c.String(http.StatusInternalServerError, "error saving trip")
	}

	if isHTMX(c) {
		return render(c, http.StatusOK, pages.DayBuilder(
			csrfToken(c),
			trip.ID.String(),
			trip.Data.Legs[legIdx],
			legIdx,
			trip.Data.Legs[legIdx].Days[dayIdx],
			dayIdx,
		))
	}
	return redirect(c, "/trips/"+trip.ID.String()+"/edit")
}

// UpdateEvent updates an existing event in a day.
func (h *BuilderHandler) UpdateEvent(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	legIdx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || legIdx < 0 || legIdx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}
	dayIdx, err := strconv.Atoi(c.Param("dayIdx"))
	if err != nil || dayIdx < 0 || dayIdx >= len(trip.Data.Legs[legIdx].Days) {
		return c.String(http.StatusBadRequest, "invalid day index")
	}
	evtIdx, err := strconv.Atoi(c.Param("eventIdx"))
	if err != nil || evtIdx < 0 || evtIdx >= len(trip.Data.Legs[legIdx].Days[dayIdx].Events) {
		return c.String(http.StatusBadRequest, "invalid event index")
	}

	eventType := c.FormValue("event_type")
	title := c.FormValue("title")
	if title == "" || eventType == "" {
		return c.String(http.StatusBadRequest, "event_type and title are required")
	}
	if len(title) > 500 || len(c.FormValue("notes")) > 5000 || len(c.FormValue("location")) > 500 {
		return c.String(http.StatusBadRequest, "field value too long")
	}

	startTime := c.FormValue("start_time")
	endTime := c.FormValue("end_time")
	if !validateTimeFormat(startTime) {
		return c.String(http.StatusBadRequest, "invalid start time format (expected HH:MM)")
	}
	if !validateTimeFormat(endTime) {
		return c.String(http.StatusBadRequest, "invalid end time format (expected HH:MM)")
	}

	existing := &trip.Data.Legs[legIdx].Days[dayIdx].Events[evtIdx]
	existing.Type = eventType
	existing.Title = title
	existing.StartTime = startTime
	existing.EndTime = endTime
	existing.Duration = c.FormValue("duration")
	existing.Notes = c.FormValue("notes")
	existing.Optional = c.FormValue("optional") == "on"
	existing.URL = c.FormValue("url")

	// Reset type-specific fields before repopulating
	existing.Location = ""
	existing.Address = ""
	existing.Latitude = ""
	existing.Longitude = ""
	existing.TicketRequired = false
	existing.BookingReference = ""
	existing.TransportMode = ""
	existing.Departure = nil
	existing.Arrival = nil
	existing.Carrier = ""
	existing.FlightNumber = ""
	existing.TrackFlight = false
	existing.CheckIn = false
	existing.CheckOut = false

	switch eventType {
	case "activity":
		existing.Location = c.FormValue("location")
		existing.Address = c.FormValue("address")
		existing.Latitude = c.FormValue("latitude")
		existing.Longitude = c.FormValue("longitude")
		existing.TicketRequired = c.FormValue("ticket_required") == "on"
		existing.BookingReference = c.FormValue("booking_reference")
	case "transit":
		existing.TransportMode = c.FormValue("transport_mode")
		existing.Carrier = c.FormValue("carrier")
		existing.FlightNumber = c.FormValue("flight_number")
		existing.TrackFlight = c.FormValue("track_flight") == "on"
		existing.Duration = c.FormValue("duration")
		depLoc := c.FormValue("departure_location")
		depCode := c.FormValue("departure_code")
		if depLoc != "" {
			existing.Departure = &domain.TransitPoint{
				Location:  depLoc,
				Code:      depCode,
				Latitude:  c.FormValue("departure_latitude"),
				Longitude: c.FormValue("departure_longitude"),
			}
		}
		arrLoc := c.FormValue("arrival_location")
		arrCode := c.FormValue("arrival_code")
		if arrLoc != "" {
			existing.Arrival = &domain.TransitPoint{
				Location:  arrLoc,
				Code:      arrCode,
				Latitude:  c.FormValue("arrival_latitude"),
				Longitude: c.FormValue("arrival_longitude"),
			}
		}
	case "accommodation":
		existing.CheckIn = c.FormValue("check_in") == "on"
		existing.CheckOut = c.FormValue("check_out") == "on"
	}

	return h.saveAndRenderDay(c, trip, legIdx, dayIdx)
}

// DeleteEvent removes an event from a day.
func (h *BuilderHandler) DeleteEvent(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	legIdx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || legIdx < 0 || legIdx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}
	dayIdx, err := strconv.Atoi(c.Param("dayIdx"))
	if err != nil || dayIdx < 0 || dayIdx >= len(trip.Data.Legs[legIdx].Days) {
		return c.String(http.StatusBadRequest, "invalid day index")
	}
	evtIdx, err := strconv.Atoi(c.Param("eventIdx"))
	if err != nil || evtIdx < 0 || evtIdx >= len(trip.Data.Legs[legIdx].Days[dayIdx].Events) {
		return c.String(http.StatusBadRequest, "invalid event index")
	}

	events := trip.Data.Legs[legIdx].Days[dayIdx].Events
	trip.Data.Legs[legIdx].Days[dayIdx].Events = append(events[:evtIdx], events[evtIdx+1:]...)
	// Re-sequence
	for i := range trip.Data.Legs[legIdx].Days[dayIdx].Events {
		trip.Data.Legs[legIdx].Days[dayIdx].Events[i].Sequence = i + 1
	}
	return h.saveAndRenderDay(c, trip, legIdx, dayIdx)
}

// AddOption adds an alternative plan option to a day.
func (h *BuilderHandler) AddOption(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}
	legIdx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || legIdx < 0 || legIdx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}
	dayIdx, err := strconv.Atoi(c.Param("dayIdx"))
	if err != nil || dayIdx < 0 || dayIdx >= len(trip.Data.Legs[legIdx].Days) {
		return c.String(http.StatusBadRequest, "invalid day index")
	}

	title := strings.TrimSpace(c.FormValue("title"))
	if title == "" {
		return c.String(http.StatusBadRequest, "title is required")
	}
	if len(title) > 200 || len(c.FormValue("description")) > 500 || len(c.FormValue("location")) > 500 {
		return c.String(http.StatusBadRequest, "field value too long")
	}

	day := &trip.Data.Legs[legIdx].Days[dayIdx]
	opt := domain.DayOption{
		Sequence:    len(day.Options) + 1,
		Title:       title,
		Description: c.FormValue("description"),
		Location:    c.FormValue("location"),
		Events:      []domain.Event{},
	}
	day.Options = append(day.Options, opt)
	return h.saveAndRenderDay(c, trip, legIdx, dayIdx)
}

// UpdateOption edits the title, description, and location of an alternative.
func (h *BuilderHandler) UpdateOption(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}
	legIdx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || legIdx < 0 || legIdx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}
	dayIdx, err := strconv.Atoi(c.Param("dayIdx"))
	if err != nil || dayIdx < 0 || dayIdx >= len(trip.Data.Legs[legIdx].Days) {
		return c.String(http.StatusBadRequest, "invalid day index")
	}
	optIdx, err := strconv.Atoi(c.Param("optIdx"))
	if err != nil || optIdx < 0 || optIdx >= len(trip.Data.Legs[legIdx].Days[dayIdx].Options) {
		return c.String(http.StatusBadRequest, "invalid option index")
	}

	title := strings.TrimSpace(c.FormValue("title"))
	if title == "" {
		return c.String(http.StatusBadRequest, "title is required")
	}
	if len(title) > 200 || len(c.FormValue("description")) > 500 || len(c.FormValue("location")) > 500 {
		return c.String(http.StatusBadRequest, "field value too long")
	}

	opt := &trip.Data.Legs[legIdx].Days[dayIdx].Options[optIdx]
	opt.Title = title
	opt.Description = c.FormValue("description")
	opt.Location = c.FormValue("location")
	return h.saveAndRenderDay(c, trip, legIdx, dayIdx)
}

// DeleteOption removes an alternative from a day.
func (h *BuilderHandler) DeleteOption(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}
	legIdx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || legIdx < 0 || legIdx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}
	dayIdx, err := strconv.Atoi(c.Param("dayIdx"))
	if err != nil || dayIdx < 0 || dayIdx >= len(trip.Data.Legs[legIdx].Days) {
		return c.String(http.StatusBadRequest, "invalid day index")
	}
	optIdx, err := strconv.Atoi(c.Param("optIdx"))
	if err != nil || optIdx < 0 || optIdx >= len(trip.Data.Legs[legIdx].Days[dayIdx].Options) {
		return c.String(http.StatusBadRequest, "invalid option index")
	}

	day := &trip.Data.Legs[legIdx].Days[dayIdx]
	day.Options = append(day.Options[:optIdx], day.Options[optIdx+1:]...)
	for i := range day.Options {
		day.Options[i].Sequence = i + 1
	}
	return h.saveAndRenderDay(c, trip, legIdx, dayIdx)
}

// SelectOption toggles the selected state of an alternative.
// Selecting an already-selected option deselects it; otherwise the chosen option
// is marked selected and all others are deselected.
func (h *BuilderHandler) SelectOption(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}
	legIdx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || legIdx < 0 || legIdx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}
	dayIdx, err := strconv.Atoi(c.Param("dayIdx"))
	if err != nil || dayIdx < 0 || dayIdx >= len(trip.Data.Legs[legIdx].Days) {
		return c.String(http.StatusBadRequest, "invalid day index")
	}
	optIdx, err := strconv.Atoi(c.Param("optIdx"))
	if err != nil || optIdx < 0 || optIdx >= len(trip.Data.Legs[legIdx].Days[dayIdx].Options) {
		return c.String(http.StatusBadRequest, "invalid option index")
	}

	day := &trip.Data.Legs[legIdx].Days[dayIdx]
	alreadySelected := day.Options[optIdx].Selected
	for i := range day.Options {
		day.Options[i].Selected = false
	}
	if !alreadySelected {
		day.Options[optIdx].Selected = true
	}

	if err := h.trips.Update(c.Request().Context(), trip); err != nil {
		slog.Error("builder: select option", "err", err)
		return c.String(http.StatusInternalServerError, "error saving trip")
	}

	// Builder context: JS async form sends X-Day-Refresh to re-render the DayBuilder fragment.
	if c.Request().Header.Get("X-Day-Refresh") == "true" {
		return render(c, http.StatusOK, pages.DayBuilder(
			csrfToken(c),
			trip.ID.String(),
			trip.Data.Legs[legIdx],
			legIdx,
			trip.Data.Legs[legIdx].Days[dayIdx],
			dayIdx,
		))
	}
	// Trip view context: return the re-rendered day section so HTMX can swap it
	// in place without a page reload. Falls back to a plain redirect for non-HTMX.
	if isHTMX(c) {
		return render(c, http.StatusOK, pages.DaySection(
			csrfToken(c),
			trip,
			trip.Data.Legs[legIdx].Days[dayIdx],
			legIdx,
			dayIdx,
		))
	}
	return redirect(c, "/trips/"+trip.ID.String()+"#day-"+day.Date)
}

// buildEventFromForm constructs an Event from form values (shared by AddOptionEvent / UpdateOptionEvent).
func buildEventFromForm(c echo.Context, seq int) (domain.Event, error) {
	eventType := c.FormValue("event_type")
	title := c.FormValue("title")
	if title == "" || eventType == "" {
		return domain.Event{}, fmt.Errorf("event_type and title are required")
	}
	if len(title) > 500 || len(c.FormValue("notes")) > 5000 || len(c.FormValue("location")) > 500 {
		return domain.Event{}, fmt.Errorf("field value too long")
	}
	startTime := c.FormValue("start_time")
	endTime := c.FormValue("end_time")
	if !validateTimeFormat(startTime) {
		return domain.Event{}, fmt.Errorf("invalid start time format (expected HH:MM)")
	}
	if !validateTimeFormat(endTime) {
		return domain.Event{}, fmt.Errorf("invalid end time format (expected HH:MM)")
	}
	event := domain.Event{
		Sequence:  seq,
		Type:      eventType,
		Title:     title,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  c.FormValue("duration"),
		Notes:     c.FormValue("notes"),
		Optional:  c.FormValue("optional") == "on",
		URL:       c.FormValue("url"),
	}
	switch eventType {
	case "activity":
		event.Location = c.FormValue("location")
		event.Address = c.FormValue("address")
		event.Latitude = c.FormValue("latitude")
		event.Longitude = c.FormValue("longitude")
		event.TicketRequired = c.FormValue("ticket_required") == "on"
		event.BookingReference = c.FormValue("booking_reference")
	case "transit":
		event.TransportMode = c.FormValue("transport_mode")
		event.Carrier = c.FormValue("carrier")
		event.FlightNumber = c.FormValue("flight_number")
		event.TrackFlight = c.FormValue("track_flight") == "on"
		depLoc := c.FormValue("departure_location")
		if depLoc != "" {
			event.Departure = &domain.TransitPoint{
				Location:  depLoc,
				Code:      c.FormValue("departure_code"),
				Latitude:  c.FormValue("departure_latitude"),
				Longitude: c.FormValue("departure_longitude"),
			}
		}
		arrLoc := c.FormValue("arrival_location")
		if arrLoc != "" {
			event.Arrival = &domain.TransitPoint{
				Location:  arrLoc,
				Code:      c.FormValue("arrival_code"),
				Latitude:  c.FormValue("arrival_latitude"),
				Longitude: c.FormValue("arrival_longitude"),
			}
		}
	case "accommodation":
		event.CheckIn = c.FormValue("check_in") == "on"
		event.CheckOut = c.FormValue("check_out") == "on"
	}
	return event, nil
}

// AddOptionEvent adds an event to an alternative's event list.
func (h *BuilderHandler) AddOptionEvent(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}
	legIdx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || legIdx < 0 || legIdx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}
	dayIdx, err := strconv.Atoi(c.Param("dayIdx"))
	if err != nil || dayIdx < 0 || dayIdx >= len(trip.Data.Legs[legIdx].Days) {
		return c.String(http.StatusBadRequest, "invalid day index")
	}
	optIdx, err := strconv.Atoi(c.Param("optIdx"))
	if err != nil || optIdx < 0 || optIdx >= len(trip.Data.Legs[legIdx].Days[dayIdx].Options) {
		return c.String(http.StatusBadRequest, "invalid option index")
	}

	opt := &trip.Data.Legs[legIdx].Days[dayIdx].Options[optIdx]
	event, err := buildEventFromForm(c, len(opt.Events)+1)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	opt.Events = append(opt.Events, event)
	return h.saveAndRenderDay(c, trip, legIdx, dayIdx)
}

// UpdateOptionEvent edits an event within an alternative.
func (h *BuilderHandler) UpdateOptionEvent(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}
	legIdx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || legIdx < 0 || legIdx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}
	dayIdx, err := strconv.Atoi(c.Param("dayIdx"))
	if err != nil || dayIdx < 0 || dayIdx >= len(trip.Data.Legs[legIdx].Days) {
		return c.String(http.StatusBadRequest, "invalid day index")
	}
	optIdx, err := strconv.Atoi(c.Param("optIdx"))
	if err != nil || optIdx < 0 || optIdx >= len(trip.Data.Legs[legIdx].Days[dayIdx].Options) {
		return c.String(http.StatusBadRequest, "invalid option index")
	}
	evtIdx, err := strconv.Atoi(c.Param("eventIdx"))
	opt := &trip.Data.Legs[legIdx].Days[dayIdx].Options[optIdx]
	if err != nil || evtIdx < 0 || evtIdx >= len(opt.Events) {
		return c.String(http.StatusBadRequest, "invalid event index")
	}

	event, err := buildEventFromForm(c, opt.Events[evtIdx].Sequence)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	// Preserve image fields from existing event
	event.ImageURL = opt.Events[evtIdx].ImageURL
	event.ImageThumbURL = opt.Events[evtIdx].ImageThumbURL
	event.ImageCredit = opt.Events[evtIdx].ImageCredit
	opt.Events[evtIdx] = event
	return h.saveAndRenderDay(c, trip, legIdx, dayIdx)
}

// DeleteOptionEvent removes an event from an alternative.
func (h *BuilderHandler) DeleteOptionEvent(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}
	legIdx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || legIdx < 0 || legIdx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}
	dayIdx, err := strconv.Atoi(c.Param("dayIdx"))
	if err != nil || dayIdx < 0 || dayIdx >= len(trip.Data.Legs[legIdx].Days) {
		return c.String(http.StatusBadRequest, "invalid day index")
	}
	optIdx, err := strconv.Atoi(c.Param("optIdx"))
	if err != nil || optIdx < 0 || optIdx >= len(trip.Data.Legs[legIdx].Days[dayIdx].Options) {
		return c.String(http.StatusBadRequest, "invalid option index")
	}
	opt := &trip.Data.Legs[legIdx].Days[dayIdx].Options[optIdx]
	evtIdx, err := strconv.Atoi(c.Param("eventIdx"))
	if err != nil || evtIdx < 0 || evtIdx >= len(opt.Events) {
		return c.String(http.StatusBadRequest, "invalid event index")
	}

	opt.Events = append(opt.Events[:evtIdx], opt.Events[evtIdx+1:]...)
	for i := range opt.Events {
		opt.Events[i].Sequence = i + 1
	}
	return h.saveAndRenderDay(c, trip, legIdx, dayIdx)
}

// ReorderEvents reorders events within a day based on the provided index order.
func (h *BuilderHandler) ReorderEvents(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}

	legIdx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil || legIdx < 0 || legIdx >= len(trip.Data.Legs) {
		return c.String(http.StatusBadRequest, "invalid leg index")
	}
	dayIdx, err := strconv.Atoi(c.Param("dayIdx"))
	if err != nil || dayIdx < 0 || dayIdx >= len(trip.Data.Legs[legIdx].Days) {
		return c.String(http.StatusBadRequest, "invalid day index")
	}

	var order []int
	if err := json.NewDecoder(c.Request().Body).Decode(&order); err != nil {
		return c.String(http.StatusBadRequest, "invalid JSON body")
	}

	events := trip.Data.Legs[legIdx].Days[dayIdx].Events
	if len(order) != len(events) {
		return c.String(http.StatusBadRequest, "order length mismatch")
	}

	// Validate indices
	seen := make(map[int]bool, len(order))
	for _, idx := range order {
		if idx < 0 || idx >= len(events) || seen[idx] {
			return c.String(http.StatusBadRequest, "invalid event index in order")
		}
		seen[idx] = true
	}

	// Build reordered slice
	reordered := make([]domain.Event, len(events))
	for newPos, oldIdx := range order {
		reordered[newPos] = events[oldIdx]
		reordered[newPos].Sequence = newPos + 1
	}
	trip.Data.Legs[legIdx].Days[dayIdx].Events = reordered

	if err := h.trips.Update(c.Request().Context(), trip); err != nil {
		slog.Error("builder: reorder events", "err", err)
		return c.String(http.StatusInternalServerError, "error saving trip")
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *BuilderHandler) UpdateData(c echo.Context) error {
	trip, _, err := h.loadTrip(c)
	if err != nil {
		return c.String(http.StatusNotFound, "trip not found")
	}
	raw := c.FormValue("data")
	var newData domain.TripData
	if jsonErr := json.Unmarshal([]byte(raw), &newData); jsonErr != nil {
		return render(c, http.StatusOK, pages.TripEdit(csrfToken(c), trip, raw, "Invalid JSON: "+jsonErr.Error()))
	}
	if valErr := validateTripData(&newData); valErr != nil {
		return render(c, http.StatusOK, pages.TripEdit(csrfToken(c), trip, raw, valErr.Error()))
	}
	trip.Data = newData
	if newData.Title != "" {
		trip.Title = newData.Title
	}
	if newData.StartDate != "" {
		if t, parseErr := time.Parse("2006-01-02", newData.StartDate); parseErr == nil {
			trip.StartDate = t
		}
	}
	if newData.EndDate != "" {
		if t, parseErr := time.Parse("2006-01-02", newData.EndDate); parseErr == nil {
			trip.EndDate = t
		}
	}
	trip.HomeLocation = newData.HomeLocation
	trip.Timezone = newData.Timezone
	if err := h.trips.Update(c.Request().Context(), trip); err != nil {
		slog.Error("builder: update data", "err", err)
		return render(c, http.StatusOK, pages.TripEdit(csrfToken(c), trip, raw, "Failed to save: "+err.Error()))
	}
	return redirect(c, "/trips/"+trip.ID.String()+"/edit")
}
