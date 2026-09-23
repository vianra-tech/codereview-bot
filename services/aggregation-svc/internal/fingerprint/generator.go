package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/vianra/codereview/gen/go/proto/analysis/v1"
)

// Generator creates unique fingerprints for findings
type Generator struct{}

// NewGenerator creates a new fingerprint generator
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate creates a deterministic fingerprint for a finding
// Uses: file_path + rule_id + normalized_location + normalized_snippet
func (g *Generator) Generate(finding *analysis.Finding) string {
	// Normalize the code snippet to ignore whitespace differences
	normalizedSnippet := normalizeSnippet(finding.CodeSnippet)
	
	// Create a composite key
	composite := fmt.Sprintf(
		"%s|%s|%d|%d|%s",
		finding.FilePath,
		finding.RuleId,
		finding.StartLine,
		finding.EndLine,
		normalizedSnippet,
	)
	
	hash := sha256.Sum256([]byte(composite))
	return hex.EncodeToString(hash[:])
}

// GenerateRunFingerprint creates a fingerprint for an entire analysis run
func (g *Generator) GenerateRunFingerprint(runID, repoName, commitSHA string) string {
	composite := fmt.Sprintf("%s|%s|%s", runID, repoName, commitSHA)
	hash := sha256.Sum256([]byte(composite))
	return hex.EncodeToString(hash[:])
}

// normalizeSnippet removes extra whitespace for consistent fingerprinting
func normalizeSnippet(s string) string {
	// Simple normalization - in production, use AST-based normalization
	result := ""
	inWhitespace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !inWhitespace {
				result += " "
				inWhitespace = true
			}
		} else {
			result += string(r)
			inWhitespace = false
		}
	}
	return result
}

// Matches checks if two findings are the same issue
func (g *Generator) Matches(f1, f2 *analysis.Finding) bool {
	return g.Generate(f1) == g.Generate(f2)
}