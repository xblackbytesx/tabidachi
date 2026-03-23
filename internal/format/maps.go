package format

import "net/url"

// MapsURL builds a Google Maps search URL using a fallback chain:
//  1. Coordinates (most precise)
//  2. Address (street-level)
//  3. Location name (search)
//
// Returns "" if all inputs are empty.
func MapsURL(lat, lng, address, location string) string {
	const base = "https://www.google.com/maps/search/?api=1&query="
	if lat != "" && lng != "" {
		return base + url.QueryEscape(lat+","+lng)
	}
	if address != "" {
		return base + url.QueryEscape(address)
	}
	if location != "" {
		return base + url.QueryEscape(location)
	}
	return ""
}
