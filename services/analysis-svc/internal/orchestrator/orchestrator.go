package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/vianra/codereview/analysis-svc/internal/git"
	"github.com/vianra/codereview/analysis-svc/internal/ast"
	"github.com/vianra/codereview/analysis-svc/internal/rules"
	"github.com/vianra/codereview/gen/go/proto/analysis/v1"
	"github.com/vianra/codereview/gen/go/proto/events/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Orchestrator coordinates the analysis pipeline
type Orchestrator struct {
	gitManager *git.Manager
	astParser  *ast.Parser
	ruleEngine *rules.Engine
	js         jetstream.JetStream
	logger     *zap.Logger
	workDir    string
}

// NewOrchestrator creates a new analysis orchestrator
func NewOrchestrator(
	gitManager *git.Manager,
	astParser *ast.Parser,
	ruleEngine *rules.Engine,
	js jetstream.JetStream,
	logger *zap.Logger,
	workDir string,
) *Orchestrator {
	return &Orchestrator{
		gitManager: gitManager,
		astParser:  astParser,
		ruleEngine: ruleEngine,
		js:         js,
		logger:     logger,
		workDir:    workDir,
	}
}

// StartAnalysis begins the analysis pipeline for a normalized event
func (o *Orchestrator) StartAnalysis(ctx context.Context, event *events.NormalizedEvent) error {
	runID := fmt.Sprintf("run-%s-%d", event.EventId, time.Now().Unix())
	
	o.logger.Info("Starting analysis",
		zap.String("run_id", runID),
		zap.String("repo", event.Repository.FullName),
		zap.String("event_type", event.EventType),
		zap.String("commit", event.Payload.GetFields()["head_commit"].GetStructValue().GetFields()["id"].GetStringValue()),
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
		RunId:        runID,
		Repository:   event.Repository.FullName,
		CommitSha:    commitSHA,
		Findings:     findings,
		CompletedAt:  timestamppb.Now(),
		DurationMs:   int64(time.Since(time.Now()).Milliseconds()),
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
	
	// This would walk the repo and filter by supported extensions
	// For now, return empty to use the diff-based approach
	return files, nil
}

// analyzeFiles runs analysis on multiple files in parallel
func (o *Orchestrator) analyzeFiles(ctx context.Context, repoPath string, filePaths []string) ([]*analysis.Finding, error) {
	var (
		findings []*analysis.Finding
		mu       sync.Mutex
		wg       sync.WaitGroup
		errChan  = make(chan error, len(filePaths))
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
				errChan <- err
				return
			}

			mu.Lock()
			findings = append(findings, fileFindings...)
			mu.Unlock()
		}(filePath)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			o.logger.Error("File analysis error", zap.Error(err))
			// Continue with other files
		}
	}

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
		return nil, nil // Unsupported language
	}

	// Parse AST
	tree, err := o.astParser.Parse(ctx, lang, []byte(content))
	if err != nil {
		return nil, err
	}

	// Extract AST data for Rego
	astData := o.extractASTData(tree, content, filePath)

	// Run rule engine
	ruleFindings, err := o.ruleEngine.Evaluate(ctx, astData)
	if err != nil {
		return nil, err
	}

	// Semantic Review Layer (LLM)
	// We only send critical/high findings to the LLM to save costs and reduce noise
	var finalFindings []*analysis.Finding
	for _, rf := range ruleFindings {
		if rf.Severity == "critical" || rf.Severity == "high" {
			review, err := o.llmClient.ReviewFinding(ctx, llm.ReviewRequest{
				FilePath:    rf.FilePath,
				CodeSnippet: rf.CodeSnippet,
				Finding: llm.Finding{
					RuleID:   rf.RuleID,
					Message:  rf.Message,
					Severity: rf.Severity,
				},
			})
			
			if err == nil && review.IsTruePositive {
				// Update finding with AI reasoning and suggested fix
				rf.Message = fmt.Sprintf("%s\n\nAI Reasoning: %s", rf.Message, review.Reasoning)
				// We store the fix in metadata or a separate field (adding to proto in next step)
				
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
			} else if err == nil && !review.IsTruePositive {
				// Filter out False Positives identified by LLM
				continue
			} else {
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

// extractASTData extracts structured data from tree-sitter tree for Rego input
func (o *Orchestrator) extractASTData(tree *tree_sitter.Tree, source, filePath string) map[string]interface{} {
	// This would walk the tree-sitter AST and extract structured data
	// For now, return basic structure
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
			"number": i + 1,
			"content": line,
		})
	}
	return lines
}

// publishResults publishes analysis results to NATS
func (o *Orchestrator) publishResults(ctx context.Context, result *analysis.AnalysisResult) error {
	data, err := protojson.Marshal(result)
	if err != nil {
		return err
	}

	_, err = o.js.Publish(ctx, "analysis.results", data)
	return err
}