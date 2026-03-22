package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/xblackbytesx/tabidachi/web/templates/pages"
	"github.com/labstack/echo/v4"
)

const tabidachiSchema = `{
  "schemaVersion": "1.1",
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
      "timezone": "IANA tz override for this leg (optional, defaults to trip timezone)",
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
              "url": "External link — booking page, info URL (optional)",
              "status": "confirmed|tentative|cancelled (optional, default confirmed)",

              "location": "For activity type",
              "latitude": 0.0,
              "longitude": 0.0,
              "ticketRequired": false,
              "bookingReference": "optional",

              "transportMode": "flight|train|shinkansen|subway|bus|car|ferry|walk|taxi|tram",
              "departure": { "location": "string", "code": "IATA/station code (optional)", "latitude": 0.0, "longitude": 0.0 },
              "arrival": { "location": "string", "code": "optional", "latitude": 0.0, "longitude": 0.0 },
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

type PromptHandler struct{}

func NewPromptHandler() *PromptHandler {
	return &PromptHandler{}
}

// ConvertStep1Get serves the convert-existing-itinerary wizard (step 1).
func (h *PromptHandler) ConvertStep1Get(c echo.Context) error {
	return render(c, http.StatusOK, pages.PromptBuilder(csrfToken(c), 1, "", nil))
}

// ConvertStepPost handles form submissions for the convert wizard.
func (h *PromptHandler) ConvertStepPost(c echo.Context) error {
	step := c.FormValue("step")

	switch step {
	case "1":
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
		itinerary := c.FormValue("itinerary_text")
		fromDate := c.FormValue("from_date")
		toDate := c.FormValue("to_date")

		included := []string{}
		checkboxes := []string{
			"flights", "trains", "local_transit", "accommodations",
			"activities", "booking_refs", "timings",
		}
		labels := map[string]string{
			"flights":        "flights",
			"trains":         "trains and bullet trains",
			"local_transit":  "local transit (subway, bus, tram)",
			"accommodations": "accommodations (check-in/out, booking refs)",
			"activities":     "scheduled activities (time, location, ticket info)",
			"booking_refs":   "booking references and confirmation numbers",
			"timings":        "departure/arrival times and durations",
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

// Step1Get and StepPost are legacy aliases kept for backward compatibility (/trips/new/prompt).
func (h *PromptHandler) Step1Get(c echo.Context) error {
	return h.ConvertStep1Get(c)
}

func (h *PromptHandler) StepPost(c echo.Context) error {
	return h.ConvertStepPost(c)
}

// PlanStep1Get serves the plan-new-trip wizard (step 1).
func (h *PromptHandler) PlanStep1Get(c echo.Context) error {
	return render(c, http.StatusOK, pages.PlanBuilder(csrfToken(c), 1, "", nil))
}

// PlanStepPost handles form submissions for the plan wizard.
func (h *PromptHandler) PlanStepPost(c echo.Context) error {
	step := c.FormValue("step")

	switch step {
	case "1":
		data := map[string]string{
			"destinations":     c.FormValue("destinations"),
			"start_date":       c.FormValue("start_date"),
			"end_date":         c.FormValue("end_date"),
			"home_city":        c.FormValue("home_city"),
			"travellers_count": c.FormValue("travellers_count"),
			"travellers_ages":  c.FormValue("travellers_ages"),
			"intensity":        c.FormValue("intensity"),
		}
		return render(c, http.StatusOK, pages.PlanBuilder(csrfToken(c), 2, "", data))

	case "2":
		data := map[string]string{
			"destinations":     c.FormValue("destinations"),
			"start_date":       c.FormValue("start_date"),
			"end_date":         c.FormValue("end_date"),
			"home_city":        c.FormValue("home_city"),
			"travellers_count": c.FormValue("travellers_count"),
			"travellers_ages":  c.FormValue("travellers_ages"),
			"intensity":        c.FormValue("intensity"),
			"must_sees":        c.FormValue("must_sees"),
			"nice_to_haves":    c.FormValue("nice_to_haves"),
			"things_to_avoid":  c.FormValue("things_to_avoid"),
			"max_travel_time":  c.FormValue("max_travel_time"),
			"poi_interests":    c.FormValue("poi_interests"),
		}
		prompt := buildPlanningPrompt(data)
		return render(c, http.StatusOK, pages.PlanBuilder(csrfToken(c), 3, prompt, data))
	}

	return c.String(http.StatusBadRequest, "invalid step")
}

// PlanFieldGet returns an optional field partial for HTMX toggle checkboxes.
func (h *PromptHandler) PlanFieldGet(c echo.Context) error {
	field := c.QueryParam("field")
	show := c.QueryParam("show_"+field) == "on"
	return render(c, http.StatusOK, pages.PlanOptionalField(field, show))
}

func buildPrompt(itinerary, fromDate, toDate string, included []string) string {
	dateHint := ""
	if fromDate != "" && toDate != "" {
		dateHint = "\nTrip dates: " + fromDate + " to " + toDate
	}

	var includedList string
	if len(included) > 0 {
		var b strings.Builder
		b.WriteString("\nFocus specifically on extracting: ")
		for i, item := range included {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(item)
		}
		b.WriteString(".")
		includedList = b.String()
	}

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

Output schema (schemaVersion 1.1):
` + tabidachiSchema + `

Itinerary to convert:
` + itinerary
}

func buildPlanningPrompt(data map[string]string) string {
	intensityLabels := map[string]string{
		"maximize": "Maximize sightseeing — pack in as much as possible",
		"balanced": "Balanced — good coverage while conserving energy",
		"relaxed":  "Relaxed — prioritize rest, see what fits without pressure",
	}
	intensityLabel := intensityLabels[data["intensity"]]
	if intensityLabel == "" {
		intensityLabel = data["intensity"]
	}

	var params strings.Builder
	fmt.Fprintf(&params, "Trip parameters:\n")
	fmt.Fprintf(&params, "- Destinations: %s\n", data["destinations"])
	fmt.Fprintf(&params, "- Dates: %s to %s\n", data["start_date"], data["end_date"])
	if data["home_city"] != "" {
		fmt.Fprintf(&params, "- Home city: %s\n", data["home_city"])
	}
	fmt.Fprintf(&params, "- Number of travellers: %s\n", data["travellers_count"])
	if data["travellers_ages"] != "" {
		fmt.Fprintf(&params, "- Ages of travellers: %s\n", data["travellers_ages"])
	}
	fmt.Fprintf(&params, "- Trip intensity: %s\n", intensityLabel)
	fmt.Fprintf(&params, "- Must-sees: %s\n", data["must_sees"])
	if data["nice_to_haves"] != "" {
		fmt.Fprintf(&params, "- Nice-to-haves: %s\n", data["nice_to_haves"])
	}
	if data["things_to_avoid"] != "" {
		fmt.Fprintf(&params, "- Things to avoid: %s\n", data["things_to_avoid"])
	}
	if data["max_travel_time"] != "" {
		fmt.Fprintf(&params, "- Max comfortable travel time between stops: %s\n", data["max_travel_time"])
	}
	if data["poi_interests"] != "" {
		fmt.Fprintf(&params, "- Points of interest / specific interests: %s\n", data["poi_interests"])
	}

	return `IMPORTANT RULES:
1. Before planning anything, review the trip parameters and identify any missing information that would significantly affect the itinerary (e.g. preferred accommodation style, dietary requirements, mobility considerations, budget range, visa constraints). Ask these as numbered follow-up questions FIRST, before any planning.
2. Once follow-up questions are answered (or if none are needed), show a day-by-day summary table: one row per day with date, destination, key activities, and transit segments. Ask "Does this look right? Any changes before I generate the JSON?"
3. Only output the JSON after the user explicitly confirms the summary — but always offer: "Say 'generate JSON' when you're ready."
4. Output a single JSON object matching the Tabidachi schema v1.0 exactly.

---

You are a travel planning assistant. Design a detailed day-by-day itinerary for the following trip, then output it as a single JSON object matching the schema below.

` + params.String() + `
Output schema (schemaVersion 1.1):
` + tabidachiSchema + `

Important output rules:
- Always populate startTime and endTime where you can make a reasonable estimate.
- Always include transportMode, departure.location, arrival.location, and duration for transit events.
- For the optional "notes" field on legs and days: include ONLY actionable logistics (e.g. "Book tickets in advance", "JR Pass valid for this segment"). No cultural tips or food suggestions.
- Suggest realistic travel times between locations based on the destinations and transport modes.`
}
