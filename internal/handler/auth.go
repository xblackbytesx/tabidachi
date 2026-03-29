package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"
	"github.com/xblackbytesx/tabidachi/internal/auth"
	"github.com/xblackbytesx/tabidachi/internal/repository"
	"github.com/xblackbytesx/tabidachi/web/templates/pages"
	"golang.org/x/time/rate"
)

const maxRateBuckets = 10000

type rateBucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// loginLimiter enforces a per-IP rate limit on POST /login to slow brute-force attempts.
// Each IP gets a burst of 5, refilling at 1 token/minute (5 attempts per minute max).
var (
	loginMu      sync.Mutex
	loginBuckets = map[string]*rateBucket{}
)

// registerLimiter enforces a per-IP rate limit on POST /register.
// Stricter than login: 3 attempts per 10 minutes.
var (
	registerMu      sync.Mutex
	registerBuckets = map[string]*rateBucket{}
)

func init() {
	// Periodically evict stale entries to avoid unbounded map growth.
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			now := time.Now()
			loginMu.Lock()
			for ip, b := range loginBuckets {
				if now.Sub(b.lastSeen) > 15*time.Minute {
					delete(loginBuckets, ip)
				}
			}
			loginMu.Unlock()
			registerMu.Lock()
			for ip, b := range registerBuckets {
				if now.Sub(b.lastSeen) > 15*time.Minute {
					delete(registerBuckets, ip)
				}
			}
			registerMu.Unlock()
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
	// Cap map size to prevent unbounded memory growth under DDoS.
	if len(loginBuckets) >= maxRateBuckets {
		return rate.NewLimiter(rate.Every(time.Minute), 5)
	}
	b := &rateBucket{
		limiter:  rate.NewLimiter(rate.Every(time.Minute), 5),
		lastSeen: time.Now(),
	}
	loginBuckets[ip] = b
	return b.limiter
}

func registerLimiter(ip string) *rate.Limiter {
	registerMu.Lock()
	defer registerMu.Unlock()
	if b, ok := registerBuckets[ip]; ok {
		b.lastSeen = time.Now()
		return b.limiter
	}
	if len(registerBuckets) >= maxRateBuckets {
		return rate.NewLimiter(rate.Every(10*time.Minute), 3)
	}
	b := &rateBucket{
		limiter:  rate.NewLimiter(rate.Every(10*time.Minute), 3),
		lastSeen: time.Now(),
	}
	registerBuckets[ip] = b
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

	if !registerLimiter(c.RealIP()).Allow() {
		return render(c, http.StatusTooManyRequests, pages.Register(csrfToken(c), "Too many registration attempts. Please wait before trying again."))
	}

	email := strings.TrimSpace(strings.ToLower(c.FormValue("email")))
	displayName := strings.TrimSpace(c.FormValue("display_name"))
	password := c.FormValue("password")

	if email == "" || displayName == "" || password == "" {
		return render(c, http.StatusOK, pages.Register(csrfToken(c), "All fields are required."))
	}
	if !isValidEmail(email) {
		return render(c, http.StatusOK, pages.Register(csrfToken(c), "Please enter a valid email address."))
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return render(c, http.StatusOK, pages.Register(csrfToken(c), "Registration failed. Please try again."))
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
