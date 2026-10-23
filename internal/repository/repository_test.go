package repository

import (
	"path/filepath"
	"testing"
)

func TestNewPullRequest(t *testing.T) {
	tests := []struct {
		name       string
		repository string
		number     int
		wantOwner  string
		wantName   string
		wantErr    bool
	}{
		{name: "valid", repository: "PlotAI/plot-launcher", number: 42, wantOwner: "PlotAI", wantName: "plot-launcher"},
		{name: "leading dot repository", repository: "owner/.github", number: 1, wantOwner: "owner", wantName: ".github"},
		{name: "empty", repository: "", number: 1, wantErr: true},
		{name: "missing name", repository: "owner/", number: 1, wantErr: true},
		{name: "missing owner", repository: "/name", number: 1, wantErr: true},
		{name: "extra segment", repository: "owner/name/extra", number: 1, wantErr: true},
		{name: "path traversal owner", repository: "../name", number: 1, wantErr: true},
		{name: "path traversal name", repository: "owner/..", number: 1, wantErr: true},
		{name: "invalid characters", repository: "owner/na me", number: 1, wantErr: true},
		{name: "zero number", repository: "owner/name", number: 0, wantErr: true},
		{name: "negative number", repository: "owner/name", number: -3, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pullRequest, err := NewPullRequest(test.repository, test.number)
			if test.wantErr {
				if err == nil {
					t.Fatalf("NewPullRequest(%q, %d) = %+v, want error", test.repository, test.number, pullRequest)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewPullRequest(%q, %d): %v", test.repository, test.number, err)
			}
			if pullRequest.Owner != test.wantOwner || pullRequest.Name != test.wantName || pullRequest.Number != test.number {
				t.Fatalf("NewPullRequest = %+v, want %s/%s#%d", pullRequest, test.wantOwner, test.wantName, test.number)
			}
			if pullRequest.Repository() != test.repository {
				t.Fatalf("Repository = %q, want %q", pullRequest.Repository(), test.repository)
			}
		})
	}
}

func TestWorkingDirectory(t *testing.T) {
	base := t.TempDir()
	pullRequest := PullRequest{Owner: "owner", Name: "name", Number: 7}
	want := filepath.Join(base, "owner", "name", "pull-7")
	if got := pullRequest.WorkingDirectory(base); got != want {
		t.Fatalf("WorkingDirectory = %q, want %q", got, want)
	}
}
