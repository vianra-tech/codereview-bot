package sarif

import (
	"fmt"

	"github.com/vianra/codereview/gen/go/proto/analysis/v1"
)

// SARIFConverter converts findings to SARIF format
type SARIFConverter struct{}

// NewSARIFConverter creates a new SARIF converter
func NewSARIFConverter() *SARIFConverter {
	return &SARIFConverter{}
}

// Convert converts findings to SARIF format
func (c *SARIFConverter) Convert(findings []*analysis.Finding, runInfo RunInfo) (*SARIFLog, error) {
	results := make([]SARIFResult, 0, len(findings))
	
	for _, finding := range findings {
		result := c.convertFinding(finding)
		results = append(results, result)
	}
	
	// Create unique rule definitions
	rules := c.extractRules(findings)
	
	log := &SARIFLog{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []SARIFRun{
			{
				Tool: SARIFTool{
					Driver: SARIFDriver{
						Name:            "CodeReview.ai",
						Version:         "0.1.0",
						InformationURI:  "https://codereview.ai",
						FullDescription: &MultiformatMessageString{Text: "AI-powered code review platform"},
						Rules:           rules,
					},
				},
				Results:       results,
				ColumnKind:    "utf16CodeUnits",
				AutomationDetails: &AutomationDetails{
					ID: runInfo.RunID,
				},
			},
		},
	}
	
	return log, nil
}

// RunInfo contains metadata about the analysis run
type RunInfo struct {
	RunID     string
	RepoName  string
	CommitSHA string
	Timestamp string
}

// convertFinding converts a single finding to SARIF result
func (c *SARIFConverter) convertFinding(finding *analysis.Finding) SARIFResult {
	level := severityToSARIFLevel(finding.Severity)
	
	return SARIFResult{
		RuleID:    finding.RuleId,
		Level:     level,
		Message:   &MultiformatMessageString{Text: finding.Message},
		Locations: []SARIFLocation{
			{
				PhysicalLocation: &PhysicalLocation{
					ArtifactLocation: &ArtifactLocation{
						URI: finding.FilePath,
					},
					Region: &Region{
						StartLine:   int(finding.StartLine),
						EndLine:     int(finding.EndLine),
						StartColumn: int(finding.StartColumn),
						EndColumn:   int(finding.EndColumn),
					},
				},
			},
		},
		PartialFingerprints: map[string]string{
			"codereview/primaryLocationLineHash": "placeholder",
		},
		Properties: &ResultProperties{
			Severity: finding.Severity,
			Tool:     "codereview.ai",
		},
	}
}

// extractRules extracts unique rule definitions from findings
func (c *SARIFConverter) extractRules(findings []*analysis.Finding) []SARIFRule {
	ruleMap := make(map[string]SARIFRule)
	
	for _, finding := range findings {
		if _, exists := ruleMap[finding.RuleId]; !exists {
			ruleMap[finding.RuleId] = SARIFRule{
				ID:   finding.RuleId,
				Name: finding.RuleId,
				ShortDescription: &MultiformatMessageString{
					Text: fmt.Sprintf("Rule: %s", finding.RuleId),
				},
				FullDescription: &MultiformatMessageString{
					Text: finding.Message,
				},
				DefaultConfiguration: &DefaultConfiguration{
					Level: severityToSARIFLevel(finding.Severity),
				},
				Properties: &RuleProperties{
					Category: "security",
					Tags:     []string{finding.Severity},
				},
			}
		}
	}
	
	rules := make([]SARIFRule, 0, len(ruleMap))
	for _, rule := range ruleMap {
		rules = append(rules, rule)
	}
	
	return rules
}

func severityToSARIFLevel(severity string) string {
	switch severity {
	case "critical":
		return "error"
	case "high":
		return "error"
	case "medium":
		return "warning"
	case "low":
		return "note"
	case "info":
		return "none"
	default:
		return "note"
	}
}

// SARIF structs (subset of SARIF 2.1.0)

type SARIFLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []SARIFRun `json:"runs"`
}

type SARIFRun struct {
	Tool             SARIFTool            `json:"tool"`
	Results          []SARIFResult        `json:"results"`
	ColumnKind       string               `json:"columnKind"`
	AutomationDetails *AutomationDetails  `json:"automationDetails,omitempty"`
}

type SARIFTool struct {
	Driver SARIFDriver `json:"driver"`
}

type SARIFDriver struct {
	Name            string               `json:"name"`
	Version         string               `json:"version"`
	InformationURI  string               `json:"informationUri"`
	FullDescription *MultiformatMessageString `json:"fullDescription,omitempty"`
	Rules           []SARIFRule          `json:"rules,omitempty"`
}

type SARIFRule struct {
	ID                   string                    `json:"id"`
	Name                 string                    `json:"name"`
	ShortDescription     *MultiformatMessageString `json:"shortDescription,omitempty"`
	FullDescription      *MultiformatMessageString `json:"fullDescription,omitempty"`
	DefaultConfiguration *DefaultConfiguration     `json:"defaultConfiguration,omitempty"`
	Properties           *RuleProperties           `json:"properties,omitempty"`
}

type DefaultConfiguration struct {
	Level string `json:"level"`
}

type RuleProperties struct {
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
}

type SARIFResult struct {
	RuleID              string                    `json:"ruleId"`
	Level               string                    `json:"level"`
	Message             *MultiformatMessageString `json:"message"`
	Locations           []SARIFLocation           `json:"locations,omitempty"`
	PartialFingerprints map[string]string         `json:"partialFingerprints,omitempty"`
	Properties          *ResultProperties         `json:"properties,omitempty"`
}

type MultiformatMessageString struct {
	Text string `json:"text"`
}

type SARIFLocation struct {
	PhysicalLocation *PhysicalLocation `json:"physicalLocation,omitempty"`
}

type PhysicalLocation struct {
	ArtifactLocation *ArtifactLocation `json:"artifactLocation"`
	Region           *Region           `json:"region,omitempty"`
}

type ArtifactLocation struct {
	URI string `json:"uri"`
}

type Region struct {
	StartLine   int `json:"startLine"`
	EndLine     int `json:"endLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
	EndColumn   int `json:"endColumn,omitempty"`
}

type AutomationDetails struct {
	ID string `json:"id"`
}

type ResultProperties struct {
	Severity string `json:"severity"`
	Tool     string `json:"tool"`
}