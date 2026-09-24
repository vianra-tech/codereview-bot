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
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/vianra/codereview/analysis-svc/internal/git"
	"github.com/vianra/codereview/analysis-svc/internal/llm"
	"github.com/vianra/codereview/analysis-svc/internal/orchestrator"
	"github.com/vianra/codereview/analysis-svc/internal/rules"
	events "github.com/vianra/codereview/gen/go/proto/events/v1"
)

func main() {
	cfg := loadConfig()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting analysis-svc",
		zap.String("version", "0.1.0"),
	)

	// Initialize Redis (for idempotency)
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
	gitManager := git.NewManager(cfg.WorkDir)
	ruleEngine := rules.NewEngine()

	// Load builtin rules
	ctx := context.Background()
	if err := ruleEngine.LoadRuleSets(ctx, rules.BuiltinRuleSets()); err != nil {
		logger.Fatal("Failed to load builtin rules", zap.Error(err))
	}

	// Optional semantic review client
	var llmClient *llm.Client
	if cfg.NVIDIANIMAPIKey != "" {
		llmClient = llm.NewClient(cfg.NVIDIANIMURL, cfg.NVIDIANIMAPIKey, cfg.NVIDIANIMModel, logger)
	}

	// Create orchestrator
	orch := orchestrator.NewOrchestrator(gitManager, ruleEngine, llmClient, js, logger, cfg.WorkDir)

	// Setup consumer for raw events
	consumerConfig := jetstream.ConsumerConfig{
		Durable:       "analysis-svc",
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    3,
		AckWait:       5 * time.Minute,
		FilterSubject: "events.raw.>",
	}

	consumer, err := js.CreateOrUpdateConsumer(ctx, "EVENTS", consumerConfig)
	if err != nil {
		logger.Fatal("Failed to create consumer", zap.Error(err))
	}

	msgs, err := consumer.Messages()
	if err != nil {
		logger.Fatal("Failed to create message iterator", zap.Error(err))
	}

	consumeCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Process messages in a goroutine
	go func() {
		for {
			msg, err := msgs.Next()
			if err != nil {
				select {
				case <-consumeCtx.Done():
					return
				default:
					logger.Warn("Consumer iterator error", zap.Error(err))
					time.Sleep(time.Second)
					continue
				}
			}

			if err := processMessage(consumeCtx, msg, orch, redisClient, logger); err != nil {
				logger.Error("Failed to process message", zap.Error(err))
				msg.Nak()
			} else {
				msg.Ack()
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

func processMessage(ctx context.Context, msg jetstream.Msg, orch *orchestrator.Orchestrator, redisClient *redis.Client, logger *zap.Logger) error {
	// Ingress publishes normalized events as JSON
	var event events.NormalizedEvent
	if err := protojson.Unmarshal(msg.Data(), &event); err != nil {
		return err
	}

	// Check idempotency
	idempotencyKey := "analysis:" + event.EventId
	exists, err := redisClient.Exists(ctx, idempotencyKey).Result()
	if err != nil {
		return err
	}
	if exists > 0 {
		logger.Info("Skipping duplicate analysis", zap.String("event_id", event.EventId))
		return nil
	}

	// Run analysis
	if err := orch.StartAnalysis(ctx, &event); err != nil {
		return err
	}

	// Mark as processed
	return redisClient.Set(ctx, idempotencyKey, "1", 7*24*time.Hour).Err()
}

type Config struct {
	NATSURL         string
	RedisURL        string
	WorkDir         string
	NVIDIANIMURL    string
	NVIDIANIMAPIKey string
	NVIDIANIMModel  string
}

func loadConfig() Config {
	return Config{
		NATSURL:         getEnv("NATS_URL", "nats://localhost:4222"),
		RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379"),
		WorkDir:         getEnv("WORK_DIR", "/tmp/codereview"),
		NVIDIANIMURL:    getEnv("NVIDIA_NIM_URL", "https://integrate.api.nvidia.com/v1"),
		NVIDIANIMAPIKey: getEnv("NVIDIA_NIM_API_KEY", ""),
		NVIDIANIMModel:  getEnv("NVIDIA_NIM_MODEL", "meta/llama-3.1-70b-instruct"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
