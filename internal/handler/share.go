package handler

import (
	"context"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/xblackbytesx/tabidachi/internal/repository"
	"github.com/xblackbytesx/tabidachi/web/templates/pages"
	"github.com/labstack/echo/v4"
)

type ShareHandler struct {
	shares     *repository.ShareStore
	trips      *repository.TripStore
	uploadsDir string
}

func NewShareHandler(shares *repository.ShareStore, trips *repository.TripStore, uploadsDir string) *ShareHandler {
	return &ShareHandler{shares: shares, trips: trips, uploadsDir: uploadsDir}
}

// View handles GET /share/:token — public, no session required.
func (h *ShareHandler) View(c echo.Context) error {
	rawToken := c.Param("token")
	share, err := h.shares.GetByRawToken(c.Request().Context(), rawToken)
	if err != nil {
		return c.String(http.StatusNotFound, "not found")
	}

	trip, err := h.trips.GetByIDAnon(c.Request().Context(), share.TripID)
	if err != nil {
		return c.String(http.StatusNotFound, "not found")
	}

	go h.shares.UpdateLastUsed(context.Background(), share.ID)

	return render(c, http.StatusOK, pages.ShareView(trip, rawToken))
}

// ServeUpload handles GET /share/:token/uploads/* — serves trip images for share viewers.
func (h *ShareHandler) ServeUpload(c echo.Context) error {
	rawToken := c.Param("token")
	share, err := h.shares.GetByRawToken(c.Request().Context(), rawToken)
	if err != nil {
		return c.String(http.StatusNotFound, "not found")
	}

	// The wildcard captures everything after /share/:token/uploads/
	// Clean the path first to collapse any ".." segments before the prefix check,
	// preventing cross-trip path traversal (e.g. <tripID>/../<otherID>/file.jpg).
	filePath := strings.TrimPrefix(path.Clean("/"+c.Param("*")), "/")

	// Path traversal protection: the requested file must be under the trip's own directory.
	tripPrefix := share.TripID.String() + "/"
	if !strings.HasPrefix(filePath, tripPrefix) {
		return c.String(http.StatusForbidden, "forbidden")
	}

	return echo.WrapHandler(
		http.StripPrefix("/share/"+rawToken+"/uploads/",
			http.FileServer(http.Dir(h.uploadsDir)),
		),
	)(c)
}

// CreateShare handles POST /trips/:id/shares — authenticated, creates a new share link.
func (h *ShareHandler) CreateShare(c echo.Context) error {
	uid, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}

	tripID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.String(http.StatusNotFound, "not found")
	}

	// Verify trip ownership before creating a share.
	trip, err := h.trips.GetByID(c.Request().Context(), tripID, uid)
	if err != nil {
		return c.String(http.StatusNotFound, "not found")
	}

	name := strings.TrimSpace(c.FormValue("name"))
	if name == "" {
		name = "Share link"
	}
	if len(name) > 200 {
		name = name[:200]
	}

	rawToken, _, err := h.shares.Generate(c.Request().Context(), tripID, uid, name)
	if err != nil {
		slog.Error("create share", "err", err)
		shares, _ := h.shares.ListByTrip(c.Request().Context(), tripID, uid)
		return render(c, http.StatusOK, pages.TripView(csrfToken(c), trip, shares, ""))
	}

	shares, err := h.shares.ListByTrip(c.Request().Context(), tripID, uid)
	if err != nil {
		slog.Error("list shares after create", "err", err)
	}

	return render(c, http.StatusOK, pages.TripView(csrfToken(c), trip, shares, rawToken))
}

// RevokeShare handles POST /trips/:id/shares/:shareId/revoke — authenticated.
func (h *ShareHandler) RevokeShare(c echo.Context) error {
	uid, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}

	tripID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.String(http.StatusNotFound, "not found")
	}

	shareID, err := uuid.Parse(c.Param("shareId"))
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid share id")
	}

	trip, err := h.trips.GetByID(c.Request().Context(), tripID, uid)
	if err != nil {
		return c.String(http.StatusNotFound, "not found")
	}

	if err := h.shares.Delete(c.Request().Context(), shareID, tripID, uid); err != nil {
		slog.Error("revoke share", "err", err)
	}

	shares, err := h.shares.ListByTrip(c.Request().Context(), tripID, uid)
	if err != nil {
		slog.Error("list shares after revoke", "err", err)
	}

	return render(c, http.StatusOK, pages.TripView(csrfToken(c), trip, shares, ""))
}
