package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hakken/hakken/internal/auth"
	"github.com/hakken/hakken/internal/repository"
	"github.com/hakken/hakken/web/templates/pages"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	users *repository.UserStore
}

func NewAuthHandler(users *repository.UserStore) *AuthHandler {
	return &AuthHandler{users: users}
}

func (h *AuthHandler) LoginGet(c echo.Context) error {
	flash := auth.GetFlash(c.Response().Writer, c.Request())
	return render(c, http.StatusOK, pages.Login(csrfToken(c), flash))
}

func (h *AuthHandler) LoginPost(c echo.Context) error {
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
	flash := auth.GetFlash(c.Response().Writer, c.Request())
	return render(c, http.StatusOK, pages.Register(csrfToken(c), flash))
}

func (h *AuthHandler) RegisterPost(c echo.Context) error {
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
