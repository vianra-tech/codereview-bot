package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/vianra/codereview/aggregation-svc/internal/fingerprint"
	"github.com/vianra/codereview/aggregation-svc/internal/orchestrator"
	"github.com/vianra/codereview/aggregation-svc/internal/sarif"
	"github.com/vianra/codereview/gen/go/proto/analysis/v1"
	"google.golang.org/protobuf/proto"
)

func main() {
	cfg := loadConfig()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting aggregation-svc",
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

	// Initialize components
	fingerprintGen := fingerprint.NewGenerator()
	sarifConverter := sarif.NewSARIFConverter()
	orch := orchestrator.NewOrchestrator(fingerprintGen, sarifConverter, js, redisClient, logger)

	// Setup consumer for analysis results
	ctx := context.Background()
	consumerConfig := jetstream.ConsumerConfig{
		Durable:       "aggregation-svc",
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    3,
		AckWait:       5 * time.Minute,
		FilterSubject: "analysis.results",
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
	var result analysis.AnalysisResult
	if err := proto.Unmarshal(msg.Data(), &result); err != nil {
		return err
	}

	return orch.ProcessResults(ctx, &result)
}

type Config struct {
	NATSURL  string
	RedisURL string
}

func loadConfig() Config {
	return Config{
		NATSURL:  getEnv("NATS_URL", "nats://localhost:4222"),
		RedisURL: getEnv("REDIS_URL", "redis://localhost:6379"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}