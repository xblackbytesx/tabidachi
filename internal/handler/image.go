package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/hakken/hakken/internal/images"
	"github.com/hakken/hakken/internal/repository"
	"github.com/hakken/hakken/web/templates/pages"
	"github.com/labstack/echo/v4"
)

type ImageHandler struct {
	trips    *repository.TripStore
	imageSvc *images.Service
}

func NewImageHandler(trips *repository.TripStore, svc *images.Service) *ImageHandler {
	return &ImageHandler{trips: trips, imageSvc: svc}
}

func parseTripID(c echo.Context) (uuid.UUID, error) {
	return uuid.Parse(c.Param("id"))
}

// ImageSearch handles GET /trips/:id/image/search?q=
func (h *ImageHandler) ImageSearch(c echo.Context) error {
	query := c.QueryParam("q")
	if query == "" {
		return c.HTML(http.StatusOK, `<div id="image-results"></div>`)
	}
	results, err := h.imageSvc.Search(c.Request().Context(), query)
	if err != nil {
		slog.Warn("image search", "query", query, "err", err)
		return c.HTML(http.StatusOK, `<div id="image-results"><p class="search-error">Search failed — try a different query.</p></div>`)
	}
	return render(c, http.StatusOK, pages.ImageSearchResults(csrfToken(c), c.Param("id"), "", results))
}

// LegImageSearch handles GET /trips/:id/legs/:legIdx/image/search?q=
func (h *ImageHandler) LegImageSearch(c echo.Context) error {
	legIdx := c.Param("legIdx")
	query := c.QueryParam("q")
	if query == "" {
		return c.HTML(http.StatusOK, `<div id="leg-image-results-`+legIdx+`"></div>`)
	}
	results, err := h.imageSvc.Search(c.Request().Context(), query)
	if err != nil {
		slog.Warn("leg image search", "query", query, "err", err)
		return c.HTML(http.StatusOK, `<div id="leg-image-results-`+legIdx+`"><p class="search-error">Search failed — try a different query.</p></div>`)
	}
	return render(c, http.StatusOK, pages.ImageSearchResults(csrfToken(c), c.Param("id"), legIdx, results))
}

// SetTripImage handles POST /trips/:id/image
func (h *ImageHandler) SetTripImage(c echo.Context) error {
	userID, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}
	tripID, err := parseTripID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid trip id")
	}
	remoteURL := c.FormValue("imageURL")
	credit := c.FormValue("credit")
	if remoteURL == "" {
		return render(c, http.StatusOK, pages.TripImagePreviewError(csrfToken(c), tripID.String(), "", "", "No image URL provided."))
	}
	localPath, err := images.Download(c.Request().Context(), remoteURL, h.imageSvc.UploadsDir())
	if err != nil {
		slog.Error("set trip image download", "url", remoteURL, "err", err)
		return render(c, http.StatusOK, pages.TripImagePreviewError(csrfToken(c), tripID.String(), "", "", "Failed to download image — please try another."))
	}
	localURL := "/" + localPath
	if err := h.trips.UpdateTripImage(c.Request().Context(), tripID, userID, localURL, credit); err != nil {
		slog.Error("set trip image update", "err", err)
		return render(c, http.StatusOK, pages.TripImagePreviewError(csrfToken(c), tripID.String(), "", "", "Failed to save image — please try again."))
	}
	return render(c, http.StatusOK, pages.TripImagePreview(csrfToken(c), tripID.String(), localURL, credit))
}

// ClearTripImage handles DELETE /trips/:id/image  (also POST /trips/:id/image/clear)
func (h *ImageHandler) ClearTripImage(c echo.Context) error {
	userID, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}
	tripID, err := parseTripID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid trip id")
	}
	if err := h.trips.UpdateTripImage(c.Request().Context(), tripID, userID, "", ""); err != nil {
		slog.Error("clear trip image", "err", err)
	}
	return render(c, http.StatusOK, pages.TripImagePreview(csrfToken(c), tripID.String(), "", ""))
}

// SetLegImage handles POST /trips/:id/legs/:legIdx/image
func (h *ImageHandler) SetLegImage(c echo.Context) error {
	userID, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}
	tripID, err := parseTripID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid trip id")
	}
	legIdx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid leg index")
	}
	legIdxStr := strconv.Itoa(legIdx)
	remoteURL := c.FormValue("imageURL")
	credit := c.FormValue("credit")
	if remoteURL == "" {
		return render(c, http.StatusOK, pages.LegImagePreviewError(csrfToken(c), tripID.String(), legIdxStr, "", "", "No image URL provided."))
	}
	localPath, err := images.Download(c.Request().Context(), remoteURL, h.imageSvc.UploadsDir())
	if err != nil {
		slog.Error("set leg image download", "url", remoteURL, "err", err)
		return render(c, http.StatusOK, pages.LegImagePreviewError(csrfToken(c), tripID.String(), legIdxStr, "", "", "Failed to download image — please try another."))
	}
	localURL := "/" + localPath
	if err := h.trips.UpdateLegImage(c.Request().Context(), tripID, userID, legIdx, localURL, credit); err != nil {
		slog.Error("set leg image update", "err", err)
		return render(c, http.StatusOK, pages.LegImagePreviewError(csrfToken(c), tripID.String(), legIdxStr, "", "", "Failed to save image — please try again."))
	}
	return render(c, http.StatusOK, pages.LegImagePreview(csrfToken(c), tripID.String(), legIdxStr, localURL, credit))
}

// ClearLegImage handles DELETE /trips/:id/legs/:legIdx/image
func (h *ImageHandler) ClearLegImage(c echo.Context) error {
	userID, err := parseUserID(c)
	if err != nil {
		return redirect(c, "/login")
	}
	tripID, err := parseTripID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid trip id")
	}
	legIdx, err := strconv.Atoi(c.Param("legIdx"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid leg index")
	}
	legIdxStr := strconv.Itoa(legIdx)
	if err := h.trips.UpdateLegImage(c.Request().Context(), tripID, userID, legIdx, "", ""); err != nil {
		slog.Error("clear leg image", "err", err)
	}
	return render(c, http.StatusOK, pages.LegImagePreview(csrfToken(c), tripID.String(), legIdxStr, "", ""))
}
