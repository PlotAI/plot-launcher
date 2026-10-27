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
// Uncommitted changes found in the working directory are set aside and
// restored on the pull request, so an editor already editing the repository
// does not block the checkout.
func (c *GitHubController) CheckoutPRAndPrepareWorkingDirectory(ctx context.Context, pullRequest PullRequest, baseDirectory string) (Checkout, error) {
	if err := pullRequest.Validate(); err != nil {
		return Checkout{}, err
	}
	if err := validateBaseDirectory(baseDirectory); err != nil {
		return Checkout{}, err
	}

	workingDirectory := pullRequest.WorkingDirectory(baseDirectory)
	lock := c.lockFor(workingDirectory)
	lock.Lock()
	defer lock.Unlock()

	if err := os.MkdirAll(workingDirectory, 0o755); err != nil {
		return Checkout{}, fmt.Errorf("prepare working directory %s: %w", workingDirectory, err)
	}

	var notices []Notice
	remoteURL := c.remoteURL(pullRequest)
	initialized, err := hasGitDirectory(workingDirectory)
	if err != nil {
		return Checkout{}, err
	}
	if initialized {
		notice, err := c.reconcileRemote(ctx, workingDirectory, remoteURL, pullRequest)
		if err != nil {
			return Checkout{}, err
		}
		if notice != nil {
			notices = append(notices, *notice)
		}
	} else {
		if _, err := c.runGit(ctx, workingDirectory, "init", "--quiet"); err != nil {
			return Checkout{}, err
		}
		if _, err := c.runGit(ctx, workingDirectory, "remote", "add", "origin", remoteURL); err != nil {
			return Checkout{}, err
		}
	}

	pending, err := c.stashWorkingTree(ctx, workingDirectory, pullRequest)
	if err != nil {
		return Checkout{}, err
	}

	headRef := fmt.Sprintf("pull/%d/head", pullRequest.Number)
	if _, err := c.runGit(ctx, workingDirectory, "fetch", "--quiet", "--depth=1", "origin", headRef); err != nil {
		return Checkout{}, c.checkoutFailed(ctx, workingDirectory, pending, err)
	}
	commitNotice, err := c.preserveLocalCommits(ctx, workingDirectory, pullRequest, "FETCH_HEAD")
	if err != nil {
		return Checkout{}, c.checkoutFailed(ctx, workingDirectory, pending, err)
	}
	if commitNotice != nil {
		notices = append(notices, *commitNotice)
	}
	if _, err := c.runGit(ctx, workingDirectory, "checkout", "--quiet", "-B", pullRequestBranch(pullRequest), "FETCH_HEAD"); err != nil {
		return Checkout{}, c.checkoutFailed(ctx, workingDirectory, pending, err)
	}
	if err := c.recordCheckout(ctx, workingDirectory, pullRequest); err != nil {
		return Checkout{}, err
	}

	if notice := c.restoreStash(ctx, workingDirectory, pending); notice != nil {
		notices = append(notices, *notice)
	}
	return Checkout{WorkingDirectory: workingDirectory, Notices: notices}, nil
}

// reconcileRemote makes origin agree with the repository the pull request
// belongs to. A repository that was renamed or transferred redirects, so the
// pull request head resolves to the same commit at the new location; any other
// difference is reported instead of checking out another repository.
func (c *GitHubController) reconcileRemote(ctx context.Context, directory, remoteURL string, pullRequest PullRequest) (*Notice, error) {
	origin, err := c.runGit(ctx, directory, "remote", "get-url", "origin")
	if err != nil {
		return nil, err
	}
	if origin == remoteURL {
		return nil, nil
	}
	// Another spelling of the same repository, such as an SSH remote the user
	// added, needs no change.
	if sameRepository(origin, remoteURL) {
		return nil, nil
	}

	headRef := fmt.Sprintf("refs/pull/%d/head", pullRequest.Number)
	requested, err := c.headCommit(ctx, directory, remoteURL, headRef)
	if err != nil {
		return nil, &CheckoutError{
			Code:    CodeRemoteMismatch,
			Message: fmt.Sprintf("working directory %s tracks %s, and the repository of the pull request %s could not be read: %v", directory, origin, remoteURL, err),
			Remedy:  "Check the link. If the repository moved, remove the checkout and open the pull request again.",
			Command: fmt.Sprintf("rm -rf %s", shellQuote(directory)),
			Err:     err,
		}
	}
	tracked, err := c.headCommit(ctx, directory, origin, headRef)
	if err != nil || tracked != requested {
		return nil, &CheckoutError{
			Code:    CodeRemoteMismatch,
			Message: fmt.Sprintf("working directory %s tracks %s, which does not hold the pull request of %s", directory, origin, remoteURL),
			Remedy:  "Remove the checkout and open the pull request again.",
			Command: fmt.Sprintf("rm -rf %s", shellQuote(directory)),
			Err:     err,
		}
	}

	moved := movedRemote(origin, remoteURL)
	if _, err := c.runGit(ctx, directory, "remote", "set-url", "origin", moved); err != nil {
		return nil, err
	}
	return &Notice{
		Code:    NoticeRemoteUpdated,
		Message: fmt.Sprintf("origin of %s now points at %s, which replaced %s", directory, moved, origin),
	}, nil
}

// headCommit returns the commit a pull request head ref resolves to on a
// remote.
func (c *GitHubController) headCommit(ctx context.Context, directory, remote, headRef string) (string, error) {
	output, err := c.runGit(ctx, directory, "ls-remote", "--quiet", remote, headRef)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return "", fmt.Errorf("%s has no %s", remote, headRef)
	}
	return fields[0], nil
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
