package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const gitHubURL = "https://github.com/%s/%s.git"

// GitHubController implements the Repository Controller for GitHub. It checks
// out the pull request head ref into the repository's working directory and
// prepares it for the Editor Launcher.
type GitHubController struct {
	git       string
	remoteURL func(PullRequest) string

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// NewGitHubController returns a GitHub Controller backed by the git command.
func NewGitHubController() *GitHubController {
	return &GitHubController{
		git: "git",
		remoteURL: func(pullRequest PullRequest) string {
			return fmt.Sprintf(gitHubURL, pullRequest.Owner, pullRequest.Name)
		},
		locks: make(map[string]*sync.Mutex),
	}
}

// CheckoutPRAndPrepareWorkingDirectory fetches the pull request head, checks
// it out on the pull request's local branch, and returns the prepared working
// directory. A repository keeps one working directory, so requesting another
// pull request of the same repository switches that directory's branch.
func (c *GitHubController) CheckoutPRAndPrepareWorkingDirectory(ctx context.Context, pullRequest PullRequest, baseDirectory string) (string, error) {
	if err := pullRequest.Validate(); err != nil {
		return "", err
	}
	if err := validateBaseDirectory(baseDirectory); err != nil {
		return "", err
	}

	workingDirectory := pullRequest.WorkingDirectory(baseDirectory)
	lock := c.lockFor(workingDirectory)
	lock.Lock()
	defer lock.Unlock()

	if err := os.MkdirAll(workingDirectory, 0o755); err != nil {
		return "", fmt.Errorf("prepare working directory %s: %w", workingDirectory, err)
	}

	remoteURL := c.remoteURL(pullRequest)
	initialized, err := hasGitDirectory(workingDirectory)
	if err != nil {
		return "", err
	}
	if initialized {
		origin, err := c.runGit(ctx, workingDirectory, "remote", "get-url", "origin")
		if err != nil {
			return "", err
		}
		if origin != remoteURL {
			return "", fmt.Errorf("working directory %s tracks %s instead of %s", workingDirectory, origin, remoteURL)
		}
	} else {
		if _, err := c.runGit(ctx, workingDirectory, "init", "--quiet"); err != nil {
			return "", err
		}
		if _, err := c.runGit(ctx, workingDirectory, "remote", "add", "origin", remoteURL); err != nil {
			return "", err
		}
	}

	headRef := fmt.Sprintf("pull/%d/head", pullRequest.Number)
	if _, err := c.runGit(ctx, workingDirectory, "fetch", "--quiet", "--depth=1", "origin", headRef); err != nil {
		return "", err
	}
	branch := fmt.Sprintf("plot/pr-%d", pullRequest.Number)
	if _, err := c.runGit(ctx, workingDirectory, "checkout", "--quiet", "-B", branch, "FETCH_HEAD"); err != nil {
		return "", err
	}
	return workingDirectory, nil
}

func (c *GitHubController) runGit(ctx context.Context, directory string, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, c.git, arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf("git %s in %s: %w: %s", strings.Join(arguments, " "), directory, err, detail)
		}
		return "", fmt.Errorf("git %s in %s: %w", strings.Join(arguments, " "), directory, err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (c *GitHubController) lockFor(key string) *sync.Mutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	lock, ok := c.locks[key]
	if !ok {
		lock = &sync.Mutex{}
		c.locks[key] = lock
	}
	return lock
}

func hasGitDirectory(workingDirectory string) (bool, error) {
	_, err := os.Stat(filepath.Join(workingDirectory, ".git"))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect working directory %s: %w", workingDirectory, err)
	}
	return true, nil
}

func validateBaseDirectory(baseDirectory string) error {
	trimmed := strings.TrimSpace(baseDirectory)
	if trimmed == "" {
		return errors.New("base directory is required")
	}
	if !filepath.IsAbs(trimmed) {
		return fmt.Errorf("base directory %q must be absolute", baseDirectory)
	}
	info, err := os.Stat(trimmed)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect base directory %s: %w", trimmed, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("base directory %s is not a directory", trimmed)
	}
	return nil
}
