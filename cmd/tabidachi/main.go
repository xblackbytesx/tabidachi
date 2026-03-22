package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/csrf"
	"github.com/xblackbytesx/tabidachi/internal/auth"
	"github.com/xblackbytesx/tabidachi/internal/config"
	appdb "github.com/xblackbytesx/tabidachi/internal/db"
	"github.com/xblackbytesx/tabidachi/internal/handler"
	"github.com/xblackbytesx/tabidachi/internal/images"
	appmiddleware "github.com/xblackbytesx/tabidachi/internal/middleware"
	"github.com/xblackbytesx/tabidachi/internal/repository"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	if err := appdb.RunMigrations(cfg.DatabaseURL); err != nil {
		slog.Error("run migrations", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := appdb.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	auth.InitStore(cfg.SessionSecret, cfg.SecureCookies)

	userStore := repository.NewUserStore(pool)
	tripStore := repository.NewTripStore(pool)
	tokenStore := repository.NewTokenStore(pool)
	imageSvc := images.NewService(cfg.PexelsAPIKey, cfg.UnsplashKey, cfg.UploadsDir)

	authHandler := handler.NewAuthHandler(userStore, cfg.AllowRegistration)
	dashHandler := handler.NewDashboardHandler(tripStore)
	tripHandler := handler.NewTripHandler(tripStore, imageSvc)
	importHandler := handler.NewImportHandler(tripStore, imageSvc)
	promptHandler := handler.NewPromptHandler()
	builderHandler := handler.NewBuilderHandler(tripStore)
	imageHandler := handler.NewImageHandler(tripStore, imageSvc)
	settingsHandler := handler.NewSettingsHandler(tokenStore, userStore)
	apiHandler := handler.NewAPIHandler(tripStore, cfg.AppBaseURL)

	e := echo.New()
	e.HideBanner = true
	e.Use(echomiddleware.Recover())
	e.Use(appmiddleware.Logger())

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			h := c.Response().Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			if cfg.SecureCookies {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			return next(c)
		}
	})

	appURL, err := url.Parse(cfg.AppBaseURL)
	if err != nil {
		slog.Error("parse APP_BASE_URL", "err", err)
		os.Exit(1)
	}

	csrfMiddleware := csrf.Protect(
		[]byte(cfg.CSRFAuthKey),
		csrf.Secure(cfg.SecureCookies),
		csrf.RequestHeader("X-CSRF-Token"),
		csrf.CookieName("tabidachi-csrf"),
		csrf.TrustedOrigins([]string{appURL.Host}),
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slog.Error("CSRF validation failed",
				"reason", csrf.FailureReason(r),
				"path", r.URL.Path,
				"method", r.Method,
			)
			// Redirect to login (stale session or CSRF failure); HTMX uses HX-Redirect.
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusForbidden)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		})),
	)
	e.Use(echo.WrapMiddleware(csrfMiddleware))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := csrf.Token(c.Request())
			c.Set("csrf", token)
			return next(c)
		}
	})

	e.Static("/static", "web/static")

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Auth routes (public)
	e.GET("/login", authHandler.LoginGet)
	e.POST("/login", authHandler.LoginPost)
	e.GET("/register", authHandler.RegisterGet)
	e.POST("/register", authHandler.RegisterPost)
	e.POST("/logout", authHandler.Logout)

	// Authenticated routes
	protected := e.Group("", appmiddleware.RequireAuth)

	protected.GET("/", dashHandler.Get)

	protected.GET("/trips/new", tripHandler.NewOptions)
	protected.GET("/trips/new/scratch", tripHandler.NewScratch)
	protected.POST("/trips", tripHandler.Create)
	protected.GET("/trips/new/import", importHandler.Get)
	protected.POST("/trips/import", importHandler.Post)
	protected.GET("/trips/new/convert", promptHandler.ConvertStep1Get)
	protected.POST("/trips/new/convert", promptHandler.ConvertStepPost)
	protected.GET("/trips/new/plan", promptHandler.PlanStep1Get)
	protected.POST("/trips/new/plan", promptHandler.PlanStepPost)
	protected.GET("/trips/new/plan/field", promptHandler.PlanFieldGet)
	// Legacy alias — keep /trips/new/prompt working so existing bookmarks don't 404
	protected.GET("/trips/new/prompt", promptHandler.Step1Get)
	protected.POST("/trips/new/prompt", promptHandler.StepPost)

	protected.GET("/trips/:id", tripHandler.View)
	protected.GET("/trips/:id/export", tripHandler.Export)
	protected.GET("/trips/:id/edit", tripHandler.Edit)
	protected.POST("/trips/:id", tripHandler.Update)
	protected.DELETE("/trips/:id", tripHandler.Delete)
	protected.POST("/trips/:id/delete", tripHandler.Delete)

	// Image management routes
	protected.GET("/trips/:id/image/search", imageHandler.ImageSearch)
	protected.POST("/trips/:id/image", imageHandler.SetTripImage)
	protected.DELETE("/trips/:id/image", imageHandler.ClearTripImage)
	protected.POST("/trips/:id/image/clear", imageHandler.ClearTripImage)
	protected.GET("/trips/:id/legs/:legIdx/image/search", imageHandler.LegImageSearch)
	protected.POST("/trips/:id/legs/:legIdx/image", imageHandler.SetLegImage)
	protected.DELETE("/trips/:id/legs/:legIdx/image", imageHandler.ClearLegImage)
	protected.POST("/trips/:id/legs/:legIdx/image/clear", imageHandler.ClearLegImage)
	protected.GET("/trips/:id/legs/:legIdx/days/:dayIdx/events/:eventIdx/image/search", imageHandler.EventImageSearch)
	protected.POST("/trips/:id/legs/:legIdx/days/:dayIdx/events/:eventIdx/image", imageHandler.SetEventImage)
	protected.DELETE("/trips/:id/legs/:legIdx/days/:dayIdx/events/:eventIdx/image", imageHandler.ClearEventImage)
	protected.POST("/trips/:id/legs/:legIdx/days/:dayIdx/events/:eventIdx/image/clear", imageHandler.ClearEventImage)

	// Authenticated uploads
	protected.GET("/uploads/*", func(c echo.Context) error {
		return echo.WrapHandler(http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadsDir))))(c)
	})

	// Settings & token management
	protected.GET("/settings", settingsHandler.Get)
	protected.POST("/settings/tokens", settingsHandler.GenerateToken)
	protected.POST("/settings/tokens/:id/revoke", settingsHandler.RevokeToken)
	protected.POST("/settings/date-format", settingsHandler.UpdateDateFormat)
	protected.POST("/settings/account/delete", settingsHandler.DeleteAccount)

	// JSON API — Bearer token auth; GET-only so gorilla/csrf does not apply.
	api := e.Group("/api/v1", appmiddleware.RequireAPIToken(tokenStore))
	api.GET("/trips", apiHandler.ListTrips)
	api.GET("/trips/:id", apiHandler.GetTrip)

	// API-authenticated uploads (for mobile clients using Bearer tokens).
	api.GET("/uploads/*", func(c echo.Context) error {
		return echo.WrapHandler(http.StripPrefix("/api/v1/uploads/", http.FileServer(http.Dir(cfg.UploadsDir))))(c)
	})

	// Manual builder mutation endpoints
	protected.POST("/trips/:id/legs", builderHandler.AddLeg)
	protected.POST("/trips/:id/legs/:legIdx/delete", builderHandler.DeleteLeg)
	protected.POST("/trips/:id/legs/:legIdx/accommodation", builderHandler.UpdateAccommodation)
	protected.POST("/trips/:id/legs/:legIdx/days", builderHandler.AddDay)
	protected.POST("/trips/:id/legs/:legIdx/days/:dayIdx", builderHandler.UpdateDay)
	protected.POST("/trips/:id/legs/:legIdx/days/:dayIdx/delete", builderHandler.DeleteDay)
	protected.POST("/trips/:id/legs/:legIdx/days/:dayIdx/events", builderHandler.AddEvent)
	protected.POST("/trips/:id/legs/:legIdx/days/:dayIdx/events/reorder", builderHandler.ReorderEvents)
	protected.POST("/trips/:id/legs/:legIdx/days/:dayIdx/events/:eventIdx/delete", builderHandler.DeleteEvent)

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
		<-sig
		slog.Info("shutting down...")
		cancel()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		_ = e.Shutdown(shutCtx)
	}()

	addr := ":" + cfg.Port
	slog.Info("starting server", "addr", addr, "secure_cookies", cfg.SecureCookies)
	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "err", err)
	}
}
