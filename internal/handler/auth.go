package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/xblackbytesx/tabidachi/internal/auth"
	"github.com/xblackbytesx/tabidachi/internal/repository"
	"github.com/xblackbytesx/tabidachi/web/templates/pages"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

// loginLimiter enforces a per-IP rate limit on POST /login to slow brute-force attempts.
// Each IP gets a burst of 5, refilling at 1 token/minute (5 attempts per minute max).
var (
	loginMu      sync.Mutex
	loginBuckets = map[string]*loginBucket{}
)

type loginBucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func init() {
	// Periodically evict stale entries to avoid unbounded map growth.
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			loginMu.Lock()
			for ip, b := range loginBuckets {
				if time.Since(b.lastSeen) > 15*time.Minute {
					delete(loginBuckets, ip)
				}
			}
			loginMu.Unlock()
		}
	}()
}

func loginLimiter(ip string) *rate.Limiter {
	loginMu.Lock()
	defer loginMu.Unlock()
	if b, ok := loginBuckets[ip]; ok {
		b.lastSeen = time.Now()
		return b.limiter
	}
	b := &loginBucket{
		limiter:  rate.NewLimiter(rate.Every(time.Minute), 5),
		lastSeen: time.Now(),
	}
	loginBuckets[ip] = b
	return b.limiter
}

type AuthHandler struct {
	users             *repository.UserStore
	allowRegistration bool
}

func NewAuthHandler(users *repository.UserStore, allowRegistration bool) *AuthHandler {
	return &AuthHandler{users: users, allowRegistration: allowRegistration}
}

func (h *AuthHandler) LoginGet(c echo.Context) error {
	flash := auth.GetFlash(c.Response().Writer, c.Request())
	return render(c, http.StatusOK, pages.Login(csrfToken(c), flash))
}

func (h *AuthHandler) LoginPost(c echo.Context) error {
	if !loginLimiter(c.RealIP()).Allow() {
		return render(c, http.StatusTooManyRequests, pages.Login(csrfToken(c), "Too many login attempts. Please wait a minute before trying again."))
	}

	email := strings.TrimSpace(c.FormValue("email"))
	password := c.FormValue("password")

	user, err := h.users.GetByEmail(c.Request().Context(), email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return render(c, http.StatusOK, pages.Login(csrfToken(c), "Invalid email or password."))
		}
		slog.Error("login: get user", "err", err)
		return render(c, http.StatusOK, pages.Login(csrfToken(c), "An error occurred. Please try again."))
	}

	if !auth.CheckPassword(user.PasswordHash, password) {
		return render(c, http.StatusOK, pages.Login(csrfToken(c), "Invalid email or password."))
	}

	if err := auth.SetUserID(c.Response().Writer, c.Request(), user.ID.String()); err != nil {
		slog.Error("login: set session", "err", err)
		return render(c, http.StatusOK, pages.Login(csrfToken(c), "An error occurred. Please try again."))
	}
	if err := auth.SetDateFormat(c.Response().Writer, c.Request(), user.DateFormat); err != nil {
		slog.Error("login: set date format session", "err", err)
	}

	return redirect(c, "/")
}

func (h *AuthHandler) RegisterGet(c echo.Context) error {
	if !h.allowRegistration {
		return render(c, http.StatusForbidden, pages.Login(csrfToken(c), "Registration is not open on this instance."))
	}
	flash := auth.GetFlash(c.Response().Writer, c.Request())
	return render(c, http.StatusOK, pages.Register(csrfToken(c), flash))
}

func (h *AuthHandler) RegisterPost(c echo.Context) error {
	if !h.allowRegistration {
		return render(c, http.StatusForbidden, pages.Login(csrfToken(c), "Registration is not open on this instance."))
	}

	email := strings.TrimSpace(strings.ToLower(c.FormValue("email")))
	displayName := strings.TrimSpace(c.FormValue("display_name"))
	password := c.FormValue("password")

	if email == "" || displayName == "" || password == "" {
		return render(c, http.StatusOK, pages.Register(csrfToken(c), "All fields are required."))
	}
	if len(password) < 8 {
		return render(c, http.StatusOK, pages.Register(csrfToken(c), "Password must be at least 8 characters."))
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		slog.Error("register: hash password", "err", err)
		return render(c, http.StatusOK, pages.Register(csrfToken(c), "An error occurred. Please try again."))
	}

	user, err := h.users.Create(c.Request().Context(), email, displayName, hash)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return render(c, http.StatusOK, pages.Register(csrfToken(c), "An account with that email already exists."))
		}
		slog.Error("register: create user", "err", err)
		return render(c, http.StatusOK, pages.Register(csrfToken(c), "An error occurred. Please try again."))
	}

	if err := auth.SetUserID(c.Response().Writer, c.Request(), user.ID.String()); err != nil {
		slog.Error("register: set session", "err", err)
		return redirect(c, "/login")
	}
	if err := auth.SetDateFormat(c.Response().Writer, c.Request(), user.DateFormat); err != nil {
		slog.Error("register: set date format session", "err", err)
	}

	return redirect(c, "/")
}

func (h *AuthHandler) Logout(c echo.Context) error {
	auth.ClearSession(c.Response().Writer, c.Request())
	return redirect(c, "/login")
}
