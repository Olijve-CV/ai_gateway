package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"ai_gateway/internal/config"
	"ai_gateway/internal/database"
	"ai_gateway/internal/handlers"
	"ai_gateway/internal/middleware"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Printf("No .env file found or error loading: %v", err)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := database.Init(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create Echo instance
	e := echo.New()
	e.HideBanner = true

	// Setup template renderer
	renderer := handlers.NewTemplateRenderer("templates")
	e.Renderer = renderer

	// Static files
	e.Static("/static", "static")

	// Middleware
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "X-API-Key"},
	}))

	// Initialize handlers
	h := handlers.New(db, cfg)

	// Root endpoint - render index page
	e.GET("/", h.IndexPage)

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	// Add DB middleware for all routes that need it
	e.Use(middleware.DBMiddleware(db))

	// Auth routes (public)
	auth := e.Group("/api/auth")
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
	auth.GET("/me", h.GetCurrentUser, middleware.JWTAuth(cfg))

	// Config routes (JWT protected)
	configGroup := e.Group("/api/config", middleware.JWTAuth(cfg))
	configGroup.GET("/providers", h.GetProviderConfigs)
	configGroup.GET("/providers/:provider", h.GetProviderConfigsByProvider)
	configGroup.POST("/providers", h.CreateProviderConfig)
	configGroup.GET("/providers/id/:id", h.GetProviderConfigByID)
	configGroup.PUT("/providers/:id", h.UpdateProviderConfig)
	configGroup.DELETE("/providers/:id", h.DeleteProviderConfig)
	configGroup.PUT("/providers/:id/default", h.SetDefaultProviderConfig)
	configGroup.PUT("/providers/:id/toggle", h.ToggleProviderConfig)

	// API Key routes (JWT protected)
	keysGroup := e.Group("/api/keys", middleware.JWTAuth(cfg))
	keysGroup.GET("", h.ListAPIKeys)
	keysGroup.POST("", h.CreateAPIKey)
	keysGroup.GET("/:id", h.GetAPIKey)
	keysGroup.PUT("/:id", h.UpdateAPIKey)
	keysGroup.DELETE("/:id", h.DeleteAPIKey)
	keysGroup.GET("/:id/usage", h.GetAPIKeyUsage)

	// AI Gateway routes (API Key or JWT auth)
	v1 := e.Group("/v1", middleware.GatewayAuth(db, cfg))
	v1.POST("/chat/completions", h.OpenAIChatCompletions)
	v1.POST("/responses", h.OpenAICodeResponses)
	v1.POST("/messages", h.AnthropicMessages)
	v1.POST("/models/:model", h.GeminiGenerateContent)

	// Page routes (public)
	e.GET("/login", h.LoginPage)
	e.GET("/register", h.RegisterPage)
	e.GET("/dashboard", h.DashboardPage)
	e.GET("/dashboard/providers", h.ProvidersPage)
	e.GET("/dashboard/keys", h.KeysPage)
	e.GET("/logout", h.LogoutPage)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	go func() {
		log.Printf("Starting server on %s", addr)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("Server shutdown complete")
}
