package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Request defines the structure for the NVIDIA NIM API request
type Request struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens,omitempty"`
	Stream    bool      `json:"stream"`
	Temperature float64 `json:"temperature,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response defines the structure for the NVIDIA NIM API response
type Response struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}

// ReviewRequest defines the a specific request for code review
type ReviewRequest struct {
	FilePath    string `json:"file_path"`
	CodeSnippet string `json:"code_snippet"`
	Finding     Finding `json:"finding"`
}

type Finding struct {
	RuleID   string `json:"rule_id"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// ReviewResponse defines the AI's feedback on a finding
type ReviewResponse struct {
	IsTruePositive bool   `json:"is_true_positive"`
	Reasoning     string `json:"reasoning"`
	SuggestedFix  string `json:"suggested_fix"`
	NewSeverity   string `json:"new_severity"`
}

// Client handles communication with the NVIDIA NIM API
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewClient creates a new LLM client
func NewClient(baseURL, apiKey, model string, logger *zap.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// ReviewFinding asks the LLM to verify a finding and suggest a fix
func (c *Client) ReviewFinding(ctx context.Context, req ReviewRequest) (*ReviewResponse, error) {
	systemPrompt := `You are an expert security researcher and senior software engineer. 
Your task is to analyze a specific code finding and determine if it is a True Positive or a False Positive.
You will be provided with the file path, the code snippet, and the rule that was triggered.

Your response MUST be a valid JSON object with the following keys:
- is_true_positive (boolean): true if the finding is a real issue, false otherwise.
- reasoning (string): A concise explanation of why it is or isn't an issue.
- suggested_fix (string): If true positive, provide the corrected code snippet. If false positive, leave empty.
- new_severity (string): Adjusted severity (critical, high, medium, low, info) if the context changes it.

Strictly output JSON only.`

	userPrompt := fmt.Sprintf(
		"File: %s\nCode: %s\nRule: %s\nMessage: %s\nSeverity: %s",
		req.FilePath, req.CodeSnippet, req.Finding.RuleID, req.Finding.Message, req.Finding.Severity,
	)

	payload := Request{
		Model: c.model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 1024,
		Stream:    false,
		Temperature: 0.2,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	reqHttp, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	reqHttp.Header.Set("Authorization", "Bearer "+c.apiKey)
	reqHttp.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(reqHttp)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp Response
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("API returned no choices")
	}

	var review ReviewResponse
	if err := json.Unmarshal([]byte(apiResp.Choices[0].Message.Content), &review); err != nil {
		c.logger.Warn("LLM returned non-JSON response", zap.String("content", apiResp.Choices[0].Message.Content))
		return nil, fmt.Errorf("failed to parse LLM response as JSON: %w", err)
	}

	return &review, nil
}