package handler

import (
	"log/slog"
	"net/http"

	"github.com/hakken/hakken/internal/repository"
	"github.com/hakken/hakken/web/templates/pages"
	"github.com/labstack/echo/v4"
)

type DashboardHandler struct {
	trips *repository.TripStore
}

func NewDashboardHandler(trips *repository.TripStore) *DashboardHandler {
	return &DashboardHandler{trips: trips}
}

func (h *DashboardHandler) Get(c echo.Context) error {
	uid, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}

	trips, err := h.trips.List(c.Request().Context(), uid)
	if err != nil {
		slog.Error("dashboard: list trips", "err", err)
		trips = nil
	}

	return render(c, http.StatusOK, pages.Dashboard(csrfToken(c), trips))
}
