package nats

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// Connect establishes a connection to NATS
func Connect(url string, logger *zap.Logger) (*nats.Conn, error) {
	nc, err := nats.Connect(url,
		nats.Name("ingress-svc"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			logger.Warn("NATS disconnected", zap.Error(err))
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected", zap.String("url", nc.ConnectedUrl()))
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			logger.Info("NATS connection closed")
		}),
	)
	if err != nil {
		return nil, err
	}
	return nc, nil
}

// SetupStreams creates the necessary JetStream streams
func SetupStreams(ctx context.Context, js jetstream.JetStream, logger *zap.Logger) error {
	streams := []jetstream.StreamConfig{
		{
			Name:        "EVENTS",
			Description: "Raw webhook events from Git providers",
			Subjects:    []string{"events.raw.>"},
			Retention:   jetstream.LimitsPolicy,
			MaxAge:      24 * time.Hour,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
		},
		{
			Name:        "ANALYSIS",
			Description: "Analysis job queue",
			Subjects:    []string{"analysis.jobs.>"},
			Retention:   jetstream.WorkQueuePolicy,
			MaxAge:      72 * time.Hour,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
		},
		{
			Name:        "AGGREGATION",
			Description: "Aggregation job queue",
			Subjects:    []string{"aggregation.jobs.>"},
			Retention:   jetstream.WorkQueuePolicy,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
		},
		{
			Name:        "RESULTS",
			Description: "Analysis results and aggregated output",
			Subjects:    []string{"analysis.results", "analysis.aggregated"},
			Retention:   jetstream.LimitsPolicy,
			MaxAge:      72 * time.Hour,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
		},
		{
			Name:        "INTEGRATION",
			Description: "Git provider integration queue",
			Subjects:    []string{"integration.jobs.>"},
			Retention:   jetstream.WorkQueuePolicy,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
		},
		{
			Name:        "NOTIFICATIONS",
			Description: "Notification delivery queue",
			Subjects:    []string{"notifications.jobs.>"},
			Retention:   jetstream.WorkQueuePolicy,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
		},
	}

	for _, cfg := range streams {
		_, err := js.CreateOrUpdateStream(ctx, cfg)
		if err != nil {
			logger.Error("Failed to create stream", zap.String("stream", cfg.Name), zap.Error(err))
			return err
		}
		logger.Info("Stream created/updated", zap.String("stream", cfg.Name))
	}
	return nil
}
