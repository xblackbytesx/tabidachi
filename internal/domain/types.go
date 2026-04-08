package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// User represents an authenticated account.
type User struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	PasswordHash string
	DateFormat   string
	CreatedAt    time.Time
}

// Trip is the database row. Trip content (legs/days/events) lives in Data.
type Trip struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	Title            string
	StartDate        time.Time
	EndDate          time.Time
	HomeLocation     string
	Timezone         string
	CoverColor       string
	CoverImageURL    string
	CoverImageCredit string
	Data             TripData
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IsActive returns true if today falls within the trip's date range.
func (t *Trip) IsActive() bool {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	start := t.StartDate.Truncate(24 * time.Hour)
	end := t.EndDate.Truncate(24 * time.Hour)
	return !today.Before(start) && !today.After(end)
}

// TripData is the full nested itinerary stored as JSONB.
type TripData struct {
	SchemaVersion string `json:"schemaVersion"`
	Title         string `json:"title"`
	StartDate     string `json:"startDate"`
	EndDate       string `json:"endDate"`
	HomeLocation  string `json:"homeLocation,omitempty"`
	Timezone      string `json:"timezone,omitempty"`
	Legs          []Leg  `json:"legs"`
}

type Leg struct {
	Sequence        int            `json:"sequence"`
	Destination     string         `json:"destination"`
	Region          string         `json:"region,omitempty"`
	StartDate       string         `json:"startDate"`
	EndDate         string         `json:"endDate"`
	Accommodation   *Accommodation `json:"accommodation,omitempty"`
	Notes           string         `json:"notes,omitempty"`
	Days            []Day          `json:"days"`
	CoverImageURL   string         `json:"coverImageURL,omitempty"`
	CoverImageCredit string        `json:"coverImageCredit,omitempty"`
}

type Accommodation struct {
	Name             string `json:"name"`
	Neighborhood     string `json:"neighborhood,omitempty"`
	Address          string `json:"address,omitempty"`
	CheckIn          string `json:"checkIn"`
	CheckOut         string `json:"checkOut"`
	BookingReference string `json:"bookingReference,omitempty"`
}

type Day struct {
	Date   string  `json:"date"`
	Label  string  `json:"label,omitempty"`
	Type   string  `json:"type"` // normal|arrival|departure|travel|rest|flexible
	Notes  string  `json:"notes,omitempty"`
	Events []Event `json:"events"`
}

type Event struct {
	Sequence  int    `json:"sequence"`
	Type      string `json:"type"` // activity|transit|accommodation
	Title     string `json:"title"`
	StartTime string `json:"startTime,omitempty"`
	EndTime   string `json:"endTime,omitempty"`
	Duration  string `json:"duration,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Optional  bool   `json:"optional,omitempty"`

	// activity fields
	Location         string `json:"location,omitempty"`
	Address          string `json:"address,omitempty"`
	Latitude         string `json:"latitude,omitempty"`
	Longitude        string `json:"longitude,omitempty"`
	URL              string `json:"url,omitempty"`
	TicketRequired   bool   `json:"ticketRequired,omitempty"`
	BookingReference string `json:"bookingReference,omitempty"`

	// transit fields
	TransportMode string        `json:"transportMode,omitempty"`
	Departure     *TransitPoint `json:"departure,omitempty"`
	Arrival       *TransitPoint `json:"arrival,omitempty"`
	Carrier       string        `json:"carrier,omitempty"`
	FlightNumber  string        `json:"flightNumber,omitempty"`
	TrackFlight   bool          `json:"trackFlight,omitempty"`

	// accommodation event fields
	CheckIn  bool `json:"checkIn,omitempty"`
	CheckOut bool `json:"checkOut,omitempty"`

	// image fields (activity events only)
	ImageURL      string `json:"imageURL,omitempty"`
	ImageThumbURL string `json:"imageThumbURL,omitempty"`
	ImageCredit   string `json:"imageCredit,omitempty"`
}

type TransitPoint struct {
	Location  string `json:"location"`
	Code      string `json:"code,omitempty"`
	Latitude  string `json:"latitude,omitempty"`
	Longitude string `json:"longitude,omitempty"`
}

// TripShare represents a shareable link scoped to a single trip.
type TripShare struct {
	ID          uuid.UUID
	TripID      uuid.UUID
	UserID      uuid.UUID
	Name        string
	TokenHash   string
	ExpiresAt   *time.Time
	CreatedAt   time.Time
	LastUsedAt  *time.Time
}

// IsExpired returns true if the share link has passed its expiry time.
func (s *TripShare) IsExpired() bool {
	return s.ExpiresAt != nil && time.Now().After(*s.ExpiresAt)
}

// APIToken represents a personal access token for API authentication.
type APIToken struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Name        string
	TokenHash   string
	CreatedAt   time.Time
	LastUsedAt  *time.Time
}

// TransportIcon maps a transport mode string to an icon name.
func TransportIcon(mode string) string {
	icons := map[string]string{
		"flight":     "plane",
		"train":      "train",
		"shinkansen": "train",
		"subway":     "train-subway",
		"bus":        "bus",
		"car":        "car",
		"ferry":      "water",
		"walk":       "person-walking",
		"taxi":       "taxi",
		"tram":       "train-tram",
	}
	if icon, ok := icons[mode]; ok {
		return icon
	}
	return "arrow-right"
}

// FlightTrackerURL returns a Flightradar24 URL for the given flight number.
// It strips spaces so "JL 123" becomes "JL123".
func FlightTrackerURL(flightNumber string) string {
	fn := strings.ReplaceAll(strings.TrimSpace(flightNumber), " ", "")
	if fn == "" {
		return ""
	}
	return "https://www.flightradar24.com/" + fn
}

// DayTypeLabel returns a human-friendly label for a day type.
func DayTypeLabel(dayType string) string {
	labels := map[string]string{
		"arrival":   "Arrival",
		"departure": "Departure",
		"travel":    "Travel",
		"rest":      "Rest",
		"flexible":  "Flexible",
	}
	if label, ok := labels[dayType]; ok {
		return label
	}
	return ""
}
