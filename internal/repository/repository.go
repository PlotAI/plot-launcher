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

// Controller is the Repository Controller: it checks out a pull request and
// prepares its working directory.
type Controller interface {
	CheckoutPRAndPrepareWorkingDirectory(ctx context.Context, pullRequest PullRequest, baseDirectory string) (string, error)
}
