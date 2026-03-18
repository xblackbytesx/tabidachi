package middleware

import (
	"net/http"

	"github.com/xblackbytesx/tabidachi/internal/auth"
	"github.com/labstack/echo/v4"
)

// RequireAuth redirects unauthenticated requests to /login.
func RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		uid := auth.GetUserID(c.Request())
		if uid == "" {
			return c.Redirect(http.StatusSeeOther, "/login")
		}
		c.Set("userID", uid)
		c.Set("dateFormat", auth.GetDateFormat(c.Request()))
		return next(c)
	}
}
