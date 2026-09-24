package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	analysis "github.com/vianra/codereview/gen/go/proto/analysis/v1"
	"github.com/vianra/codereview/integration-svc/internal/github"
	"go.uber.org/zap"
)

// Orchestrator coordinates the GitHub integration pipeline
type Orchestrator struct {
	githubClient *github.Client
	formatter    *github.CommentFormatter
	js           jetstream.JetStream
	redisClient  *redis.Client
	logger       *zap.Logger
}

// NewOrchestrator creates a new integration orchestrator
func NewOrchestrator(
	githubClient *github.Client,
	formatter *github.CommentFormatter,
	js jetstream.JetStream,
	redisClient *redis.Client,
	logger *zap.Logger,
) *Orchestrator {
	return &Orchestrator{
		githubClient: githubClient,
		formatter:    formatter,
		js:           js,
		redisClient:  redisClient,
		logger:       logger,
	}
}

// ProcessAggregated processes aggregated results and posts to GitHub
func (o *Orchestrator) ProcessAggregated(ctx context.Context, result *AggregatedResult) error {
	o.logger.Info("Processing aggregated results for GitHub",
		zap.String("run_id", result.RunId),
		zap.String("repo", result.Repository),
		zap.Int("findings", len(result.Findings)),
	)

	// Parse repository info
	repoOwner, repoName, err := parseRepoName(result.Repository)
	if err != nil {
		return fmt.Errorf("invalid repo name: %w", err)
	}

	// Get installation ID
	installationID, err := o.githubClient.GetInstallation(ctx, repoOwner, repoName)
	if err != nil {
		return fmt.Errorf("failed to get installation: %w", err)
	}

	// Determine if this is a PR or push event
	// For now, we'll check if we have a PR number in the metadata
	prNumber := o.extractPRNumber(result)

	if prNumber > 0 {
		// PR event - create review with inline comments
		return o.processPR(ctx, installationID, repoOwner, repoName, prNumber, result)
	} else {
		// Push event - create check run on the commit
		return o.processPush(ctx, installationID, repoOwner, repoName, result)
	}
}

// processPR handles pull request integration
func (o *Orchestrator) processPR(ctx context.Context, installationID int64, repoOwner, repoName string, prNumber int, result *AggregatedResult) error {
	// Create check run first
	checkRun, err := o.githubClient.CreateCheckRun(ctx, installationID, repoOwner, repoName, result.CommitSha,
		"CodeReview.ai", "in_progress", "", nil)
	if err != nil {
		return fmt.Errorf("failed to create check run: %w", err)
	}

	// Create review with inline comments
	comments := o.formatter.CreateReviewComments(result.Findings, result.CommitSha)
	conclusion, summary := o.formatter.FormatCheckRunConclusion(result.Findings)

	body := fmt.Sprintf("## CodeReview.ai Analysis\n\n%s\n\n%d findings total", summary, len(result.Findings))

	err = o.githubClient.CreateReview(ctx, installationID, repoOwner, repoName, prNumber, result.CommitSha, body, "COMMENT", comments)
	if err != nil {
		o.logger.Warn("Failed to create review, falling back to check run only", zap.Error(err))
	}

	// Update check run with final results
	output := o.formatter.FormatCheckRunOutput(result.Findings)
	_, err = o.githubClient.UpdateCheckRun(ctx, installationID, repoOwner, repoName, checkRun.GetID(), "completed", conclusion, output)
	if err != nil {
		return fmt.Errorf("failed to update check run: %w", err)
	}

	return nil
}

// processPush handles push event integration (check run only)
func (o *Orchestrator) processPush(ctx context.Context, installationID int64, repoOwner, repoName string, result *AggregatedResult) error {
	output := o.formatter.FormatCheckRunOutput(result.Findings)
	conclusion, _ := o.formatter.FormatCheckRunConclusion(result.Findings)

	_, err := o.githubClient.CreateCheckRun(ctx, installationID, repoOwner, repoName, result.CommitSha,
		"CodeReview.ai", "completed", conclusion, output)
	if err != nil {
		return fmt.Errorf("failed to create check run: %w", err)
	}

	return nil
}

// extractPRNumber extracts PR number from result metadata
func (o *Orchestrator) extractPRNumber(result *AggregatedResult) int {
	// In a real implementation, this would come from the event metadata
	// For now, return 0 to indicate push event
	return 0
}

// parseRepoName parses "owner/repo" into components
func parseRepoName(fullName string) (string, string, error) {
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo name format: %s", fullName)
	}
	return parts[0], parts[1], nil
}

// AggregatedResult represents the aggregated analysis result
type AggregatedResult struct {
	RunId       string              `json:"run_id"`
	Repository  string              `json:"repository"`
	CommitSha   string              `json:"commit_sha"`
	Findings    []*analysis.Finding `json:"findings"`
	SARIF       interface{}         `json:"sarif"`
	CompletedAt time.Time           `json:"completed_at"`
}
