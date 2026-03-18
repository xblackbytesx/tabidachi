package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/xblackbytesx/tabidachi/internal/repository"
	"github.com/labstack/echo/v4"
)

// RequireAPIToken validates a Bearer token from the Authorization header.
// On success it sets "userID" in the Echo context (same key as RequireAuth).
func RequireAPIToken(tokenStore *repository.TokenStore) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			raw, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || raw == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing or invalid Authorization header"})
			}

			tok, err := tokenStore.GetByRawToken(c.Request().Context(), raw)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			}

			// Non-blocking last-used update; fresh context avoids cancellation on response send.
			go tokenStore.UpdateLastUsed(context.Background(), tok.ID)

			c.Set("userID", tok.UserID.String())
			return next(c)
		}
	}
}
