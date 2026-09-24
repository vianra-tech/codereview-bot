package orchestrator

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/vianra/codereview/analysis-svc/internal/ast"
	"github.com/vianra/codereview/analysis-svc/internal/git"
	"github.com/vianra/codereview/analysis-svc/internal/llm"
	"github.com/vianra/codereview/analysis-svc/internal/rules"
	analysis "github.com/vianra/codereview/gen/go/proto/analysis/v1"
	events "github.com/vianra/codereview/gen/go/proto/events/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Orchestrator coordinates the analysis pipeline
type Orchestrator struct {
	gitManager *git.Manager
	ruleEngine *rules.Engine
	llmClient  *llm.Client
	js         jetstream.JetStream
	logger     *zap.Logger
	workDir    string
}

// NewOrchestrator creates a new analysis orchestrator
func NewOrchestrator(
	gitManager *git.Manager,
	ruleEngine *rules.Engine,
	llmClient *llm.Client,
	js jetstream.JetStream,
	logger *zap.Logger,
	workDir string,
) *Orchestrator {
	return &Orchestrator{
		gitManager: gitManager,
		ruleEngine: ruleEngine,
		llmClient:  llmClient,
		js:         js,
		logger:     logger,
		workDir:    workDir,
	}
}

// StartAnalysis begins the analysis pipeline for a normalized event
func (o *Orchestrator) StartAnalysis(ctx context.Context, event *events.NormalizedEvent) error {
	runID := fmt.Sprintf("run-%s-%d", event.EventId, time.Now().Unix())
	startedAt := time.Now()

	o.logger.Info("Starting analysis",
		zap.String("run_id", runID),
		zap.String("repo", event.Repository.FullName),
		zap.String("event_type", event.EventType),
		zap.String("commit", o.extractCommitSHA(event)),
	)

	// Extract commit info from payload
	commitSHA := o.extractCommitSHA(event)
	if commitSHA == "" {
		return fmt.Errorf("could not extract commit SHA from event")
	}

	// Clone repository
	repoURL := fmt.Sprintf("https://github.com/%s.git", event.Repository.FullName)
	repoPath, err := o.gitManager.CloneRepo(ctx, repoURL, commitSHA)
	if err != nil {
		return fmt.Errorf("failed to clone repo: %w", err)
	}

	// Determine changed files
	var changedFiles []string
	if event.EventType == "pull_request" {
		// For PRs, compare base and head
		baseSHA := o.extractBaseSHA(event)
		if baseSHA != "" {
			changedFiles, err = o.gitManager.GetChangedFiles(ctx, repoPath, baseSHA, commitSHA)
			if err != nil {
				o.logger.Warn("Failed to get changed files, analyzing all", zap.Error(err))
			}
		}
	}

	// If no changed files determined, analyze all supported files
	if len(changedFiles) == 0 {
		changedFiles, err = o.getAllSupportedFiles(repoPath)
		if err != nil {
			return fmt.Errorf("failed to get supported files: %w", err)
		}
	}

	// Run analysis in parallel for each file
	findings, err := o.analyzeFiles(ctx, repoPath, changedFiles)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Aggregate and publish results
	analysisResult := &analysis.AnalysisResult{
		RunId:       runID,
		Repository:  event.Repository.FullName,
		CommitSha:   commitSHA,
		Findings:    findings,
		CompletedAt: timestamppb.Now(),
		DurationMs:  time.Since(startedAt).Milliseconds(),
	}

	// Publish to NATS for aggregation
	if err := o.publishResults(ctx, analysisResult); err != nil {
		return fmt.Errorf("failed to publish results: %w", err)
	}

	o.logger.Info("Analysis completed",
		zap.String("run_id", runID),
		zap.Int("findings_count", len(findings)),
	)

	return nil
}

// extractCommitSHA extracts commit SHA from event payload
func (o *Orchestrator) extractCommitSHA(event *events.NormalizedEvent) string {
	if event.Payload == nil {
		return ""
	}

	fields := event.Payload.GetFields()
	if headCommit, ok := fields["head_commit"]; ok {
		if sc, ok := headCommit.GetStructValue().GetFields()["id"]; ok {
			return sc.GetStringValue()
		}
	}

	if after, ok := fields["after"]; ok {
		return after.GetStringValue()
	}

	return ""
}

// extractBaseSHA extracts base commit SHA for PR events
func (o *Orchestrator) extractBaseSHA(event *events.NormalizedEvent) string {
	if event.Payload == nil {
		return ""
	}

	fields := event.Payload.GetFields()
	if pr, ok := fields["pull_request"]; ok {
		if sc, ok := pr.GetStructValue().GetFields()["base"]; ok {
			if base, ok := sc.GetStructValue().GetFields()["sha"]; ok {
				return base.GetStringValue()
			}
		}
	}

	return ""
}

// getAllSupportedFiles finds all supported files in the repository
func (o *Orchestrator) getAllSupportedFiles(repoPath string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			// Skip VCS and dependency directories
			if d.Name() == ".git" || d.Name() == "vendor" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		lang := ast.DetectLanguage(d.Name())
		if lang != "" {
			rel, err := filepath.Rel(repoPath, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

// analyzeFiles runs analysis on multiple files in parallel
func (o *Orchestrator) analyzeFiles(ctx context.Context, repoPath string, filePaths []string) ([]*analysis.Finding, error) {
	var (
		findings []*analysis.Finding
		mu       sync.Mutex
		wg       sync.WaitGroup
	)

	// Limit concurrency
	sem := make(chan struct{}, 10)

	for _, filePath := range filePaths {
		wg.Add(1)
		go func(fp string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			fileFindings, err := o.analyzeFile(ctx, repoPath, fp)
			if err != nil {
				o.logger.Error("File analysis error",
					zap.String("file", fp), zap.Error(err))
				return
			}

			mu.Lock()
			findings = append(findings, fileFindings...)
			mu.Unlock()
		}(filePath)
	}

	wg.Wait()

	return findings, nil
}

// analyzeFile runs analysis on a single file
func (o *Orchestrator) analyzeFile(ctx context.Context, repoPath, filePath string) ([]*analysis.Finding, error) {
	// Read file content
	content, err := o.gitManager.GetFileContent(ctx, repoPath, "", filePath)
	if err != nil {
		return nil, err
	}

	// Detect language
	lang := ast.DetectLanguage(filePath)
	if lang == "" {
		return nil, fmt.Errorf("unsupported language for %s", filePath)
	}

	// Run rule engine against structural input derived from source
	astData := o.extractASTData(content, filePath)
	ruleFindings, err := o.ruleEngine.Evaluate(ctx, astData)
	if err != nil {
		return nil, fmt.Errorf("rule engine error: %w", err)
	}

	// Semantic Review Layer (LLM)
	// We only send critical/high findings to the LLM to save costs and reduce noise
	var finalFindings []*analysis.Finding
	for _, rf := range ruleFindings {
		if o.llmClient != nil && (rf.Severity == "critical" || rf.Severity == "high") {
			review, err := o.llmClient.ReviewFinding(ctx, llm.ReviewRequest{
				FilePath:    rf.FilePath,
				CodeSnippet: rf.CodeSnippet,
				Finding: llm.Finding{
					RuleID:   rf.RuleID,
					Message:  rf.Message,
					Severity: rf.Severity,
				},
			})

			switch {
			case err == nil && review.IsTruePositive:
				// Update finding with AI reasoning and suggested fix
				rf.Message = fmt.Sprintf("%s\n\nAI Reasoning: %s", rf.Message, review.Reasoning)
				finalFindings = append(finalFindings, &analysis.Finding{
					RuleId:      rf.RuleID,
					Severity:    review.NewSeverity,
					Message:     rf.Message,
					FilePath:    rf.FilePath,
					StartLine:   int32(rf.StartLine),
					EndLine:     int32(rf.EndLine),
					StartColumn: int32(rf.StartColumn),
					EndColumn:   int32(rf.EndColumn),
					CodeSnippet: rf.CodeSnippet,
				})
			case err == nil:
				// Filter out false positives identified by LLM
				continue
			default:
				// If LLM fails, we keep the static finding to be safe
				finalFindings = append(finalFindings, &analysis.Finding{
					RuleId:      rf.RuleID,
					Severity:    rf.Severity,
					Message:     rf.Message,
					FilePath:    rf.FilePath,
					StartLine:   int32(rf.StartLine),
					EndLine:     int32(rf.EndLine),
					StartColumn: int32(rf.StartColumn),
					EndColumn:   int32(rf.EndColumn),
					CodeSnippet: rf.CodeSnippet,
				})
			}
		} else {
			// Low/Info findings skip LLM and go straight to results
			finalFindings = append(finalFindings, &analysis.Finding{
				RuleId:      rf.RuleID,
				Severity:    rf.Severity,
				Message:     rf.Message,
				FilePath:    rf.FilePath,
				StartLine:   int32(rf.StartLine),
				EndLine:     int32(rf.EndLine),
				StartColumn: int32(rf.StartColumn),
				EndColumn:   int32(rf.EndColumn),
				CodeSnippet: rf.CodeSnippet,
			})
		}
	}

	return finalFindings, nil
}

// extractASTData extracts structured data from source for Rego input
func (o *Orchestrator) extractASTData(source, filePath string) map[string]interface{} {
	return map[string]interface{}{
		"file_path": filePath,
		"source":    source,
		"language":  ast.DetectLanguage(filePath).String(),
		"lines":     splitLines(source),
	}
}

// splitLines splits source into lines
func splitLines(source string) []map[string]interface{} {
	lines := []map[string]interface{}{}
	for i, line := range strings.Split(source, "\n") {
		lines = append(lines, map[string]interface{}{
			"number":  i + 1,
			"content": line,
		})
	}
	return lines
}

// publishResults publishes analysis results to NATS
func (o *Orchestrator) publishResults(ctx context.Context, result *analysis.AnalysisResult) error {
	data, err := proto.Marshal(result)
	if err != nil {
		return err
	}

	_, err = o.js.Publish(ctx, "analysis.results", data)
	return err
}
