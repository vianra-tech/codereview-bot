package rules

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/open-policy-agent/opa/v1/rego"
)

// Embedded policy modules. Queries in RuleSet reference these via data.<package>.<rule>.
//
//go:embed policies/security.rego
var securityPolicy string

//go:embed policies/style.rego
var stylePolicy string

// Engine evaluates Rego policies against input data
type Engine struct {
	queries map[string]*rego.PreparedEvalQuery
	modules []string
}

// RuleSet represents a collection of rules
type RuleSet struct {
	ID          string
	Name        string
	Description string
	Language    string
	Severity    string
	Query       string // Rego query to evaluate, e.g. data.security.sql_injection
}

// Finding represents a rule violation
type Finding struct {
	RuleID      string
	Severity    string
	Message     string
	FilePath    string
	StartLine   int
	EndLine     int
	StartColumn int
	EndColumn   int
	CodeSnippet string
	Metadata    map[string]interface{}
}

// NewEngine creates a new rule engine
func NewEngine() *Engine {
	return &Engine{
		queries: make(map[string]*rego.PreparedEvalQuery),
		modules: []string{securityPolicy, stylePolicy},
	}
}

// LoadRuleSet compiles and loads a rule set
func (e *Engine) LoadRuleSet(ctx context.Context, rs RuleSet) error {
	opts := []func(*rego.Rego){rego.Query(rs.Query)}
	for i, module := range e.modules {
		opts = append(opts, rego.Module(fmt.Sprintf("policy_%d.rego", i), module))
	}

	prepared, err := rego.New(opts...).PrepareForEval(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare rule %s: %w", rs.ID, err)
	}

	e.queries[rs.ID] = &prepared
	return nil
}

// LoadRuleSets loads multiple rule sets
func (e *Engine) LoadRuleSets(ctx context.Context, ruleSets []RuleSet) error {
	for _, rs := range ruleSets {
		if err := e.LoadRuleSet(ctx, rs); err != nil {
			return err
		}
	}
	return nil
}

// Evaluate runs all loaded rules against the input
func (e *Engine) Evaluate(ctx context.Context, input interface{}) ([]Finding, error) {
	var allFindings []Finding

	for ruleID, query := range e.queries {
		results, err := query.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			return nil, fmt.Errorf("rule %s evaluation failed: %w", ruleID, err)
		}

		findings := e.extractFindings(ruleID, results)
		allFindings = append(allFindings, findings...)
	}

	return allFindings, nil
}

// EvaluateRule runs a specific rule against the input
func (e *Engine) EvaluateRule(ctx context.Context, ruleID string, input interface{}) ([]Finding, error) {
	query, ok := e.queries[ruleID]
	if !ok {
		return nil, fmt.Errorf("rule %s not loaded", ruleID)
	}

	results, err := query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return nil, fmt.Errorf("rule %s evaluation failed: %w", ruleID, err)
	}

	return e.extractFindings(ruleID, results), nil
}

// extractFindings converts OPA results to Finding structs
func (e *Engine) extractFindings(ruleID string, results rego.ResultSet) []Finding {
	var findings []Finding

	for _, result := range results {
		for _, expr := range result.Expressions {
			if expr.Value == nil {
				continue
			}

			// Handle array of findings
			if findingsArray, ok := expr.Value.([]interface{}); ok {
				for _, f := range findingsArray {
					if finding, ok := f.(map[string]interface{}); ok {
						findings = append(findings, Finding{
							RuleID:      ruleID,
							Severity:    getString(finding, "severity"),
							Message:     getString(finding, "message"),
							FilePath:    getString(finding, "file_path"),
							StartLine:   getInt(finding, "start_line"),
							EndLine:     getInt(finding, "end_line"),
							StartColumn: getInt(finding, "start_column"),
							EndColumn:   getInt(finding, "end_column"),
							CodeSnippet: getString(finding, "code_snippet"),
							Metadata:    finding,
						})
					}
				}
			}
		}
	}

	return findings
}

// getString safely extracts a string from a map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getInt safely extracts an int from a map
func getInt(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	}
	return 0
}

// BuiltinRuleSets returns the default rule sets
func BuiltinRuleSets() []RuleSet {
	return []RuleSet{
		{
			ID:          "security.sql_injection",
			Name:        "SQL Injection",
			Description: "Detects potential SQL injection vulnerabilities",
			Language:    "python",
			Severity:    "critical",
			Query:       `data.security.sql_injection`,
		},
		{
			ID:          "security.command_injection",
			Name:        "Command Injection",
			Description: "Detects potential command injection vulnerabilities",
			Language:    "python",
			Severity:    "critical",
			Query:       `data.security.command_injection`,
		},
		{
			ID:          "security.path_traversal",
			Name:        "Path Traversal",
			Description: "Detects potential path traversal vulnerabilities",
			Language:    "python",
			Severity:    "high",
			Query:       `data.security.path_traversal`,
		},
		{
			ID:          "security.hardcoded_secret",
			Name:        "Hardcoded Secret",
			Description: "Detects hardcoded API keys, passwords, tokens",
			Language:    "python",
			Severity:    "critical",
			Query:       `data.security.hardcoded_secret`,
		},
		{
			ID:          "security.xss",
			Name:        "Cross-Site Scripting",
			Description: "Detects potential XSS vulnerabilities",
			Language:    "python",
			Severity:    "high",
			Query:       `data.security.xss`,
		},
		{
			ID:          "style.trailing_whitespace",
			Name:        "Trailing Whitespace",
			Description: "Detects trailing whitespace on lines",
			Language:    "multi",
			Severity:    "info",
			Query:       `data.style.trailing_whitespace`,
		},
	}
}
