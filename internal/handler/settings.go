package handler

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/hakken/hakken/internal/repository"
	"github.com/hakken/hakken/web/templates/pages"
	"github.com/labstack/echo/v4"
)

type SettingsHandler struct {
	tokens *repository.TokenStore
}

func NewSettingsHandler(tokens *repository.TokenStore) *SettingsHandler {
	return &SettingsHandler{tokens: tokens}
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

	return render(c, http.StatusOK, pages.Settings(csrfToken(c), tokens, ""))
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
		return render(c, http.StatusOK, pages.Settings(csrfToken(c), tokens, "Token name is required."))
	}

	rawToken, _, err := h.tokens.Generate(c.Request().Context(), userID, name)
	if err != nil {
		slog.Error("settings: generate token", "err", err)
		tokens, _ := h.tokens.List(c.Request().Context(), userID)
		return render(c, http.StatusOK, pages.Settings(csrfToken(c), tokens, "Failed to generate token."))
	}

	tokens, err := h.tokens.List(c.Request().Context(), userID)
	if err != nil {
		slog.Error("settings: list tokens after generate", "err", err)
	}

	return render(c, http.StatusOK, pages.SettingsWithNewToken(csrfToken(c), tokens, rawToken))
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
	return render(c, http.StatusOK, pages.Settings(csrfToken(c), tokens, ""))
}
