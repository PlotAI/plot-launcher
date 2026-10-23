// Package protocol implements the Protocol Handler application interface. It
// receives "Open In Editor Requested" browser requests over HTTP or as a
// plot-launcher URL and hands them to the Plot Launcher.
package protocol

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/PlotAI/plot-launcher/internal/repository"
)

// Scheme is the launcher protocol scheme used by browsers and the operating
// system protocol registration.
const Scheme = "plot-launcher"

// EventOpenInEditorRequested is the application event that triggers the
// Protocol Handler to receive a browser request.
const EventOpenInEditorRequested = "Open In Editor Requested"

// OpenRequest is the payload of an Open In Editor Requested event.
type OpenRequest struct {
	Repository        string
	PullRequestNumber int
}

// PullRequest returns the validated pull request carried by the request.
func (r OpenRequest) PullRequest() (repository.PullRequest, error) {
	return repository.NewPullRequest(r.Repository, r.PullRequestNumber)
}

// Validate reports whether the request carries a valid repository and pull
// request number.
func (r OpenRequest) Validate() error {
	_, err := r.PullRequest()
	return err
}

// ParseURL parses a plot-launcher://open?repository=owner/name&pull_request=1
// URL.
func ParseURL(rawURL string) (OpenRequest, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return OpenRequest{}, fmt.Errorf("parse launcher URL: %w", err)
	}
	if parsed.Scheme != Scheme {
		return OpenRequest{}, fmt.Errorf("launcher URL must use the %s scheme", Scheme)
	}
	if parsed.Host != "open" {
		return OpenRequest{}, fmt.Errorf("unsupported launcher action %q", parsed.Host)
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return OpenRequest{}, fmt.Errorf("unsupported launcher path %q", parsed.Path)
	}
	return ParseQuery(parsed.Query())
}

// ParseQuery parses an Open In Editor Requested request from query values.
func ParseQuery(values url.Values) (OpenRequest, error) {
	repositoryValue, err := singleValue(values, "repository")
	if err != nil {
		return OpenRequest{}, err
	}
	pullRequestValue, err := singleValue(values, "pull_request")
	if err != nil {
		return OpenRequest{}, err
	}

	number, err := strconv.Atoi(pullRequestValue)
	if err != nil {
		return OpenRequest{}, errors.New("pull_request must be a positive integer")
	}
	request := OpenRequest{
		Repository:        strings.TrimSpace(repositoryValue),
		PullRequestNumber: number,
	}
	if err := request.Validate(); err != nil {
		return OpenRequest{}, err
	}
	return request, nil
}

func singleValue(values url.Values, key string) (string, error) {
	provided, ok := values[key]
	if !ok || len(provided) == 0 {
		return "", fmt.Errorf("%s is required", key)
	}
	if len(provided) > 1 {
		return "", fmt.Errorf("%s must be provided once", key)
	}
	return provided[0], nil
}
