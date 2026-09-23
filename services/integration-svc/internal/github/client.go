package github

import (
	"context"
	"fmt"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v60/github"
	"go.uber.org/zap"
)

// Client wraps GitHub API operations
type Client struct {
	appClient *github.Client
	logger    *zap.Logger
	appID     int64
}

// NewClient creates a new GitHub client
func NewClient(appID int64, privateKey []byte, logger *zap.Logger) (*Client, error) {
	itr, err := ghinstallation.NewKeyFromFile(
		github.DefaultClient,
		appID,
		0, // installation ID 0 means app-level
		string(privateKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub app transport: %w", err)
	}

	appClient := github.NewClient(itr)

	return &Client{
		appClient: appClient,
		logger:    logger,
		appID:     appID,
	}, nil
}

// GetInstallationClient returns a client for a specific installation
func (c *Client) GetInstallationClient(ctx context.Context, installationID int64) (*github.Client, error) {
	itr, err := ghinstallation.NewKeyFromFile(
		github.DefaultClient,
		c.appID,
		installationID,
		"", // private key handled by app client
	)
	if err != nil {
		return nil, err
	}
	return github.NewClient(itr), nil
}

// CreateCheckRun creates a check run for a commit
func (c *Client) CreateCheckRun(ctx context.Context, installationID int64, repoOwner, repoName, commitSHA, name, status, conclusion string, output *github.CheckRunOutput) (*github.CheckRun, error) {
	client, err := c.GetInstallationClient(ctx, installationID)
	if err != nil {
		return nil, err
	}

	checkRun := github.CreateCheckRunOptions{
		Name:        name,
		HeadSHA:     commitSHA,
		Status:      github.String(status),
		StartedAt:   &github.Timestamp{Time: time.Now()},
		Output:      output,
	}

	if conclusion != "" {
		checkRun.Conclusion = github.String(conclusion)
		checkRun.CompletedAt = &github.Timestamp{Time: time.Now()}
	}

	run, _, err := client.Checks.CreateCheckRun(ctx, repoOwner, repoName, checkRun)
	return run, err
}

// UpdateCheckRun updates an existing check run
func (c *Client) UpdateCheckRun(ctx context.Context, installationID int64, repoOwner, repoName string, checkRunID int64, status, conclusion string, output *github.CheckRunOutput) (*github.CheckRun, error) {
	client, err := c.GetInstallationClient(ctx, installationID)
	if err != nil {
		return nil, err
	}

	checkRun := github.UpdateCheckRunOptions{
		Name:       "CodeReview.ai",
		Status:     github.String(status),
		Output:     output,
	}

	if conclusion != "" {
		checkRun.Conclusion = github.String(conclusion)
		checkRun.CompletedAt = &github.Timestamp{Time: time.Now()}
	}

	run, _, err := client.Checks.UpdateCheckRun(ctx, repoOwner, repoName, checkRunID, checkRun)
	return run, err
}

// CreateReviewComment creates an inline review comment on a PR
func (c *Client) CreateReviewComment(ctx context.Context, installationID int64, repoOwner, repoName string, prNumber int, commitSHA, path string, line int, body string) error {
	client, err := c.GetInstallationClient(ctx, installationID)
	if err != nil {
		return err
	}

	comment := github.ReviewComment{
		Body:     github.String(body),
		Path:     github.String(path),
		Line:     github.Int(line),
		CommitID: github.String(commitSHA),
	}

	_, _, err = client.PullRequests.CreateComment(ctx, repoOwner, repoName, prNumber, &comment)
	return err
}

// CreateReview creates a review with multiple comments
func (c *Client) CreateReview(ctx context.Context, installationID int64, repoOwner, repoName string, prNumber int, commitSHA, body, event string, comments []*github.DraftReviewComment) error {
	client, err := c.GetInstallationClient(ctx, installationID)
	if err != nil {
		return err
	}

	review := github.ReviewRequest{
		Body:     github.String(body),
		CommitID: github.String(commitSHA),
		Event:    github.String(event), // COMMENT, APPROVE, REQUEST_CHANGES
		Comments: comments,
	}

	_, _, err = client.PullRequests.CreateReview(ctx, repoOwner, repoName, prNumber, &review)
	return err
}

// GetInstallation gets installation ID for a repo
func (c *Client) GetInstallation(ctx context.Context, repoOwner, repoName string) (int64, error) {
	installation, _, err := c.appClient.Apps.FindRepositoryInstallation(ctx, repoOwner, repoName)
	if err != nil {
		return 0, err
	}
	return installation.GetID(), nil
}

// GetRepo gets repository information
func (c *Client) GetRepo(ctx context.Context, installationID int64, repoOwner, repoName string) (*github.Repository, error) {
	client, err := c.GetInstallationClient(ctx, installationID)
	if err != nil {
		return nil, err
	}
	repo, _, err := client.Repositories.Get(ctx, repoOwner, repoName)
	return repo, err
}