package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	events "github.com/vianra/codereview/gen/go/proto/events/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Handler handles webhook requests
type Handler struct {
	redisClient *redis.Client
	js          jetstream.JetStream
	config      *Config
	logger      *zap.Logger
}

// Config holds webhook configuration
type Config struct {
	WebhookSecret string
}

// NewHandler creates a new webhook handler
func NewHandler(redisClient *redis.Client, js jetstream.JetStream, config *Config, logger *zap.Logger) *Handler {
	return &Handler{
		redisClient: redisClient,
		js:          js,
		config:      config,
		logger:      logger,
	}
}

// GitHubWebhook handles GitHub webhook events
func (h *Handler) GitHubWebhook(c *gin.Context) {
	// Read body
	body, err := c.GetRawData()
	if err != nil {
		h.logger.Error("Failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Verify HMAC signature
	signature := c.GetHeader("X-Hub-Signature-256")
	if err := VerifyHMAC(body, signature, h.config.WebhookSecret); err != nil {
		h.logger.Warn("Invalid webhook signature", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	// Check idempotency
	deliveryID := c.GetHeader("X-GitHub-Delivery")
	if deliveryID == "" {
		deliveryID = c.GetHeader("X-GitHub-Event") + "-" + time.Now().Format(time.RFC3339Nano)
	}

	isNew, err := CheckAndSetIdempotencyKey(c.Request.Context(), h.redisClient, deliveryID)
	if err != nil {
		h.logger.Error("Failed to check idempotency", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !isNew {
		h.logger.Info("Duplicate webhook delivery, skipping", zap.String("delivery_id", deliveryID))
		c.Status(http.StatusAccepted)
		return
	}

	// Parse event type
	eventType := c.GetHeader("X-GitHub-Event")
	if eventType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing X-GitHub-Event header"})
		return
	}

	// Normalize event
	event, err := h.normalizeGitHubEvent(c.Request.Context(), body, c.Request.Header, eventType, deliveryID)
	if err != nil {
		h.logger.Error("Failed to normalize event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process event"})
		return
	}

	// Publish to NATS
	subject := "events.raw.github." + eventType
	eventData, err := protojson.Marshal(event)
	if err != nil {
		h.logger.Error("Failed to marshal event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process event"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	_, err = h.js.Publish(ctx, subject, eventData)
	if err != nil {
		h.logger.Error("Failed to publish event to NATS", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue event"})
		return
	}

	h.logger.Info("Event published to NATS",
		zap.String("subject", subject),
		zap.String("event_type", eventType),
		zap.String("delivery_id", deliveryID),
	)

	c.Status(http.StatusAccepted)
}

// GitLabWebhook handles GitLab webhook events
func (h *Handler) GitLabWebhook(c *gin.Context) {
	// Similar to GitHub but with GitLab-specific headers
	body, err := c.GetRawData()
	if err != nil {
		h.logger.Error("Failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// GitLab uses X-Gitlab-Token for verification
	token := c.GetHeader("X-Gitlab-Token")
	if token != h.config.WebhookSecret {
		h.logger.Warn("Invalid GitLab webhook token")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	eventType := c.GetHeader("X-Gitlab-Event")
	if eventType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing X-Gitlab-Event header"})
		return
	}

	deliveryID := c.GetHeader("X-Gitlab-Event-UUID")
	if deliveryID == "" {
		deliveryID = "gitlab-" + eventType + "-" + time.Now().Format(time.RFC3339Nano)
	}

	isNew, err := CheckAndSetIdempotencyKey(c.Request.Context(), h.redisClient, deliveryID)
	if err != nil {
		h.logger.Error("Failed to check idempotency", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !isNew {
		h.logger.Info("Duplicate webhook delivery, skipping", zap.String("delivery_id", deliveryID))
		c.Status(http.StatusAccepted)
		return
	}

	// Normalize event
	event, err := h.normalizeGitLabEvent(c.Request.Context(), body, c.Request.Header, eventType, deliveryID)
	if err != nil {
		h.logger.Error("Failed to normalize event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process event"})
		return
	}

	// Publish to NATS
	subject := "events.raw.gitlab." + strings.ToLower(eventType)
	eventData, err := protojson.Marshal(event)
	if err != nil {
		h.logger.Error("Failed to marshal event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process event"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	_, err = h.js.Publish(ctx, subject, eventData)
	if err != nil {
		h.logger.Error("Failed to publish event to NATS", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue event"})
		return
	}

	h.logger.Info("Event published to NATS",
		zap.String("subject", subject),
		zap.String("event_type", eventType),
		zap.String("delivery_id", deliveryID),
	)

	c.Status(http.StatusAccepted)
}

// VerifyHMAC verifies the HMAC-SHA256 signature
func VerifyHMAC(payload []byte, signature, secret string) error {
	if secret == "" {
		return errors.New("webhook secret not configured")
	}

	const prefix = "sha256="
	if !strings.HasPrefix(signature, prefix) {
		return errors.New("invalid signature format")
	}
	expectedSig := signature[len(prefix):]

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	actualSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expectedSig), []byte(actualSig)) {
		return errors.New("invalid signature")
	}
	return nil
}

// CheckAndSetIdempotencyKey checks if a delivery ID was already processed
func CheckAndSetIdempotencyKey(ctx context.Context, redisClient *redis.Client, deliveryID string) (bool, error) {
	key := "webhook_idempotency:" + deliveryID
	result, err := redisClient.SetNX(ctx, key, "1", 7*24*time.Hour).Result()
	if err != nil {
		return false, err
	}
	return result, nil // true = first time, false = duplicate
}

// normalizeGitHubEvent converts GitHub webhook to normalized event
func (h *Handler) normalizeGitHubEvent(ctx context.Context, body []byte, header http.Header, eventType, deliveryID string) (*events.NormalizedEvent, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	// Extract repository info
	repo := events.Repository{}
	if repoData, ok := payload["repository"].(map[string]interface{}); ok {
		if id, ok := repoData["id"].(float64); ok {
			repo.Id = int64(id)
		}
		if fullName, ok := repoData["full_name"].(string); ok {
			repo.FullName = fullName
		}
		if defaultBranch, ok := repoData["default_branch"].(string); ok {
			repo.DefaultBranch = defaultBranch
		}
		if language, ok := repoData["language"].(string); ok {
			repo.Language = language
		}
		if isPrivate, ok := repoData["private"].(bool); ok {
			repo.IsPrivate = isPrivate
		}
	}

	// Extract actor info
	actor := events.Actor{}
	if actorData, ok := payload["sender"].(map[string]interface{}); ok {
		if id, ok := actorData["id"].(float64); ok {
			actor.Id = int64(id)
		}
		if login, ok := actorData["login"].(string); ok {
			actor.Login = login
		}
		if typ, ok := actorData["type"].(string); ok {
			actor.Type = typ
		}
	}

	// Convert payload to structpb
	payloadStruct, err := structpb.NewStruct(payload)
	if err != nil {
		return nil, err
	}

	// Get installation ID
	var installationID int64
	if instData, ok := payload["installation"].(map[string]interface{}); ok {
		if id, ok := instData["id"].(float64); ok {
			installationID = int64(id)
		}
	}

	event := &events.NormalizedEvent{
		EventId:        deliveryID,
		Source:         "github",
		EventType:      eventType,
		Timestamp:      timestamppb.Now(),
		InstallationId: installationID,
		Repository:     &repo,
		Actor:          &actor,
		Payload:        payloadStruct,
		Metadata: &events.EventMetadata{
			WebhookId:           deliveryID,
			SignatureVerified:   true,
			ProcessingStartedAt: timestamppb.Now(),
		},
	}

	return event, nil
}

// normalizeGitLabEvent converts GitLab webhook to normalized event
func (h *Handler) normalizeGitLabEvent(ctx context.Context, body []byte, header http.Header, eventType, deliveryID string) (*events.NormalizedEvent, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	// Extract repository info
	repo := events.Repository{}
	if repoData, ok := payload["project"].(map[string]interface{}); ok {
		if id, ok := repoData["id"].(float64); ok {
			repo.Id = int64(id)
		}
		if fullName, ok := repoData["path_with_namespace"].(string); ok {
			repo.FullName = fullName
		}
		if defaultBranch, ok := repoData["default_branch"].(string); ok {
			repo.DefaultBranch = defaultBranch
		}
	}

	// Extract actor info
	actor := events.Actor{}
	if actorData, ok := payload["user"].(map[string]interface{}); ok {
		if id, ok := actorData["id"].(float64); ok {
			actor.Id = int64(id)
		}
		if login, ok := actorData["username"].(string); ok {
			actor.Login = login
		}
		actor.Type = "User"
	}

	// Convert payload to structpb
	payloadStruct, err := structpb.NewStruct(payload)
	if err != nil {
		return nil, err
	}

	event := &events.NormalizedEvent{
		EventId:        deliveryID,
		Source:         "gitlab",
		EventType:      strings.ToLower(eventType),
		Timestamp:      timestamppb.Now(),
		InstallationId: 0, // GitLab doesn't have installation concept
		Repository:     &repo,
		Actor:          &actor,
		Payload:        payloadStruct,
		Metadata: &events.EventMetadata{
			WebhookId:           deliveryID,
			SignatureVerified:   true,
			ProcessingStartedAt: timestamppb.Now(),
		},
	}

	return event, nil
}

// RequestLogger is a Gin middleware for request logging
func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		logger.Info("HTTP request",
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.String("client_ip", clientIP),
		)
	}
}
