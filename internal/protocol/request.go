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

// ParseQuery parses an Open In Editor Requested request from query values. A
// request carries either repository and pull_request, or a single url naming a
// GitHub pull request.
func ParseQuery(values url.Values) (OpenRequest, error) {
	if _, ok := values["url"]; ok {
		return parsePullRequestURLQuery(values)
	}

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

// parsePullRequestURLQuery parses a request that carries a GitHub pull request
// URL instead of repository and pull_request.
func parsePullRequestURLQuery(values url.Values) (OpenRequest, error) {
	if _, ok := values["repository"]; ok {
		return OpenRequest{}, errors.New("provide either url or repository and pull_request, not both")
	}
	if _, ok := values["pull_request"]; ok {
		return OpenRequest{}, errors.New("provide either url or repository and pull_request, not both")
	}
	rawURL, err := singleValue(values, "url")
	if err != nil {
		return OpenRequest{}, err
	}
	return ParsePullRequestURL(rawURL)
}

// ParsePullRequestURL parses a GitHub pull request URL such as
// https://github.com/owner/name/pull/42. Trailing path segments and query
// values, as found in links to a pull request's files or commits, are ignored.
func ParsePullRequestURL(rawURL string) (OpenRequest, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return OpenRequest{}, errors.New("url is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return OpenRequest{}, fmt.Errorf("parse pull request URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return OpenRequest{}, errors.New("pull request URL must use the http or https scheme")
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "github.com", "www.github.com":
	default:
		return OpenRequest{}, errors.New("pull request URL must point at github.com")
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 4 || segments[2] != "pull" {
		return OpenRequest{}, fmt.Errorf("pull request URL %q must use the github.com/<owner>/<name>/pull/<number> form", trimmed)
	}
	number, err := strconv.Atoi(segments[3])
	if err != nil {
		return OpenRequest{}, errors.New("pull request URL must contain a positive pull request number")
	}

	request := OpenRequest{
		Repository:        segments[0] + "/" + segments[1],
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
