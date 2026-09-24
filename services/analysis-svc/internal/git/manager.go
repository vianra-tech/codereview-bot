package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

// Manager handles git operations for repository analysis
type Manager struct {
	baseDir string
}

// NewManager creates a new Git manager
func NewManager(baseDir string) *Manager {
	return &Manager{baseDir: baseDir}
}

// CloneRepo clones the repository and checks out the target commit.
// The clone is full (not shallow) so that arbitrary commit SHAs can be
// reached and compared against.
func (m *Manager) CloneRepo(ctx context.Context, repoURL, commitSHA string) (string, error) {
	repoPath := filepath.Join(m.baseDir, commitSHA)
	if _, err := os.Stat(repoPath); err == nil {
		return repoPath, nil
	}

	_, err := git.PlainCloneContext(ctx, repoPath, false, &git.CloneOptions{
		URL:      repoURL,
		Progress: nil,
	})
	if err != nil {
		return "", fmt.Errorf("clone failed: %w", err)
	}

	// Checkout specific commit if necessary
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", err
	}

	w, err := repo.Worktree()
	if err != nil {
		return "", err
	}

	err = w.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(commitSHA),
	})
	if err != nil {
		return "", fmt.Errorf("checkout failed: %w", err)
	}

	return repoPath, nil
}

// GetChangedFiles returns the list of files changed between base and head SHAs
func (m *Manager) GetChangedFiles(ctx context.Context, repoPath, baseSHA, headSHA string) ([]string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}

	baseCommit, err := repo.CommitObject(plumbing.NewHash(baseSHA))
	if err != nil {
		return nil, err
	}

	headCommit, err := repo.CommitObject(plumbing.NewHash(headSHA))
	if err != nil {
		return nil, err
	}

	baseTree, err := baseCommit.Tree()
	if err != nil {
		return nil, err
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return nil, err
	}

	changes, err := baseTree.Diff(headTree)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			return nil, err
		}
		// Only track added or modified files
		if action == merkletrie.Insert || action == merkletrie.Modify {
			files = append(files, change.To.Name)
		}
	}

	return files, nil
}

// GetFileContent retrieves the content of a file at a specific commit.
// If commitSHA is empty, the file is read from the working tree.
func (m *Manager) GetFileContent(ctx context.Context, repoPath, commitSHA, filePath string) (string, error) {
	if commitSHA == "" {
		absPath := filepath.Join(repoPath, filePath)
		content, err := os.ReadFile(absPath)
		if err != nil {
			return "", err
		}
		return string(content), nil
	}

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", err
	}

	commit, err := repo.CommitObject(plumbing.NewHash(commitSHA))
	if err != nil {
		return "", err
	}

	tree, err := commit.Tree()
	if err != nil {
		return "", err
	}

	file, err := tree.File(filePath)
	if err != nil {
		return "", errors.New("file not found in commit")
	}

	content, err := file.Contents()
	if err != nil {
		return "", err
	}

	return content, nil
}
