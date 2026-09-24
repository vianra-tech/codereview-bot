package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/vianra/codereview/ingress-svc/internal/config"
	"github.com/vianra/codereview/ingress-svc/internal/nats"
	"github.com/vianra/codereview/ingress-svc/internal/webhook"
)

func main() {
	cfg := config.Load("ingress-svc")

	logger, err := config.SetupLogger(cfg.LogLevel)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	logger.Info("Starting ingress-svc",
		zap.String("version", "0.1.0"),
		zap.String("environment", cfg.Environment),
	)

	// Initialize Redis client
	redisOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Fatal("Failed to parse Redis URL", zap.Error(err))
	}
	redisClient := redis.NewClient(redisOpt)
	defer redisClient.Close()

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	logger.Info("Connected to Redis")

	// Initialize NATS JetStream
	nc, err := nats.Connect(cfg.NATSURL, logger)
	if err != nil {
		logger.Fatal("Failed to connect to NATS", zap.Error(err))
	}
	defer nc.Drain()

	js, err := jetstream.New(nc)
	if err != nil {
		logger.Fatal("Failed to create JetStream context", zap.Error(err))
	}

	// Setup NATS streams
	if err := nats.SetupStreams(ctx, js, logger); err != nil {
		logger.Fatal("Failed to setup NATS streams", zap.Error(err))
	}
	logger.Info("NATS JetStream streams configured")

	// Initialize webhook handler
	webhookHandler := webhook.NewHandler(redisClient, js, &webhook.Config{
		WebhookSecret: cfg.GitHubWebhookSecret,
	}, logger)

	// Setup Gin router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(webhook.RequestLogger(logger))

	// Health endpoints
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "ingress-svc"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		// Check dependencies
		if err := redisClient.Ping(c.Request.Context()).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "redis unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "service": "ingress-svc"})
	})

	// Webhook endpoints
	router.POST("/webhooks/github", webhookHandler.GitHubWebhook)
	router.POST("/webhooks/gitlab", webhookHandler.GitLabWebhook)

	// Start HTTP server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		logger.Info("Starting HTTP server", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}
	logger.Info("Server exited")
}
