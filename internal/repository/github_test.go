package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const testRemoteContent = "pull request content"

type testRepository struct {
	bare   string
	source string
}

func newTestRepository(t *testing.T) *testRepository {
	t.Helper()
	requireGit(t)

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("create source repository: %v", err)
	}
	runGit(t, source, "init", "--quiet", "-b", "main")
	writeFile(t, filepath.Join(source, "README.md"), "base content")
	runGit(t, source, "add", ".")
	runGit(t, source, "commit", "--quiet", "-m", "base")
	writeFile(t, filepath.Join(source, "README.md"), testRemoteContent)
	runGit(t, source, "commit", "--quiet", "-am", "pull request")

	bare := filepath.Join(root, "origin.git")
	runGit(t, root, "clone", "--quiet", "--bare", source, bare)
	runGit(t, source, "push", "--quiet", bare, "HEAD:refs/pull/7/head")

	return &testRepository{bare: bare, source: source}
}

func (r *testRepository) remoteURL() string {
	return "file://" + r.bare
}

func (r *testRepository) update(t *testing.T, content string) {
	t.Helper()
	writeFile(t, filepath.Join(r.source, "README.md"), content)
	runGit(t, r.source, "commit", "--quiet", "-am", content)
	runGit(t, r.source, "push", "--quiet", r.bare, "HEAD:refs/pull/7/head")
}

func TestGitHubControllerChecksOutPullRequest(t *testing.T) {
	testRepository := newTestRepository(t)
	controller := NewGitHubController()
	controller.remoteURL = func(PullRequest) string { return testRepository.remoteURL() }

	base := t.TempDir()
	pullRequest := PullRequest{Owner: "owner", Name: "name", Number: 7}
	workingDirectory, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
	if err != nil {
		t.Fatalf("CheckoutPRAndPrepareWorkingDirectory: %v", err)
	}
	wantDirectory := pullRequest.WorkingDirectory(base)
	if workingDirectory != wantDirectory {
		t.Fatalf("working directory = %q, want %q", workingDirectory, wantDirectory)
	}
	if content := readFile(t, filepath.Join(workingDirectory, "README.md")); content != testRemoteContent {
		t.Fatalf("README.md = %q, want %q", content, testRemoteContent)
	}
	if branch := runGit(t, workingDirectory, "rev-parse", "--abbrev-ref", "HEAD"); branch != "plot/pr-7" {
		t.Fatalf("branch = %q, want %q", branch, "plot/pr-7")
	}
	if origin := runGit(t, workingDirectory, "remote", "get-url", "origin"); origin != testRepository.remoteURL() {
		t.Fatalf("origin = %q, want %q", origin, testRepository.remoteURL())
	}

	testRepository.update(t, "updated pull request content")
	secondDirectory, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
	if err != nil {
		t.Fatalf("second CheckoutPRAndPrepareWorkingDirectory: %v", err)
	}
	if secondDirectory != workingDirectory {
		t.Fatalf("working directory = %q, want %q", secondDirectory, workingDirectory)
	}
	if content := readFile(t, filepath.Join(workingDirectory, "README.md")); content != "updated pull request content" {
		t.Fatalf("README.md = %q, want %q", content, "updated pull request content")
	}
}

func TestGitHubControllerRejectsDifferentRemote(t *testing.T) {
	requireGit(t)
	controller := NewGitHubController()
	controller.remoteURL = func(PullRequest) string { return "https://github.com/owner/name.git" }

	base := t.TempDir()
	pullRequest := PullRequest{Owner: "owner", Name: "name", Number: 7}
	workingDirectory := pullRequest.WorkingDirectory(base)
	if err := os.MkdirAll(workingDirectory, 0o755); err != nil {
		t.Fatalf("create working directory: %v", err)
	}
	runGit(t, workingDirectory, "init", "--quiet")
	runGit(t, workingDirectory, "remote", "add", "origin", "https://github.com/other/repository.git")

	if _, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base); err == nil {
		t.Fatal("CheckoutPRAndPrepareWorkingDirectory with a different remote returned no error")
	}
}

func TestGitHubControllerValidatesInputs(t *testing.T) {
	controller := NewGitHubController()
	base := t.TempDir()

	if _, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), PullRequest{Owner: "owner", Name: "name"}, base); err == nil {
		t.Fatal("CheckoutPRAndPrepareWorkingDirectory with number 0 returned no error")
	}

	valid := PullRequest{Owner: "owner", Name: "name", Number: 7}
	if _, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), valid, "relative/path"); err == nil {
		t.Fatal("CheckoutPRAndPrepareWorkingDirectory with a relative base directory returned no error")
	}
	if _, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), valid, ""); err == nil {
		t.Fatal("CheckoutPRAndPrepareWorkingDirectory with an empty base directory returned no error")
	}
}

func TestGitHubControllerSerializesConcurrentCheckouts(t *testing.T) {
	testRepository := newTestRepository(t)
	controller := NewGitHubController()
	controller.remoteURL = func(PullRequest) string { return testRepository.remoteURL() }

	base := t.TempDir()
	pullRequest := PullRequest{Owner: "owner", Name: "name", Number: 7}

	var waitGroup sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
			errors <- err
		}()
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent checkout: %v", err)
		}
	}
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

func runGit(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=Plot Test",
		"GIT_AUTHOR_EMAIL=plot@example.com",
		"GIT_COMMITTER_NAME=Plot Test",
		"GIT_COMMITTER_EMAIL=plot@example.com",
		"GIT_TERMINAL_PROMPT=0",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(arguments, " "), directory, err, output)
	}
	return strings.TrimSpace(string(output))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
