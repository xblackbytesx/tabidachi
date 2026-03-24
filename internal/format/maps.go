package format

import "net/url"

// MapsURL builds a Google Maps search URL using a fallback chain:
//  1. Coordinates (most precise)
//  2. Address (street-level search)
//
// The location name is display-only and not used as a search query.
// Returns "" if neither coordinates nor address are available.
func MapsURL(lat, lng, address, location string) string {
	const base = "https://www.google.com/maps/search/?api=1&query="
	if lat != "" && lng != "" {
		return base + url.QueryEscape(lat+","+lng)
	}
	if address != "" {
		return base + url.QueryEscape(address)
	}
	return ""
}
