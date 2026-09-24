package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	"github.com/vianra/codereview/aggregation-svc/internal/fingerprint"
	"github.com/vianra/codereview/aggregation-svc/internal/sarif"
	analysis "github.com/vianra/codereview/gen/go/proto/analysis/v1"
	"go.uber.org/zap"
)

// Orchestrator coordinates the aggregation pipeline
type Orchestrator struct {
	fingerprintGen *fingerprint.Generator
	sarifConverter *sarif.SARIFConverter
	js             jetstream.JetStream
	redisClient    *redis.Client
	logger         *zap.Logger
}

// NewOrchestrator creates a new aggregation orchestrator
func NewOrchestrator(
	fingerprintGen *fingerprint.Generator,
	sarifConverter *sarif.SARIFConverter,
	js jetstream.JetStream,
	redisClient *redis.Client,
	logger *zap.Logger,
) *Orchestrator {
	return &Orchestrator{
		fingerprintGen: fingerprintGen,
		sarifConverter: sarifConverter,
		js:             js,
		redisClient:    redisClient,
		logger:         logger,
	}
}

// ProcessResults processes analysis results and aggregates them
func (o *Orchestrator) ProcessResults(ctx context.Context, result *analysis.AnalysisResult) error {
	o.logger.Info("Processing analysis results",
		zap.String("run_id", result.RunId),
		zap.String("repo", result.Repository),
		zap.Int("findings", len(result.Findings)),
	)

	// Deduplicate findings using fingerprints
	dedupedFindings := o.deduplicateFindings(result.Findings)

	// Check for existing findings in history (learning patterns)
	enrichedFindings := o.enrichWithHistory(ctx, dedupedFindings)

	// Convert to SARIF for GitHub Security tab
	sarifLog, err := o.sarifConverter.Convert(enrichedFindings, sarif.RunInfo{
		RunID:     result.RunId,
		RepoName:  result.Repository,
		CommitSHA: result.CommitSha,
		Timestamp: result.CompletedAt.AsTime().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("failed to convert to SARIF: %w", err)
	}

	// Store findings in Redis for future deduplication
	if err := o.storeFindings(ctx, result.Repository, result.CommitSha, enrichedFindings); err != nil {
		o.logger.Warn("Failed to store findings in history", zap.Error(err))
	}

	// Publish aggregated results for integration service
	aggregatedResult := &AggregatedResult{
		RunId:       result.RunId,
		Repository:  result.Repository,
		CommitSha:   result.CommitSha,
		Findings:    enrichedFindings,
		SARIF:       sarifLog,
		CompletedAt: time.Now(),
	}

	if err := o.publishAggregated(ctx, aggregatedResult); err != nil {
		return fmt.Errorf("failed to publish aggregated results: %w", err)
	}

	o.logger.Info("Aggregation completed",
		zap.String("run_id", result.RunId),
		zap.Int("original_findings", len(result.Findings)),
		zap.Int("deduped_findings", len(enrichedFindings)),
	)

	return nil
}

// deduplicateFindings removes duplicate findings using fingerprint matching
func (o *Orchestrator) deduplicateFindings(findings []*analysis.Finding) []*analysis.Finding {
	seen := make(map[string]*analysis.Finding)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Process in parallel for large finding sets
	for _, finding := range findings {
		wg.Add(1)
		go func(f *analysis.Finding) {
			defer wg.Done()
			fp := o.fingerprintGen.Generate(f)

			mu.Lock()
			defer mu.Unlock()

			if existing, ok := seen[fp]; ok {
				// Keep the one with higher severity
				if severityRank(f.Severity) > severityRank(existing.Severity) {
					seen[fp] = f
				}
			} else {
				seen[fp] = f
			}
		}(finding)
	}

	wg.Wait()

	result := make([]*analysis.Finding, 0, len(seen))
	for _, f := range seen {
		result = append(result, f)
	}

	return result
}

// enrichWithHistory checks if findings match historical patterns
func (o *Orchestrator) enrichWithHistory(ctx context.Context, findings []*analysis.Finding) []*analysis.Finding {
	// Check Redis for historical patterns
	// In production, this would also query the learning_patterns table
	return findings
}

// storeFindings stores findings in Redis for future reference
func (o *Orchestrator) storeFindings(ctx context.Context, repo, commitSHA string, findings []*analysis.Finding) error {
	key := fmt.Sprintf("findings:%s:%s", repo, commitSHA)
	data, err := json.Marshal(findings)
	if err != nil {
		return err
	}

	return o.redisClient.Set(ctx, key, data, 30*24*time.Hour).Err()
}

// publishAggregated publishes aggregated results to NATS
func (o *Orchestrator) publishAggregated(ctx context.Context, result *AggregatedResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	_, err = o.js.Publish(ctx, "analysis.aggregated", data)
	return err
}

// severityRank returns a numeric rank for severity comparison
func severityRank(severity string) int {
	switch severity {
	case "critical":
		return 5
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

// AggregatedResult represents the aggregated analysis result
type AggregatedResult struct {
	RunId       string              `json:"run_id"`
	Repository  string              `json:"repository"`
	CommitSha   string              `json:"commit_sha"`
	Findings    []*analysis.Finding `json:"findings"`
	SARIF       *sarif.SARIFLog     `json:"sarif"`
	CompletedAt time.Time           `json:"completed_at"`
}
