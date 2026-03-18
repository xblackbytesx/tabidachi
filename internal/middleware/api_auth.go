package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/xblackbytesx/tabidachi/internal/repository"
	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

// apiLimiters holds per-IP rate limiters for API token endpoints.
var apiLimiters struct {
	sync.Mutex
	m map[string]*apiLimiterEntry
}

type apiLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func init() {
	apiLimiters.m = make(map[string]*apiLimiterEntry)

	// Evict stale entries every 5 minutes.
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			apiLimiters.Lock()
			cutoff := time.Now().Add(-10 * time.Minute)
			for ip, entry := range apiLimiters.m {
				if entry.lastSeen.Before(cutoff) {
					delete(apiLimiters.m, ip)
				}
			}
			apiLimiters.Unlock()
		}
	}()
}

func getAPILimiter(ip string) *rate.Limiter {
	apiLimiters.Lock()
	defer apiLimiters.Unlock()

	entry, ok := apiLimiters.m[ip]
	if !ok {
		// 10 requests/second, burst of 20.
		entry = &apiLimiterEntry{limiter: rate.NewLimiter(10, 20)}
		apiLimiters.m[ip] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

// RequireAPIToken validates a Bearer token from the Authorization header.
// On success it sets "userID" in the Echo context (same key as RequireAuth).
func RequireAPIToken(tokenStore *repository.TokenStore) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !getAPILimiter(c.RealIP()).Allow() {
				return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			}

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
