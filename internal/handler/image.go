package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/xblackbytesx/tabidachi/internal/images"
	"github.com/xblackbytesx/tabidachi/internal/repository"
	"github.com/xblackbytesx/tabidachi/web/templates/pages"
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

// EventImageSearch handles GET /trips/:id/legs/:legIdx/days/:dayIdx/events/:eventIdx/image/search?q=
func (h *ImageHandler) EventImageSearch(c echo.Context) error {
	legIdx := c.Param("legIdx")
	dayIdx := c.Param("dayIdx")
	eventIdx := c.Param("eventIdx")
	query := c.QueryParam("q")
	if query == "" {
		return c.HTML(http.StatusOK, `<div id="event-image-results-`+legIdx+`-`+dayIdx+`-`+eventIdx+`"></div>`)
	}
	results, err := h.imageSvc.Search(c.Request().Context(), query)
	if err != nil {
		slog.Warn("event image search", "query", query, "err", err)
		return c.HTML(http.StatusOK, `<div id="event-image-results-`+legIdx+`-`+dayIdx+`-`+eventIdx+`"><p class="search-error">Search failed — try a different query.</p></div>`)
	}
	return render(c, http.StatusOK, pages.EventImageSearchResults(csrfToken(c), c.Param("id"), legIdx, dayIdx, eventIdx, results))
}

// SetEventImage handles POST /trips/:id/legs/:legIdx/days/:dayIdx/events/:eventIdx/image
func (h *ImageHandler) SetEventImage(c echo.Context) error {
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
	dayIdx, err := strconv.Atoi(c.Param("dayIdx"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid day index")
	}
	eventIdx, err := strconv.Atoi(c.Param("eventIdx"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid event index")
	}
	legIdxStr := strconv.Itoa(legIdx)
	dayIdxStr := strconv.Itoa(dayIdx)
	eventIdxStr := strconv.Itoa(eventIdx)

	remoteURL := c.FormValue("imageURL")
	thumbRemoteURL := c.FormValue("thumbURL")
	credit := c.FormValue("credit")

	if remoteURL == "" {
		return render(c, http.StatusOK, pages.EventImageSection(csrfToken(c), tripID.String(), legIdxStr, dayIdxStr, eventIdxStr, "", "", "No image URL provided."))
	}
	localPath, err := images.Download(c.Request().Context(), remoteURL, h.imageSvc.UploadsDir())
	if err != nil {
		slog.Error("set event image download", "url", remoteURL, "err", err)
		return render(c, http.StatusOK, pages.EventImageSection(csrfToken(c), tripID.String(), legIdxStr, dayIdxStr, eventIdxStr, "", "", "Failed to download image — please try another."))
	}
	localFull := "/" + localPath
	localThumb := localFull
	if thumbRemoteURL != "" && thumbRemoteURL != remoteURL {
		if thumbPath, err := images.Download(c.Request().Context(), thumbRemoteURL, h.imageSvc.UploadsDir()); err == nil {
			localThumb = "/" + thumbPath
		}
	}
	if err := h.trips.UpdateEventImage(c.Request().Context(), tripID, userID, legIdx, dayIdx, eventIdx, localFull, localThumb, credit); err != nil {
		slog.Error("set event image update", "err", err)
		return render(c, http.StatusOK, pages.EventImageSection(csrfToken(c), tripID.String(), legIdxStr, dayIdxStr, eventIdxStr, "", "", "Failed to save image — please try again."))
	}
	return render(c, http.StatusOK, pages.EventImageSection(csrfToken(c), tripID.String(), legIdxStr, dayIdxStr, eventIdxStr, localFull, credit, ""))
}

// ClearEventImage handles DELETE /trips/:id/legs/:legIdx/days/:dayIdx/events/:eventIdx/image
func (h *ImageHandler) ClearEventImage(c echo.Context) error {
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
	dayIdx, err := strconv.Atoi(c.Param("dayIdx"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid day index")
	}
	eventIdx, err := strconv.Atoi(c.Param("eventIdx"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid event index")
	}
	legIdxStr := strconv.Itoa(legIdx)
	dayIdxStr := strconv.Itoa(dayIdx)
	eventIdxStr := strconv.Itoa(eventIdx)
	if err := h.trips.UpdateEventImage(c.Request().Context(), tripID, userID, legIdx, dayIdx, eventIdx, "", "", ""); err != nil {
		slog.Error("clear event image", "err", err)
	}
	return render(c, http.StatusOK, pages.EventImageSection(csrfToken(c), tripID.String(), legIdxStr, dayIdxStr, eventIdxStr, "", "", ""))
}
