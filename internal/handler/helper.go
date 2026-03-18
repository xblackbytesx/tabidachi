package handler

import (
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/hakken/hakken/internal/format"
	"github.com/labstack/echo/v4"
)

// parseUserID safely extracts and parses the authenticated user's UUID from
// the echo context. Returns an error if the value is absent or malformed.
func parseUserID(c echo.Context) (uuid.UUID, error) {
	s, ok := c.Get("userID").(string)
	if !ok || s == "" {
		return uuid.Nil, errors.New("not authenticated")
	}
	return uuid.Parse(s)
}

func csrfToken(c echo.Context) string {
	v := c.Get("csrf")
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func datePref(c echo.Context) string {
	if f, ok := c.Get("dateFormat").(string); ok && f != "" {
		return f
	}
	return "dmy"
}

func redirect(c echo.Context, path string) error {
	return c.Redirect(http.StatusSeeOther, path)
}

func isHTMX(c echo.Context) bool {
	return c.Request().Header.Get("HX-Request") == "true"
}

func render(c echo.Context, status int, t templ.Component) error {
	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response().Header().Set("Cache-Control", "no-store")
	c.Response().WriteHeader(status)
	ctx := format.WithPref(c.Request().Context(), datePref(c))
	return t.Render(ctx, c.Response().Writer)
}
