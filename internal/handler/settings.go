package handler

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/xblackbytesx/tabidachi/internal/auth"
	"github.com/xblackbytesx/tabidachi/internal/repository"
	"github.com/xblackbytesx/tabidachi/web/templates/pages"
	"github.com/labstack/echo/v4"
)

type SettingsHandler struct {
	tokens *repository.TokenStore
	users  *repository.UserStore
}

func NewSettingsHandler(tokens *repository.TokenStore, users *repository.UserStore) *SettingsHandler {
	return &SettingsHandler{tokens: tokens, users: users}
}

// Get handles GET /settings
func (h *SettingsHandler) Get(c echo.Context) error {
	userID, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}

	tokens, err := h.tokens.List(c.Request().Context(), userID)
	if err != nil {
		slog.Error("settings: list tokens", "err", err)
		tokens = nil
	}

	return render(c, http.StatusOK, pages.Settings(csrfToken(c), tokens, "", datePref(c)))
}

// GenerateToken handles POST /settings/tokens
func (h *SettingsHandler) GenerateToken(c echo.Context) error {
	userID, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}

	name := c.FormValue("name")
	if name == "" {
		tokens, _ := h.tokens.List(c.Request().Context(), userID)
		return render(c, http.StatusOK, pages.Settings(csrfToken(c), tokens, "Token name is required.", datePref(c)))
	}

	rawToken, _, err := h.tokens.Generate(c.Request().Context(), userID, name)
	if err != nil {
		slog.Error("settings: generate token", "err", err)
		tokens, _ := h.tokens.List(c.Request().Context(), userID)
		return render(c, http.StatusOK, pages.Settings(csrfToken(c), tokens, "Failed to generate token.", datePref(c)))
	}

	tokens, err := h.tokens.List(c.Request().Context(), userID)
	if err != nil {
		slog.Error("settings: list tokens after generate", "err", err)
	}

	return render(c, http.StatusOK, pages.SettingsWithNewToken(csrfToken(c), tokens, rawToken, datePref(c)))
}

// RevokeToken handles POST /settings/tokens/:id/revoke
func (h *SettingsHandler) RevokeToken(c echo.Context) error {
	userID, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}

	tokenID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid token id")
	}

	if err := h.tokens.Delete(c.Request().Context(), tokenID, userID); err != nil {
		slog.Error("settings: revoke token", "err", err)
	}

	tokens, _ := h.tokens.List(c.Request().Context(), userID)
	return render(c, http.StatusOK, pages.Settings(csrfToken(c), tokens, "", datePref(c)))
}

// UpdateDateFormat handles POST /settings/date-format
func (h *SettingsHandler) UpdateDateFormat(c echo.Context) error {
	userID, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}

	pref := c.FormValue("date_format")
	if pref != "dmy" && pref != "mdy" && pref != "iso" {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid date format")
	}

	if err := h.users.UpdateDateFormat(c.Request().Context(), userID, pref); err != nil {
		slog.Error("settings: update date format", "err", err)
	}

	if err := auth.SetDateFormat(c.Response().Writer, c.Request(), pref); err != nil {
		slog.Error("settings: save date format to session", "err", err)
	}

	// Update echo context so render() picks up the new pref immediately.
	c.Set("dateFormat", pref)

	tokens, _ := h.tokens.List(c.Request().Context(), userID)
	return render(c, http.StatusOK, pages.Settings(csrfToken(c), tokens, "", pref))
}
