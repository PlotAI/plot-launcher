// Package repository implements the Repository Controller and its GitHub
// Controller specialization.
package repository

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// segmentPattern matches a single repository path segment. Path traversal is
// rejected separately by Validate.
var segmentPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// PullRequest identifies a pull request in a source repository.
type PullRequest struct {
	Owner  string
	Name   string
	Number int
}

// NewPullRequest validates a repository name and pull request number.
func NewPullRequest(repositoryName string, number int) (PullRequest, error) {
	owner, name, err := splitRepository(repositoryName)
	if err != nil {
		return PullRequest{}, err
	}
	pullRequest := PullRequest{Owner: owner, Name: name, Number: number}
	if err := pullRequest.Validate(); err != nil {
		return PullRequest{}, err
	}
	return pullRequest, nil
}

// Repository returns the owner/name form of the repository.
func (p PullRequest) Repository() string {
	return p.Owner + "/" + p.Name
}

// Validate reports whether the pull request identifies a valid repository and
// a positive pull request number.
func (p PullRequest) Validate() error {
	if _, _, err := splitRepository(p.Repository()); err != nil {
		return err
	}
	if p.Number <= 0 {
		return errors.New("pull request number must be a positive integer")
	}
	return nil
}

// WorkingDirectory returns the prepared working directory for the pull
// request below the configured base directory. It is named after the
// repository, so every pull request of a repository shares one directory and
// git switches the branch for the requested pull request.
func (p PullRequest) WorkingDirectory(baseDirectory string) string {
	return filepath.Join(baseDirectory, p.Name)
}

func splitRepository(repositoryName string) (string, string, error) {
	parts := strings.Split(repositoryName, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("repository %q must use the owner/name form", repositoryName)
	}
	for _, part := range parts {
		if part == "." || part == ".." {
			return "", "", fmt.Errorf("repository %q contains an invalid path segment", repositoryName)
		}
		if !segmentPattern.MatchString(part) {
			return "", "", fmt.Errorf("repository %q contains invalid characters", repositoryName)
		}
	}
	return parts[0], parts[1], nil
}

// Codes of the recoveries a Repository Controller can report in a Notice.
const (
	// NoticeStashKept reports uncommitted changes that were set aside and
	// could not be restored on the checked out pull request.
	NoticeStashKept = "stash_kept"
	// NoticeRemoteUpdated reports a working directory whose origin was
	// pointed at the repository's new location.
	NoticeRemoteUpdated = "remote_updated"
	// NoticeLocalCommitsKept reports commits that were moved off the pull
	// request branch before it was reset.
	NoticeLocalCommitsKept = "local_commits_kept"
)

// Codes of the failures a Repository Controller can report in a
// CheckoutError.
const (
	// CodeWorkingTree reports uncommitted changes that could not be set
	// aside before the branch switch.
	CodeWorkingTree = "working_tree"
	// CodeRemoteMismatch reports a working directory that tracks another
	// repository than the requested one.
	CodeRemoteMismatch = "remote_mismatch"
	// CodeLocalCommits reports commits that could not be moved off the pull
	// request branch before it was reset.
	CodeLocalCommits = "local_commits"
)

// Notice reports a recovery performed on a working directory, so the caller
// can tell the user what changed behind their back. Command, when set, is the
// command that inspects or undoes the recovery.
type Notice struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Command string `json:"command,omitempty"`
}

// Checkout is a prepared working directory and the recoveries that preparing
// it required.
type Checkout struct {
	WorkingDirectory string
	Notices          []Notice
}

// CheckoutError reports a checkout failure in terms the user can act on: a
// stable code, a message that says what went wrong, what to do about it, and
// the command that does it.
type CheckoutError struct {
	Code    string
	Message string
	Remedy  string
	Command string
	Err     error
}

// Error returns the message for the user.
func (e *CheckoutError) Error() string {
	return e.Message
}

// Unwrap returns the failure that caused the checkout to fail.
func (e *CheckoutError) Unwrap() error {
	return e.Err
}

// Controller is the Repository Controller: it checks out a pull request and
// prepares its working directory.
type Controller interface {
	CheckoutPRAndPrepareWorkingDirectory(ctx context.Context, pullRequest PullRequest, baseDirectory string) (Checkout, error)
}
