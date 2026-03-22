package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
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
		trip.Data.Legs[idx].Accommodation = &domain.Accommodation{
			Name:             name,
			Neighborhood:     c.FormValue("neighborhood"),
			Address:          c.FormValue("address"),
			CheckIn:          c.FormValue("check_in"),
			CheckOut:         c.FormValue("check_out"),
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
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return c.String(http.StatusBadRequest, "invalid date format")
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

	day := &trip.Data.Legs[legIdx].Days[dayIdx]
	event := domain.Event{
		Sequence:  len(day.Events) + 1,
		Type:      eventType,
		Title:     title,
		StartTime: c.FormValue("start_time"),
		EndTime:   c.FormValue("end_time"),
		Duration:  c.FormValue("duration"),
		Notes:     c.FormValue("notes"),
		Optional:  c.FormValue("optional") == "on",
	}

	switch eventType {
	case "activity":
		event.Location = c.FormValue("location")
		event.TicketRequired = c.FormValue("ticket_required") == "on"
		event.BookingReference = c.FormValue("booking_reference")
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
