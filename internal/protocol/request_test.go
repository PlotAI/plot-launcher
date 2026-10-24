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

func TestParseQueryAcceptsPullRequestURL(t *testing.T) {
	tests := []struct {
		name       string
		rawURL     string
		repository string
		number     int
	}{
		{"canonical", "https://github.com/PlotCustomer/menu/pull/6", "PlotCustomer/menu", 6},
		{"trailing slash", "https://github.com/PlotCustomer/menu/pull/6/", "PlotCustomer/menu", 6},
		{"files tab", "https://github.com/PlotCustomer/menu/pull/6/files", "PlotCustomer/menu", 6},
		{"query string", "https://github.com/PlotCustomer/menu/pull/6?diff=split", "PlotCustomer/menu", 6},
		{"http scheme", "http://github.com/PlotCustomer/menu/pull/6", "PlotCustomer/menu", 6},
		{"www host", "https://www.github.com/PlotCustomer/menu/pull/6", "PlotCustomer/menu", 6},
		{"surrounding space", " https://github.com/PlotCustomer/menu/pull/6 ", "PlotCustomer/menu", 6},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := ParseQuery(url.Values{"url": {test.rawURL}})
			if err != nil {
				t.Fatalf("ParseQuery(%q): %v", test.rawURL, err)
			}
			if parsed.Repository != test.repository {
				t.Fatalf("Repository = %q, want %q", parsed.Repository, test.repository)
			}
			if parsed.PullRequestNumber != test.number {
				t.Fatalf("PullRequestNumber = %d, want %d", parsed.PullRequestNumber, test.number)
			}
		})
	}
}

func TestParseQueryRejectsInvalidPullRequestURLs(t *testing.T) {
	tests := map[string]url.Values{
		"empty":              {"url": {""}},
		"other host":         {"url": {"https://gitlab.com/owner/name/pull/1"}},
		"missing pull":       {"url": {"https://github.com/owner/name"}},
		"issue instead":      {"url": {"https://github.com/owner/name/issues/1"}},
		"non-numeric number": {"url": {"https://github.com/owner/name/pull/zero"}},
		"zero number":        {"url": {"https://github.com/owner/name/pull/0"}},
		"path traversal":     {"url": {"https://github.com/owner/../pull/1"}},
		"unsupported scheme": {"url": {"ftp://github.com/owner/name/pull/1"}},
		"mixed with repository": {
			"url":        {"https://github.com/owner/name/pull/1"},
			"repository": {"owner/name"},
		},
		"mixed with pull_request": {
			"url":          {"https://github.com/owner/name/pull/1"},
			"pull_request": {"1"},
		},
		"repeated url": {"url": {
			"https://github.com/owner/name/pull/1",
			"https://github.com/owner/name/pull/2",
		}},
	}
	for name, values := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseQuery(values); err == nil {
				t.Fatalf("ParseQuery(%v) returned no error", values)
			}
		})
	}
}

func TestParseURLAcceptsPullRequestURLQuery(t *testing.T) {
	parsed, err := ParseURL("plot-launcher://open?url=https%3A%2F%2Fgithub.com%2Fowner%2Fname%2Fpull%2F9")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if parsed.Repository != "owner/name" || parsed.PullRequestNumber != 9 {
		t.Fatalf("parsed = %+v", parsed)
	}
}
