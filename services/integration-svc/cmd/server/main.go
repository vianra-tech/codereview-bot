package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/vianra/codereview/integration-svc/internal/github"
	"github.com/vianra/codereview/integration-svc/internal/orchestrator"
	"github.com/vianra/codereview/gen/go/proto/analysis/v1"
	"google.golang.org/protobuf/proto"
)

func main() {
	cfg := loadConfig()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting integration-svc",
		zap.String("version", "0.1.0"),
	)

	// Initialize Redis
	redisOpt, _ := redis.ParseURL(cfg.RedisURL)
	redisClient := redis.NewClient(redisOpt)
	defer redisClient.Close()

	// Initialize NATS
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		logger.Fatal("Failed to connect to NATS", zap.Error(err))
	}
	defer nc.Drain()

	js, err := jetstream.New(nc)
	if err != nil {
		logger.Fatal("Failed to create JetStream context", zap.Error(err))
	}

	// Initialize GitHub client
	githubClient, err := github.NewClient(cfg.GitHubAppID, []byte(cfg.GitHubAppPrivateKey), logger)
	if err != nil {
		logger.Fatal("Failed to create GitHub client", zap.Error(err))
	}

	formatter := github.NewCommentFormatter()
	orch := orchestrator.NewOrchestrator(githubClient, formatter, js, redisClient, logger)

	// Setup consumer for aggregated results
	ctx := context.Background()
	consumerConfig := jetstream.ConsumerConfig{
		Durable:       "integration-svc",
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    3,
		AckWait:       5 * time.Minute,
		FilterSubject: "analysis.aggregated",
	}

	consumer, err := js.CreateOrUpdateConsumer(ctx, "EVENTS", consumerConfig)
	if err != nil {
		logger.Fatal("Failed to create consumer", zap.Error(err))
	}

	consumeCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msgs, err := consumer.Messages()
	if err != nil {
		logger.Fatal("Failed to create message iterator", zap.Error(err))
	}

	// Process messages
	go func() {
		for {
			select {
			case <-consumeCtx.Done():
				return
			case msg, ok := <-msgs:
				if !ok {
					return
				}
				if err := processMessage(consumeCtx, msg, orch, logger); err != nil {
					logger.Error("Failed to process message", zap.Error(err))
					msg.Nak()
				} else {
					msg.Ack()
				}
			}
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")
	cancel()
	logger.Info("Stopped")
}

func processMessage(ctx context.Context, msg jetstream.Msg, orch *orchestrator.Orchestrator, logger *zap.Logger) error {
	var result orchestrator.AggregatedResult
	if err := proto.Unmarshal(msg.Data(), &result); err != nil {
		return err
	}

	return orch.ProcessAggregated(ctx, &result)
}

type Config struct {
	NATSURL               string
	RedisURL              string
	GitHubAppID           int64
	GitHubAppPrivateKey   string
}

func loadConfig() Config {
	return Config{
		NATSURL:             getEnv("NATS_URL", "nats://localhost:4222"),
		RedisURL:            getEnv("REDIS_URL", "redis://localhost:6379"),
		GitHubAppID:         getEnvAsInt64("GITHUB_APP_ID", 0),
		GitHubAppPrivateKey: getEnv("GITHUB_APP_PRIVATE_KEY", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		var result int64
		fmt.Sscanf(value, "%d", &result)
		return result
	}
	return defaultValue
}