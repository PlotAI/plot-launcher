package protocol

import (
	"net/url"
	"testing"
)

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

func TestParseSchemeURL(t *testing.T) {
	tests := []struct {
		name       string
		rawURL     string
		repository string
		number     int
	}{
		{"pull request url", "plot-launcher://open?url=https://github.com/owner/name/pull/7", "owner/name", 7},
		{"encoded pull request url", "plot-launcher://open?url=https%3A%2F%2Fgithub.com%2Fowner%2Fname%2Fpull%2F7", "owner/name", 7},
		{"repository and number", "plot-launcher://open?repository=owner/name&pull_request=7", "owner/name", 7},
		{"trailing slash", "plot-launcher://open/?repository=owner/name&pull_request=7", "owner/name", 7},
		{"uppercase scheme", "PLOT-LAUNCHER://open?repository=owner/name&pull_request=7", "owner/name", 7},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := ParseSchemeURL(test.rawURL)
			if err != nil {
				t.Fatalf("ParseSchemeURL(%q): %v", test.rawURL, err)
			}
			if parsed.Repository != test.repository || parsed.PullRequestNumber != test.number {
				t.Fatalf("ParseSchemeURL(%q) = %+v, want %s#%d", test.rawURL, parsed, test.repository, test.number)
			}
		})
	}
}

func TestParseSchemeURLRejectsInvalidURLs(t *testing.T) {
	tests := map[string]string{
		"wrong scheme":          "https://github.com/owner/name/pull/7",
		"wrong host":            "plot-launcher://settings?repository=owner/name&pull_request=7",
		"unexpected path":       "plot-launcher://open/settings?repository=owner/name&pull_request=7",
		"missing parameters":    "plot-launcher://open",
		"missing number":        "plot-launcher://open?repository=owner/name",
		"non-numeric number":    "plot-launcher://open?repository=owner/name&pull_request=zero",
		"unsupported pull host": "plot-launcher://open?url=https://example.com/owner/name/pull/7",
	}
	for name, rawURL := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseSchemeURL(rawURL); err == nil {
				t.Fatalf("ParseSchemeURL(%q) returned no error", rawURL)
			}
		})
	}
}

func TestIsSchemeURL(t *testing.T) {
	tests := map[string]bool{
		"plot-launcher://open?repository=owner/name&pull_request=7": true,
		"  plot-launcher://open":                                    true,
		"https://github.com/owner/name/pull/7":                      false,
		"plot-launcher":                                             false,
		"serve":                                                     false,
	}
	for rawURL, want := range tests {
		if got := IsSchemeURL(rawURL); got != want {
			t.Fatalf("IsSchemeURL(%q) = %t, want %t", rawURL, got, want)
		}
	}
}
