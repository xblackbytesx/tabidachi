package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
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

// durationRe matches human-friendly durations like "1h30m", "90min", "2h", "45m".
var durationRe = regexp.MustCompile(`(?i)^\s*(?:(\d+)\s*h(?:(?:ours?|rs?|))?\s*)?(?:(\d+)\s*m(?:in(?:utes?|s?)?)?)??\s*$`)

// parseDuration converts human-friendly duration strings to ISO 8601 format.
// Accepts: "1h30m", "90min", "2h", "45m", "1h 30m", "PT1H30M" (passed through).
// Returns empty string for empty input, original string if already ISO 8601.
func parseDuration(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Already ISO 8601 — pass through
	if strings.HasPrefix(s, "PT") || strings.HasPrefix(s, "P") {
		return s
	}
	m := durationRe.FindStringSubmatch(s)
	if m == nil {
		return s // unrecognised format, store as-is
	}
	hours, _ := strconv.Atoi(m[1])
	mins, _ := strconv.Atoi(m[2])
	if hours == 0 && mins == 0 {
		return s
	}
	result := "PT"
	if hours > 0 {
		result += fmt.Sprintf("%dH", hours)
	}
	if mins > 0 {
		result += fmt.Sprintf("%dM", mins)
	}
	return result
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

	// Duplicate day check — day dates must be unique within a leg.
	// Note: day dates are NOT constrained to the leg's date range because
	// departure/arrival days legitimately fall outside (e.g. departure day
	// before leg start, return-home day after leg end).
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

	eventURL := c.FormValue("url")
	if len(eventURL) > 2000 {
		return c.String(http.StatusBadRequest, "URL too long (max 2000 characters)")
	}

	day := &trip.Data.Legs[legIdx].Days[dayIdx]
	event := domain.Event{
		Sequence:  len(day.Events) + 1,
		Type:      eventType,
		Title:     title,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  parseDuration(c.FormValue("duration")),
		Notes:     c.FormValue("notes"),
		Optional:  c.FormValue("optional") == "on",
		URL:       eventURL,
		Status:    c.FormValue("status"),
	}

	switch eventType {
	case "activity":
		event.Location = c.FormValue("location")
		event.TicketRequired = c.FormValue("ticket_required") == "on"
		event.BookingReference = c.FormValue("booking_reference")
		if lat := c.FormValue("latitude"); lat != "" {
			if v, err := strconv.ParseFloat(lat, 64); err == nil {
				event.Latitude = v
			}
		}
		if lng := c.FormValue("longitude"); lng != "" {
			if v, err := strconv.ParseFloat(lng, 64); err == nil {
				event.Longitude = v
			}
		}
	case "transit":
		event.TransportMode = c.FormValue("transport_mode")
		event.Carrier = c.FormValue("carrier")
		event.FlightNumber = c.FormValue("flight_number")
		depLoc := c.FormValue("departure_location")
		depCode := c.FormValue("departure_code")
		if depLoc != "" {
			event.Departure = &domain.TransitPoint{Location: depLoc, Code: depCode}
		}
		arrLoc := c.FormValue("arrival_location")
		arrCode := c.FormValue("arrival_code")
		if arrLoc != "" {
			event.Arrival = &domain.TransitPoint{Location: arrLoc, Code: arrCode}
		}
	case "accommodation":
		event.CheckIn = c.FormValue("check_in") == "on"
		event.CheckOut = c.FormValue("check_out") == "on"
	}

	day.Events = append(day.Events, event)
	return h.saveAndRedirect(c, trip)
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

// UpdateEvent updates an existing event in place.
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

	eventURL := c.FormValue("url")
	if len(eventURL) > 2000 {
		return c.String(http.StatusBadRequest, "URL too long (max 2000 characters)")
	}

	existing := &trip.Data.Legs[legIdx].Days[dayIdx].Events[evtIdx]
	existing.Type = eventType
	existing.Title = title
	existing.StartTime = startTime
	existing.EndTime = endTime
	existing.Duration = parseDuration(c.FormValue("duration"))
	existing.Notes = c.FormValue("notes")
	existing.Optional = c.FormValue("optional") == "on"
	existing.URL = eventURL
	existing.Status = c.FormValue("status")

	// Reset type-specific fields before applying
	existing.Location = ""
	existing.Latitude = 0
	existing.Longitude = 0
	existing.TicketRequired = false
	existing.BookingReference = ""
	existing.TransportMode = ""
	existing.Departure = nil
	existing.Arrival = nil
	existing.Carrier = ""
	existing.FlightNumber = ""
	existing.CheckIn = false
	existing.CheckOut = false

	switch eventType {
	case "activity":
		existing.Location = c.FormValue("location")
		existing.TicketRequired = c.FormValue("ticket_required") == "on"
		existing.BookingReference = c.FormValue("booking_reference")
		if lat := c.FormValue("latitude"); lat != "" {
			if v, err := strconv.ParseFloat(lat, 64); err == nil {
				existing.Latitude = v
			}
		}
		if lng := c.FormValue("longitude"); lng != "" {
			if v, err := strconv.ParseFloat(lng, 64); err == nil {
				existing.Longitude = v
			}
		}
	case "transit":
		existing.TransportMode = c.FormValue("transport_mode")
		existing.Carrier = c.FormValue("carrier")
		existing.FlightNumber = c.FormValue("flight_number")
		depLoc := c.FormValue("departure_location")
		depCode := c.FormValue("departure_code")
		if depLoc != "" {
			existing.Departure = &domain.TransitPoint{Location: depLoc, Code: depCode}
		}
		arrLoc := c.FormValue("arrival_location")
		arrCode := c.FormValue("arrival_code")
		if arrLoc != "" {
			existing.Arrival = &domain.TransitPoint{Location: arrLoc, Code: arrCode}
		}
	case "accommodation":
		existing.CheckIn = c.FormValue("check_in") == "on"
		existing.CheckOut = c.FormValue("check_out") == "on"
	}

	return h.saveAndRedirect(c, trip)
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
	return h.saveAndRedirect(c, trip)
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
