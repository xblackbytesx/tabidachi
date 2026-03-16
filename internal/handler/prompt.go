package handler

import (
	"net/http"

	"github.com/hakken/hakken/web/templates/pages"
	"github.com/labstack/echo/v4"
)

type PromptHandler struct{}

func NewPromptHandler() *PromptHandler {
	return &PromptHandler{}
}

func (h *PromptHandler) Step1Get(c echo.Context) error {
	return render(c, http.StatusOK, pages.PromptBuilder(csrfToken(c), 1, "", nil))
}

func (h *PromptHandler) StepPost(c echo.Context) error {
	step := c.FormValue("step")

	switch step {
	case "1":
		// Moving from step 1 → step 2
		itinerary := c.FormValue("itinerary_text")
		fromDate := c.FormValue("from_date")
		toDate := c.FormValue("to_date")
		data := map[string]string{
			"itinerary_text": itinerary,
			"from_date":      fromDate,
			"to_date":        toDate,
		}
		return render(c, http.StatusOK, pages.PromptBuilder(csrfToken(c), 2, "", data))

	case "2":
		// Moving from step 2 → step 3: generate the prompt
		itinerary := c.FormValue("itinerary_text")
		fromDate := c.FormValue("from_date")
		toDate := c.FormValue("to_date")

		// Build what's included list
		included := []string{}
		checkboxes := []string{
			"flights", "trains", "local_transit", "accommodations",
			"activities", "booking_refs", "timings",
		}
		labels := map[string]string{
			"flights":       "flights",
			"trains":        "trains and bullet trains",
			"local_transit": "local transit (subway, bus, tram)",
			"accommodations": "accommodations (check-in/out, booking refs)",
			"activities":    "scheduled activities (time, location, ticket info)",
			"booking_refs":  "booking references and confirmation numbers",
			"timings":       "departure/arrival times and durations",
		}
		for _, key := range checkboxes {
			if c.FormValue(key) == "on" {
				included = append(included, labels[key])
			}
		}

		prompt := buildPrompt(itinerary, fromDate, toDate, included)
		data := map[string]string{
			"prompt": prompt,
		}
		return render(c, http.StatusOK, pages.PromptBuilder(csrfToken(c), 3, prompt, data))
	}

	return c.String(http.StatusBadRequest, "invalid step")
}

func buildPrompt(itinerary, fromDate, toDate string, included []string) string {
	dateHint := ""
	if fromDate != "" && toDate != "" {
		dateHint = "\nTrip dates: " + fromDate + " to " + toDate
	}

	includedList := ""
	if len(included) > 0 {
		includedList = "\nFocus specifically on extracting: "
		for i, item := range included {
			if i > 0 {
				includedList += ", "
			}
			includedList += item
		}
		includedList += "."
	}

	const schema = `{
  "schemaVersion": "1.0",
  "title": "string (required)",
  "startDate": "YYYY-MM-DD (required)",
  "endDate": "YYYY-MM-DD (required)",
  "homeLocation": "string (optional)",
  "timezone": "IANA tz string e.g. Asia/Tokyo (optional, default UTC)",
  "legs": [
    {
      "sequence": 1,
      "destination": "City name",
      "region": "Region (optional)",
      "startDate": "YYYY-MM-DD",
      "endDate": "YYYY-MM-DD",
      "accommodation": {
        "name": "Hotel name",
        "neighborhood": "optional",
        "address": "optional",
        "checkIn": "ISO8601 datetime",
        "checkOut": "ISO8601 datetime",
        "bookingReference": "optional"
      },
      "notes": "Actionable logistics only (optional)",
      "days": [
        {
          "date": "YYYY-MM-DD",
          "label": "Day label (optional)",
          "type": "normal|arrival|departure|travel|rest|flexible",
          "notes": "Actionable logistics only (optional)",
          "events": [
            {
              "sequence": 1,
              "type": "activity|transit|accommodation",
              "title": "Event title",
              "startTime": "HH:MM (optional)",
              "endTime": "HH:MM (optional)",
              "duration": "ISO8601 e.g. PT1H30M (optional)",
              "notes": "optional",
              "optional": false,

              "location": "For activity type",
              "ticketRequired": false,
              "bookingReference": "optional",

              "transportMode": "flight|train|shinkansen|subway|bus|car|ferry|walk|taxi|tram",
              "departure": { "location": "string", "code": "IATA/station code (optional)" },
              "arrival": { "location": "string", "code": "optional" },
              "carrier": "optional",
              "flightNumber": "optional",

              "checkIn": true,
              "checkOut": false
            }
          ]
        }
      ]
    }
  ]
}`

	return `IMPORTANT RULES:
1. Do NOT change, correct, or adjust any dates, times, or trip details from the source itinerary. Preserve all dates and times exactly as written, even if they seem unusual.
2. If travel times between legs seem inconsistent or cannot be verified, note this as a flag but do NOT change the data.
3. If anything in the itinerary is unclear or ambiguous — a missing date, an unspecified transport mode, an unclear location — ask the user for clarification BEFORE outputting any JSON. List all questions first.
4. Once any clarifications are resolved (or if none are needed), output a human-readable summary table of the trip structure: one row per day with date, destination, key events, and transit segments. Ask the user to confirm this table is correct before proceeding to JSON.

Only after the user confirms the summary table is accurate should you output the JSON.

---

You are a travel data extraction assistant. Extract ONLY the practical logistics from the itinerary below and output a single JSON object matching the exact schema provided. Ignore food recommendations, cultural tips, packing advice, and general guidance. Focus only on: departures, arrivals, transit segments (include transport mode, carrier, duration), accommodations (check-in/out times, booking references), and scheduled activities (time, location).

Always include startTime and endTime where available. Always include transportMode, departure.location, arrival.location, and duration for transit events. Always include carrier and flightNumber or trainType where mentioned. Populate every optional field that has available data.

For the optional "notes" field on legs and days: include a note ONLY if it contains genuinely actionable logistics — things like baggage storage instructions, pass validity reminders, check-out deadlines, or event booking requirements. Do NOT include food suggestions, cultural etiquette, general tips, or inspirational content in notes. Keep notes short and practical (one or two sentences max).` +
		dateHint + includedList + `

Output schema (schemaVersion 1.0):
` + schema + `

Itinerary to convert:
` + itinerary
}
