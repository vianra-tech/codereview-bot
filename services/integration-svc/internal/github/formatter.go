package github

import (
	"fmt"
	"strings"

	"github.com/google/go-github/v60/github"
	analysis "github.com/vianra/codereview/gen/go/proto/analysis/v1"
)

// CommentFormatter formats findings into GitHub review comments
type CommentFormatter struct{}

// NewCommentFormatter creates a new comment formatter
func NewCommentFormatter() *CommentFormatter {
	return &CommentFormatter{}
}

// FormatFinding formats a single finding as a review comment body
func (f *CommentFormatter) FormatFinding(finding *analysis.Finding) string {
	var b strings.Builder

	// Severity emoji
	emoji := severityEmoji(finding.Severity)

	b.WriteString(fmt.Sprintf("%s **%s**\n\n", emoji, strings.Title(finding.Severity)))
	b.WriteString(fmt.Sprintf("%s\n\n", finding.Message))

	if finding.CodeSnippet != "" {
		b.WriteString("```")
		b.WriteString(guessLanguage(finding.FilePath))
		b.WriteString("\n")
		b.WriteString(finding.CodeSnippet)
		b.WriteString("\n```\n\n")
	}

	b.WriteString(fmt.Sprintf("*File: `%s` (lines %d-%d)*", finding.FilePath, finding.StartLine, finding.EndLine))

	return b.String()
}

// FormatCheckRunOutput formats findings for a check run output
func (f *CommentFormatter) FormatCheckRunOutput(findings []*analysis.Finding) *github.CheckRunOutput {
	var b strings.Builder

	if len(findings) == 0 {
		b.WriteString("✅ **No issues found** - Great job!")
		return &github.CheckRunOutput{
			Title:   github.String("CodeReview.ai Analysis"),
			Summary: github.String(b.String()),
		}
	}

	// Group by severity
	bySeverity := make(map[string][]*analysis.Finding)
	for _, finding := range findings {
		bySeverity[finding.Severity] = append(bySeverity[finding.Severity], finding)
	}

	b.WriteString("## CodeReview.ai Analysis Results\n\n")

	severities := []string{"critical", "high", "medium", "low", "info"}
	for _, sev := range severities {
		if findings, ok := bySeverity[sev]; ok {
			emoji := severityEmoji(sev)
			b.WriteString(fmt.Sprintf("### %s %s (%d)\n\n", emoji, strings.Title(sev), len(findings)))

			for _, finding := range findings {
				b.WriteString(fmt.Sprintf("- **%s**: %s (`%s:%d`)\n", finding.RuleId, finding.Message, finding.FilePath, finding.StartLine))
			}
			b.WriteString("\n")
		}
	}

	// conclusion is derived by FormatCheckRunConclusion at the call site.
	return &github.CheckRunOutput{
		Title:       github.String("CodeReview.ai Analysis"),
		Summary:     github.String(b.String()),
		Text:        github.String(b.String()),
		Annotations: f.createAnnotations(findings),
	}
}

// createAnnotations creates GitHub check run annotations for inline display
func (f *CommentFormatter) createAnnotations(findings []*analysis.Finding) []*github.CheckRunAnnotation {
	annotations := make([]*github.CheckRunAnnotation, 0, len(findings))

	for _, finding := range findings {
		level := "notice"
		switch finding.Severity {
		case "critical", "high":
			level = "failure"
		case "medium":
			level = "warning"
		case "low", "info":
			level = "notice"
		}

		annotations = append(annotations, &github.CheckRunAnnotation{
			Path:            github.String(finding.FilePath),
			StartLine:       github.Int(int(finding.StartLine)),
			EndLine:         github.Int(int(finding.EndLine)),
			StartColumn:     github.Int(int(finding.StartColumn)),
			EndColumn:       github.Int(int(finding.EndColumn)),
			AnnotationLevel: github.String(level),
			Message:         github.String(fmt.Sprintf("[%s] %s", finding.RuleId, finding.Message)),
			Title:           github.String(fmt.Sprintf("CodeReview.ai: %s", finding.RuleId)),
		})
	}

	// GitHub limits annotations to 50 per request
	if len(annotations) > 50 {
		annotations = annotations[:50]
	}

	return annotations
}

// CreateReviewComments creates review comments from findings
func (f *CommentFormatter) CreateReviewComments(findings []*analysis.Finding, commitSHA string) []*github.DraftReviewComment {
	comments := make([]*github.DraftReviewComment, 0, len(findings))

	for _, finding := range findings {
		// Only create inline comments for actionable findings
		if finding.Severity == "info" || finding.Severity == "low" {
			continue
		}

		comments = append(comments, &github.DraftReviewComment{
			Path: github.String(finding.FilePath),
			Line: github.Int(int(finding.EndLine)),
			Body: github.String(f.FormatFinding(finding)),
		})
	}

	return comments
}

// FormatCheckRunConclusion formats the final check run conclusion
func (f *CommentFormatter) FormatCheckRunConclusion(findings []*analysis.Finding) (conclusion, summary string) {
	if len(findings) == 0 {
		return "success", "✅ No issues found"
	}

	hasCritical := false
	hasHigh := false
	hasMedium := false

	for _, finding := range findings {
		switch finding.Severity {
		case "critical":
			hasCritical = true
		case "high":
			hasHigh = true
		case "medium":
			hasMedium = true
		}
	}

	if hasCritical {
		return "failure", fmt.Sprintf("🔴 %d critical issue(s) found - must fix before merge", countSeverity(findings, "critical"))
	}
	if hasHigh {
		return "failure", fmt.Sprintf("🟠 %d high severity issue(s) found - review required", countSeverity(findings, "high"))
	}
	if hasMedium {
		return "failure", fmt.Sprintf("🟡 %d medium issue(s) found - consider fixing", countSeverity(findings, "medium"))
	}

	return "success", fmt.Sprintf("🟢 %d low/info issues found", countSeverity(findings, "low")+countSeverity(findings, "info"))
}

func countSeverity(findings []*analysis.Finding, severity string) int {
	count := 0
	for _, f := range findings {
		if f.Severity == severity {
			count++
		}
	}
	return count
}

func severityEmoji(severity string) string {
	switch severity {
	case "critical":
		return "🔴"
	case "high":
		return "🟠"
	case "medium":
		return "🟡"
	case "low":
		return "🟢"
	case "info":
		return "ℹ️"
	default:
		return "⚪"
	}
}

func guessLanguage(filePath string) string {
	ext := strings.ToLower(filePath[strings.LastIndex(filePath, ".")+1:])
	switch ext {
	case "py":
		return "python"
	case "go":
		return "go"
	case "js", "jsx":
		return "javascript"
	case "ts", "tsx":
		return "typescript"
	case "java":
		return "java"
	case "cpp", "cc", "cxx":
		return "cpp"
	case "c":
		return "c"
	case "rs":
		return "rust"
	default:
		return ""
	}
}
