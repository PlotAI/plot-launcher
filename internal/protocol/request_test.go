package protocol

import (
	"net/url"
	"testing"
)

func TestParseURL(t *testing.T) {
	parsed, err := ParseURL("plot-launcher://open?repository=PlotAI%2Fplot-launcher&pull_request=42")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if parsed.Repository != "PlotAI/plot-launcher" {
		t.Fatalf("Repository = %q, want %q", parsed.Repository, "PlotAI/plot-launcher")
	}
	if parsed.PullRequestNumber != 42 {
		t.Fatalf("PullRequestNumber = %d, want 42", parsed.PullRequestNumber)
	}
	pullRequest, err := parsed.PullRequest()
	if err != nil {
		t.Fatalf("PullRequest: %v", err)
	}
	if pullRequest.Owner != "PlotAI" || pullRequest.Name != "plot-launcher" || pullRequest.Number != 42 {
		t.Fatalf("PullRequest = %+v", pullRequest)
	}
}

func TestParseURLTrailingSlash(t *testing.T) {
	if _, err := ParseURL("plot-launcher://open/?repository=owner/name&pull_request=1"); err != nil {
		t.Fatalf("ParseURL with a trailing slash: %v", err)
	}
}

func TestParseURLRejectsInvalidURLs(t *testing.T) {
	tests := []string{
		"https://open?repository=owner/name&pull_request=1",
		"plot-launcher://checkout?repository=owner/name&pull_request=1",
		"plot-launcher://open/extra?repository=owner/name&pull_request=1",
		"plot-launcher://open?repository=owner/name",
		"plot-launcher://open?pull_request=1",
		"plot-launcher://open?repository=owner&pull_request=1",
		"plot-launcher://open?repository=owner/name/extra&pull_request=1",
		"plot-launcher://open?repository=../name&pull_request=1",
		"plot-launcher://open?repository=owner/..&pull_request=1",
		"plot-launcher://open?repository=owner/na%20me&pull_request=1",
		"plot-launcher://open?repository=owner/name&pull_request=0",
		"plot-launcher://open?repository=owner/name&pull_request=-1",
		"plot-launcher://open?repository=owner/name&pull_request=abc",
		"plot-launcher://open?repository=owner/name&pull_request=1&pull_request=2",
		"::not a url::",
	}
	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			if _, err := ParseURL(rawURL); err == nil {
				t.Fatalf("ParseURL(%q) returned no error", rawURL)
			}
		})
	}
}

func TestParseQueryTrimsRepository(t *testing.T) {
	parsed, err := ParseQuery(url.Values{
		"repository":   {" owner/name "},
		"pull_request": {"7"},
	})
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	if parsed.Repository != "owner/name" {
		t.Fatalf("Repository = %q, want %q", parsed.Repository, "owner/name")
	}
}

func TestParseQueryRejectsMissingFields(t *testing.T) {
	if _, err := ParseQuery(url.Values{}); err == nil {
		t.Fatal("ParseQuery with no values returned no error")
	}
	if _, err := ParseQuery(url.Values{"repository": {"owner/name"}}); err == nil {
		t.Fatal("ParseQuery without pull_request returned no error")
	}
	if _, err := ParseQuery(url.Values{"pull_request": {"1"}}); err == nil {
		t.Fatal("ParseQuery without repository returned no error")
	}
}
