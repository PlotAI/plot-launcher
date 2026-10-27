package repository

import (
	"context"
	"fmt"
	"strings"
)

// pendingChanges is a stash entry holding the uncommitted changes of a
// working directory while its branch is switched.
type pendingChanges struct {
	reference string
	message   string
}

// stashWorkingTree sets uncommitted changes, including untracked files, aside
// so that they cannot block the checkout. It returns nil when the working tree
// is clean.
func (c *GitHubController) stashWorkingTree(ctx context.Context, directory string, pullRequest PullRequest) (*pendingChanges, error) {
	status, err := c.runGit(ctx, directory, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	if status == "" {
		return nil, nil
	}

	// A repository without commits carries nothing to stash, and git refuses
	// to stash before the first commit.
	if _, err := c.runGit(ctx, directory, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return nil, nil
	}

	branch, err := c.runGit(ctx, directory, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, err
	}
	message := fmt.Sprintf("plot: changes on %s before opening %s#%d", branch, pullRequest.Repository(), pullRequest.Number)
	if _, err := c.runGit(ctx, directory, "stash", "push", "--quiet", "--include-untracked", "--message", message); err != nil {
		return nil, &CheckoutError{
			Code:    CodeWorkingTree,
			Message: fmt.Sprintf("could not set aside the uncommitted changes in %s: %v", directory, err),
			Remedy:  "Commit or stash the changes, then open the pull request again.",
			Command: fmt.Sprintf("git -C %s stash push --include-untracked", shellQuote(directory)),
			Err:     err,
		}
	}

	reference, err := c.runGit(ctx, directory, "rev-parse", "--verify", "refs/stash")
	if err != nil {
		return nil, err
	}
	return &pendingChanges{reference: reference, message: message}, nil
}

// restoreStash puts the changes set aside by stashWorkingTree back on the
// checked out pull request. Changes that conflict with the pull request stay
// stashed, and the returned notice names the entry that holds them.
func (c *GitHubController) restoreStash(ctx context.Context, directory string, pending *pendingChanges) *Notice {
	if pending == nil {
		return nil
	}
	// A stash entry is dropped only when it applies: popping a conflicting
	// entry leaves it in place for the user to recover.
	if err := c.popStash(ctx, directory, pending); err == nil {
		return nil
	}

	c.resetWorkingTree(ctx, directory)
	return &Notice{
		Code: NoticeStashKept,
		Message: fmt.Sprintf("the uncommitted changes do not apply to the pull request and stay in the stash entry %q",
			pending.message),
		Command: fmt.Sprintf("git -C %s stash pop", shellQuote(directory)),
	}
}

// checkoutFailed returns the failure to report when the branch switch failed,
// with the changes of a failed attempt put back where they came from.
func (c *GitHubController) checkoutFailed(ctx context.Context, directory string, pending *pendingChanges, cause error) error {
	if pending == nil {
		return cause
	}
	if err := c.popStash(ctx, directory, pending); err == nil {
		return cause
	}

	c.resetWorkingTree(ctx, directory)
	return &CheckoutError{
		Code:    CodeWorkingTree,
		Message: fmt.Sprintf("the checkout failed, and the uncommitted changes did not apply again, so they stay in the stash entry %q: %v", pending.message, cause),
		Remedy:  "Restore the changes, then open the pull request again.",
		Command: fmt.Sprintf("git -C %s stash pop", shellQuote(directory)),
		Err:     cause,
	}
}

// preserveLocalCommits moves commits off the branch the checkout resets, so
// work committed in the working directory survives the switch. Commits of a
// branch that is not being reset are left where they are.
func (c *GitHubController) preserveLocalCommits(ctx context.Context, directory string, pullRequest PullRequest, target string) (*Notice, error) {
	branch := pullRequestBranch(pullRequest)
	current, err := c.runGit(ctx, directory, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		// A repository without commits has nothing to preserve.
		return nil, nil
	}
	if current != branch && current != "HEAD" {
		return nil, nil
	}

	// The commit the launcher last placed on the branch separates local work
	// from the history it put there itself. Without a record, a shallow
	// checkout cannot tell the two apart, so the fetched head is the
	// conservative base.
	base, err := c.runGit(ctx, directory, "rev-parse", "--verify", "--quiet", pullRequestMarker(pullRequest))
	if err != nil {
		base = target
	}
	ahead, err := c.runGit(ctx, directory, "rev-list", "--count", base+"..HEAD")
	if err != nil {
		return nil, err
	}
	if ahead == "0" {
		return nil, nil
	}

	head, err := c.runGit(ctx, directory, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	saved, err := c.saveBranch(ctx, directory, branch+"-local", head)
	if err != nil {
		return nil, &CheckoutError{
			Code:    CodeLocalCommits,
			Message: fmt.Sprintf("could not keep the local commits of %s: %v", branch, err),
			Remedy:  "Move the commits to another branch, then open the pull request again.",
			Command: fmt.Sprintf("git -C %s branch %s-local", shellQuote(directory), branch),
			Err:     err,
		}
	}

	return &Notice{
		Code:    NoticeLocalCommitsKept,
		Message: fmt.Sprintf("kept the commits of %s on %s, because the pull request head replaced the branch", branch, saved),
	}, nil
}

// recordCheckout records the commit the pull request branch was set to, so the
// next checkout can tell local commits from the ones it placed there.
func (c *GitHubController) recordCheckout(ctx context.Context, directory string, pullRequest PullRequest) error {
	_, err := c.runGit(ctx, directory, "update-ref", pullRequestMarker(pullRequest), "HEAD")
	return err
}

// pullRequestBranch is the local branch of a pull request.
func pullRequestBranch(pullRequest PullRequest) string {
	return fmt.Sprintf("plot/pr-%d", pullRequest.Number)
}

// pullRequestMarker is the ref recording the commit the pull request branch
// was last set to.
func pullRequestMarker(pullRequest PullRequest) string {
	return fmt.Sprintf("refs/plot/pr-%d/head", pullRequest.Number)
}

// saveBranch creates branch at commit, or moves it forward when commit
// descends from it. A divergent branch is left alone and a numbered name is
// used instead, so no commit is ever dropped.
func (c *GitHubController) saveBranch(ctx context.Context, directory, branch, commit string) (string, error) {
	for attempt := 1; attempt <= 10; attempt++ {
		candidate := branch
		if attempt > 1 {
			candidate = fmt.Sprintf("%s-%d", branch, attempt)
		}
		existing, err := c.runGit(ctx, directory, "rev-parse", "--verify", "--quiet", "refs/heads/"+candidate)
		if err != nil {
			if _, err := c.runGit(ctx, directory, "branch", "--quiet", candidate, commit); err != nil {
				return "", err
			}
			return candidate, nil
		}
		if existing == commit {
			return candidate, nil
		}
		if _, err := c.runGit(ctx, directory, "merge-base", "--is-ancestor", candidate, commit); err == nil {
			if _, err := c.runGit(ctx, directory, "branch", "--quiet", "--force", candidate, commit); err != nil {
				return "", err
			}
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no free name for the local branch %s", branch)
}

// popStash restores a stash entry and drops it. A conflicting entry fails and
// stays in the stash.
func (c *GitHubController) popStash(ctx context.Context, directory string, pending *pendingChanges) error {
	selector, err := c.stashSelector(ctx, directory, pending.reference)
	if err != nil {
		return err
	}
	_, err = c.runGit(ctx, directory, "stash", "pop", "--quiet", selector)
	return err
}

// stashSelector returns the stash@{<index>} reference of a stash commit, which
// is the form git accepts when restoring an entry.
func (c *GitHubController) stashSelector(ctx context.Context, directory, reference string) (string, error) {
	entries, err := c.runGit(ctx, directory, "stash", "list", "--format=%H")
	if err != nil {
		return "", err
	}
	for index, entry := range strings.Fields(entries) {
		if entry == reference {
			return fmt.Sprintf("stash@{%d}", index), nil
		}
	}
	return "", fmt.Errorf("stash entry %s is gone", reference)
}

// resetWorkingTree discards a partially applied stash. The entry it came from
// still holds every file it touched.
func (c *GitHubController) resetWorkingTree(ctx context.Context, directory string) {
	if _, err := c.runGit(ctx, directory, "reset", "--quiet", "--hard", "HEAD"); err != nil {
		return
	}
	c.runGit(ctx, directory, "clean", "--quiet", "-fd")
}

// shellQuote quotes a value for a shell command line shown to the user.
func shellQuote(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\n\"'\\$`&|;<>()*?[]{}!#~") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
