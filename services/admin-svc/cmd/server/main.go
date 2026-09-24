package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/vianra/codereview/admin-svc/internal/db"
	"github.com/vianra/codereview/admin-svc/internal/handler"
)

func main() {
	port := getEnv("PORT", "8081")
	databaseURL := getEnv("SUPABASE_DB_URL", "")
	logLevel := getEnv("LOG_LEVEL", "info")

	logger, err := setupLogger(logLevel)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	logger.Info("Starting admin-svc",
		zap.String("version", "0.1.0"),
		zap.String("port", port),
	)

	if databaseURL == "" {
		logger.Fatal("SUPABASE_DB_URL is required but not set")
	}

	database, err := db.NewDB(databaseURL)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer database.Close()

	adminHandler := handler.NewAdminHandler(database, logger)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "admin-svc"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		if err := database.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "database unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "service": "admin-svc"})
	})

	api := router.Group("/api/v1")
	{
		api.POST("/orgs", adminHandler.CreateOrganization)
		api.GET("/orgs/:id", adminHandler.GetOrganization)
		api.POST("/orgs/:id/repos", adminHandler.AddRepository)
		api.GET("/orgs/:id/repos", adminHandler.GetRepositories)
		api.POST("/orgs/:id/subscription", adminHandler.UpdateSubscription)
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		logger.Info("Starting HTTP server", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}
	logger.Info("Server exited")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func setupLogger(level string) (*zap.Logger, error) {
	var zapLevel zap.AtomicLevel
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	config := zap.NewProductionConfig()
	config.Level = zapLevel
	return config.Build()
}
