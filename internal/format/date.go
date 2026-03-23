package format

import (
	"context"
	"strconv"
	"time"
)

type ctxKey struct{}

// WithPref stores the user's date format preference in ctx.
func WithPref(ctx context.Context, pref string) context.Context {
	return context.WithValue(ctx, ctxKey{}, pref)
}

// PrefFromCtx retrieves the date format preference from ctx, defaulting to "dmy".
func PrefFromCtx(ctx context.Context) string {
	if p, ok := ctx.Value(ctxKey{}).(string); ok && p != "" {
		return p
	}
	return "dmy"
}

// Layout returns the Go time layout string for the given preference.
func Layout(pref string) string {
	switch pref {
	case "mdy":
		return "Jan 2, 2006"
	case "iso":
		return "2006-01-02"
	default: // "dmy"
		return "2 Jan 2006"
	}
}

// Date parses an ISO date string (YYYY-MM-DD) and formats it using the ctx preference.
func Date(ctx context.Context, iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.Format(Layout(PrefFromCtx(ctx)))
}

// Time formats a time.Time value using the ctx preference.
func Time(ctx context.Context, t time.Time) string {
	return t.Format(Layout(PrefFromCtx(ctx)))
}

// DateTime parses an ISO 8601 datetime string (e.g. "2026-04-17T15:00:00+09:00")
// and formats it as a human-readable date + time, dropping the timezone suffix.
// Falls back to the raw string if parsing fails.
func DateTime(ctx context.Context, s string) string {
	if s == "" {
		return ""
	}
	t := parseDateTime(s)
	if t.IsZero() {
		return s
	}
	switch PrefFromCtx(ctx) {
	case "mdy":
		return t.Format("Mon Jan 2, 15:04")
	case "iso":
		return t.Format("2006-01-02 15:04")
	default: // "dmy"
		return t.Format("Mon 2 Jan, 15:04")
	}
}

// StayInfo formats check-in/check-out datetimes as a compact stay summary.
// Example: "in Fri 17th · out Sun 19th (2 nights)"
func StayInfo(checkIn, checkOut string) string {
	inTime := parseDateTime(checkIn)
	outTime := parseDateTime(checkOut)
	if inTime.IsZero() || outTime.IsZero() {
		return checkIn + " – " + checkOut
	}
	// Compare calendar dates, not duration — check-in 15:00 to check-out 11:00
	// is less than 24h per night but still counts as the correct number of nights.
	inDate := time.Date(inTime.Year(), inTime.Month(), inTime.Day(), 0, 0, 0, 0, time.UTC)
	outDate := time.Date(outTime.Year(), outTime.Month(), outTime.Day(), 0, 0, 0, 0, time.UTC)
	nights := int(outDate.Sub(inDate).Hours() / 24)
	nightLabel := "night"
	if nights != 1 {
		nightLabel = "nights"
	}
	return "in " + formatDayShort(inTime) + "  ·  out " + formatDayShort(outTime) +
		" (" + strconv.Itoa(nights) + " " + nightLabel + ")"
}

// parseDateTime tries common ISO 8601 datetime formats and returns the parsed time.
func parseDateTime(s string) time.Time {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04Z07:00",
		"2006-01-02T15:04",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// formatDayShort returns "Mon 2nd" style strings.
func formatDayShort(t time.Time) string {
	dow := t.Format("Mon")
	day := t.Day()
	return dow + " " + strconv.Itoa(day) + ordinalSuffix(day)
}

func ordinalSuffix(day int) string {
	if day >= 11 && day <= 13 {
		return "th"
	}
	switch day % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	default:
		return "th"
	}
}

// ShortDate formats an ISO date string without the year — for use in contexts where
// the year is already implicit (e.g. leg date ranges within a trip).
func ShortDate(ctx context.Context, iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	switch PrefFromCtx(ctx) {
	case "mdy":
		return t.Format("Jan 2")
	case "iso":
		return t.Format("01-02")
	default: // "dmy"
		return t.Format("2 Jan")
	}
}

// DayDate formats an ISO date string as "Monday, 2 Jan" / "Monday, Jan 2" / "Monday, 2006-01-02".
func DayDate(ctx context.Context, iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	switch PrefFromCtx(ctx) {
	case "mdy":
		return t.Format("Monday, Jan 2")
	case "iso":
		return t.Format("Monday, 2006-01-02")
	default: // "dmy"
		return t.Format("Monday, 2 Jan")
	}
}
