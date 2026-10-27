package repository

import (
	"context"
	"errors"
	"fmt"
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
	r.pushPullRequest(t, 7, content)
}

func (r *testRepository) pushPullRequest(t *testing.T, number int, content string) {
	t.Helper()
	writeFile(t, filepath.Join(r.source, "README.md"), content)
	runGit(t, r.source, "commit", "--quiet", "-am", content)
	runGit(t, r.source, "push", "--quiet", r.bare, fmt.Sprintf("HEAD:refs/pull/%d/head", number))
}

func TestGitHubControllerChecksOutPullRequest(t *testing.T) {
	testRepository := newTestRepository(t)
	controller := NewGitHubController()
	controller.remoteURL = func(PullRequest) string { return testRepository.remoteURL() }

	base := t.TempDir()
	pullRequest := PullRequest{Owner: "owner", Name: "name", Number: 7}
	checkout, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
	if err != nil {
		t.Fatalf("CheckoutPRAndPrepareWorkingDirectory: %v", err)
	}
	workingDirectory := checkout.WorkingDirectory
	wantDirectory := pullRequest.WorkingDirectory(base)
	if workingDirectory != wantDirectory {
		t.Fatalf("working directory = %q, want %q", workingDirectory, wantDirectory)
	}
	if len(checkout.Notices) != 0 {
		t.Fatalf("notices = %+v, want none", checkout.Notices)
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
	secondCheckout, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
	if err != nil {
		t.Fatalf("second CheckoutPRAndPrepareWorkingDirectory: %v", err)
	}
	if secondCheckout.WorkingDirectory != workingDirectory {
		t.Fatalf("working directory = %q, want %q", secondCheckout.WorkingDirectory, workingDirectory)
	}
	if content := readFile(t, filepath.Join(workingDirectory, "README.md")); content != "updated pull request content" {
		t.Fatalf("README.md = %q, want %q", content, "updated pull request content")
	}
}

func TestGitHubControllerSwitchesPullRequestsInOneDirectory(t *testing.T) {
	testRepository := newTestRepository(t)
	controller := NewGitHubController()
	controller.remoteURL = func(PullRequest) string { return testRepository.remoteURL() }

	base := t.TempDir()
	first := PullRequest{Owner: "owner", Name: "name", Number: 7}
	firstCheckout, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), first, base)
	if err != nil {
		t.Fatalf("first checkout: %v", err)
	}
	firstDirectory := firstCheckout.WorkingDirectory

	testRepository.pushPullRequest(t, 8, "other pull request content")
	second := PullRequest{Owner: "owner", Name: "name", Number: 8}
	secondCheckout, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), second, base)
	if err != nil {
		t.Fatalf("second checkout: %v", err)
	}
	secondDirectory := secondCheckout.WorkingDirectory
	if secondDirectory != firstDirectory {
		t.Fatalf("working directory = %q, want the repository directory %q", secondDirectory, firstDirectory)
	}
	if branch := runGit(t, secondDirectory, "rev-parse", "--abbrev-ref", "HEAD"); branch != "plot/pr-8" {
		t.Fatalf("branch = %q, want %q", branch, "plot/pr-8")
	}
	if content := readFile(t, filepath.Join(secondDirectory, "README.md")); content != "other pull request content" {
		t.Fatalf("README.md = %q, want %q", content, "other pull request content")
	}

	backCheckout, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), first, base)
	if err != nil {
		t.Fatalf("checkout of the first pull request again: %v", err)
	}
	backDirectory := backCheckout.WorkingDirectory
	if backDirectory != firstDirectory {
		t.Fatalf("working directory = %q, want %q", backDirectory, firstDirectory)
	}
	if branch := runGit(t, backDirectory, "rev-parse", "--abbrev-ref", "HEAD"); branch != "plot/pr-7" {
		t.Fatalf("branch = %q, want %q", branch, "plot/pr-7")
	}
	if content := readFile(t, filepath.Join(backDirectory, "README.md")); content != testRemoteContent {
		t.Fatalf("README.md = %q, want %q", content, testRemoteContent)
	}
}

func TestGitHubControllerFollowsMovedRepository(t *testing.T) {
	testRepository := newTestRepository(t)
	controller := NewGitHubController()
	controller.remoteURL = func(PullRequest) string { return testRepository.remoteURL() }

	base := t.TempDir()
	pullRequest := PullRequest{Owner: "owner", Name: "name", Number: 7}
	firstCheckout, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
	if err != nil {
		t.Fatalf("first checkout: %v", err)
	}
	workingDirectory := firstCheckout.WorkingDirectory

	// A transferred repository answers at its new location with the same
	// pull request head.
	moved := filepath.Join(t.TempDir(), "moved.git")
	runGit(t, base, "clone", "--quiet", "--mirror", testRepository.bare, moved)
	controller.remoteURL = func(PullRequest) string { return "file://" + moved }

	secondCheckout, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
	if err != nil {
		t.Fatalf("checkout of the moved repository: %v", err)
	}
	if origin := runGit(t, workingDirectory, "remote", "get-url", "origin"); origin != "file://"+moved {
		t.Fatalf("origin = %q, want %q", origin, "file://"+moved)
	}
	if content := readFile(t, filepath.Join(workingDirectory, "README.md")); content != testRemoteContent {
		t.Fatalf("README.md = %q, want %q", content, testRemoteContent)
	}
	if len(secondCheckout.Notices) != 1 || secondCheckout.Notices[0].Code != NoticeRemoteUpdated {
		t.Fatalf("notices = %+v, want one %q notice", secondCheckout.Notices, NoticeRemoteUpdated)
	}
}

func TestGitHubControllerRejectsDifferentRemote(t *testing.T) {
	testRepository := newTestRepository(t)
	controller := NewGitHubController()
	controller.remoteURL = func(PullRequest) string { return testRepository.remoteURL() }

	base := t.TempDir()
	pullRequest := PullRequest{Owner: "owner", Name: "name", Number: 7}
	if _, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base); err != nil {
		t.Fatalf("first checkout: %v", err)
	}

	// Another repository carrying an unrelated pull request 7 must not
	// replace the checkout.
	other := newTestRepository(t)
	other.pushPullRequest(t, 7, "unrelated pull request content")
	controller.remoteURL = func(PullRequest) string { return other.remoteURL() }

	_, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
	if err == nil {
		t.Fatal("CheckoutPRAndPrepareWorkingDirectory with a different remote returned no error")
	}
	var checkoutError *CheckoutError
	if !errors.As(err, &checkoutError) || checkoutError.Code != CodeRemoteMismatch {
		t.Fatalf("error = %v, want a %q error", err, CodeRemoteMismatch)
	}
	if checkoutError.Remedy == "" {
		t.Fatal("remedy is empty")
	}
}

func TestGitHubControllerRestoresUncommittedChanges(t *testing.T) {
	testRepository := newTestRepository(t)
	controller := NewGitHubController()
	controller.remoteURL = func(PullRequest) string { return testRepository.remoteURL() }

	base := t.TempDir()
	pullRequest := PullRequest{Owner: "owner", Name: "name", Number: 7}
	checkout, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
	if err != nil {
		t.Fatalf("first checkout: %v", err)
	}
	workingDirectory := checkout.WorkingDirectory

	writeFile(t, filepath.Join(workingDirectory, "README.md"), "local edit")
	writeFile(t, filepath.Join(workingDirectory, "notes.md"), "local note")

	// An editor has the repository open with uncommitted work: opening the
	// pull request again keeps that work.
	second, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
	if err != nil {
		t.Fatalf("checkout with uncommitted changes: %v", err)
	}
	if len(second.Notices) != 0 {
		t.Fatalf("notices = %+v, want none", second.Notices)
	}
	if content := readFile(t, filepath.Join(workingDirectory, "README.md")); content != "local edit" {
		t.Fatalf("README.md = %q, want %q", content, "local edit")
	}
	if content := readFile(t, filepath.Join(workingDirectory, "notes.md")); content != "local note" {
		t.Fatalf("notes.md = %q, want %q", content, "local note")
	}
	if stashes := runGit(t, workingDirectory, "stash", "list"); stashes != "" {
		t.Fatalf("stash list = %q, want empty", stashes)
	}

	// Work that another pull request does not touch follows the branch switch.
	runGit(t, workingDirectory, "checkout", "--quiet", "--", "README.md")
	testRepository.pushPullRequest(t, 8, "other pull request content")
	third, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), PullRequest{Owner: "owner", Name: "name", Number: 8}, base)
	if err != nil {
		t.Fatalf("checkout of another pull request: %v", err)
	}
	if len(third.Notices) != 0 {
		t.Fatalf("notices = %+v, want none", third.Notices)
	}
	if branch := runGit(t, workingDirectory, "rev-parse", "--abbrev-ref", "HEAD"); branch != "plot/pr-8" {
		t.Fatalf("branch = %q, want %q", branch, "plot/pr-8")
	}
	if content := readFile(t, filepath.Join(workingDirectory, "README.md")); content != "other pull request content" {
		t.Fatalf("README.md = %q, want %q", content, "other pull request content")
	}
	if content := readFile(t, filepath.Join(workingDirectory, "notes.md")); content != "local note" {
		t.Fatalf("notes.md = %q, want %q", content, "local note")
	}
	if stashes := runGit(t, workingDirectory, "stash", "list"); stashes != "" {
		t.Fatalf("stash list = %q, want empty", stashes)
	}
}

func TestGitHubControllerKeepsConflictingChangesInStash(t *testing.T) {
	testRepository := newTestRepository(t)
	controller := NewGitHubController()
	controller.remoteURL = func(PullRequest) string { return testRepository.remoteURL() }

	base := t.TempDir()
	pullRequest := PullRequest{Owner: "owner", Name: "name", Number: 7}
	checkout, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
	if err != nil {
		t.Fatalf("first checkout: %v", err)
	}
	workingDirectory := checkout.WorkingDirectory
	writeFile(t, filepath.Join(workingDirectory, "README.md"), "local edit")

	// The pull request moves on under the local edit, so restoring it would
	// mean resolving a conflict: the edit stays in the stash instead.
	testRepository.update(t, "updated pull request content")
	second, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
	if err != nil {
		t.Fatalf("checkout with a conflicting change: %v", err)
	}
	if content := readFile(t, filepath.Join(workingDirectory, "README.md")); content != "updated pull request content" {
		t.Fatalf("README.md = %q, want %q", content, "updated pull request content")
	}
	if status := runGit(t, workingDirectory, "status", "--porcelain"); status != "" {
		t.Fatalf("status = %q, want a clean working tree", status)
	}
	if len(second.Notices) != 1 || second.Notices[0].Code != NoticeStashKept {
		t.Fatalf("notices = %+v, want one %q notice", second.Notices, NoticeStashKept)
	}
	if stashed := runGit(t, workingDirectory, "stash", "list"); !strings.Contains(stashed, "plot: changes on plot/pr-7") {
		t.Fatalf("stash list = %q, want the plot stash entry", stashed)
	}

	// The set aside work is still recoverable from the stash entry.
	if stashed := runGit(t, workingDirectory, "stash", "show", "--patch", "stash@{0}"); !strings.Contains(stashed, "+local edit") {
		t.Fatalf("stash entry = %q, want it to hold the local edit", stashed)
	}
}

func TestGitHubControllerRestoresChangesWhenTheCheckoutFails(t *testing.T) {
	testRepository := newTestRepository(t)
	controller := NewGitHubController()
	controller.remoteURL = func(PullRequest) string { return testRepository.remoteURL() }

	base := t.TempDir()
	pullRequest := PullRequest{Owner: "owner", Name: "name", Number: 7}
	checkout, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
	if err != nil {
		t.Fatalf("first checkout: %v", err)
	}
	workingDirectory := checkout.WorkingDirectory
	writeFile(t, filepath.Join(workingDirectory, "README.md"), "local edit")

	// Pull request 9 does not exist, so the fetch fails after the changes
	// were set aside; they must come back.
	missing := PullRequest{Owner: "owner", Name: "name", Number: 9}
	if _, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), missing, base); err == nil {
		t.Fatal("checkout of a missing pull request returned no error")
	}
	if content := readFile(t, filepath.Join(workingDirectory, "README.md")); content != "local edit" {
		t.Fatalf("README.md = %q, want %q", content, "local edit")
	}
	if stashes := runGit(t, workingDirectory, "stash", "list"); stashes != "" {
		t.Fatalf("stash list = %q, want empty", stashes)
	}
}

func TestGitHubControllerKeepsLocalCommits(t *testing.T) {
	testRepository := newTestRepository(t)
	controller := NewGitHubController()
	controller.remoteURL = func(PullRequest) string { return testRepository.remoteURL() }

	base := t.TempDir()
	pullRequest := PullRequest{Owner: "owner", Name: "name", Number: 7}
	checkout, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
	if err != nil {
		t.Fatalf("first checkout: %v", err)
	}
	workingDirectory := checkout.WorkingDirectory

	writeFile(t, filepath.Join(workingDirectory, "local.md"), "local work")
	runGit(t, workingDirectory, "add", ".")
	runGit(t, workingDirectory, "commit", "--quiet", "-m", "local work")
	localCommit := runGit(t, workingDirectory, "rev-parse", "HEAD")

	// The pull request moves on, and the branch is reset to its head: the
	// local commit must not become unreachable.
	testRepository.update(t, "updated pull request content")
	second, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
	if err != nil {
		t.Fatalf("checkout with a local commit: %v", err)
	}
	if head := runGit(t, workingDirectory, "rev-parse", "HEAD"); head == localCommit {
		t.Fatal("branch was not reset to the pull request head")
	}
	if branch := runGit(t, workingDirectory, "rev-parse", "--abbrev-ref", "HEAD"); branch != "plot/pr-7" {
		t.Fatalf("branch = %q, want %q", branch, "plot/pr-7")
	}
	if saved := runGit(t, workingDirectory, "rev-parse", "plot/pr-7-local"); saved != localCommit {
		t.Fatalf("plot/pr-7-local = %q, want %q", saved, localCommit)
	}
	if len(second.Notices) != 1 || second.Notices[0].Code != NoticeLocalCommitsKept {
		t.Fatalf("notices = %+v, want one %q notice", second.Notices, NoticeLocalCommitsKept)
	}

	// Opening the same pull request again reports nothing: the branch now
	// holds exactly what the launcher checked out.
	third, err := controller.CheckoutPRAndPrepareWorkingDirectory(context.Background(), pullRequest, base)
	if err != nil {
		t.Fatalf("third checkout: %v", err)
	}
	if len(third.Notices) != 0 {
		t.Fatalf("notices = %+v, want none", third.Notices)
	}
	if saved := runGit(t, workingDirectory, "rev-parse", "plot/pr-7-local"); saved != localCommit {
		t.Fatalf("plot/pr-7-local = %q, want %q", saved, localCommit)
	}
}

func TestSaveBranchKeepsDivergentHistory(t *testing.T) {
	requireGit(t)
	directory := t.TempDir()
	path := filepath.Join(directory, "file.md")
	runGit(t, directory, "init", "--quiet", "-b", "main")
	writeFile(t, path, "one")
	runGit(t, directory, "add", ".")
	runGit(t, directory, "commit", "--quiet", "-m", "one")
	first := runGit(t, directory, "rev-parse", "HEAD")
	writeFile(t, path, "two")
	runGit(t, directory, "commit", "--quiet", "-am", "two")
	second := runGit(t, directory, "rev-parse", "HEAD")

	runGit(t, directory, "checkout", "--quiet", "-b", "side", first)
	writeFile(t, path, "side")
	runGit(t, directory, "commit", "--quiet", "-am", "side")
	side := runGit(t, directory, "rev-parse", "HEAD")

	controller := NewGitHubController()
	saved, err := controller.saveBranch(context.Background(), directory, "plot/pr-7-local", side)
	if err != nil {
		t.Fatalf("saveBranch: %v", err)
	}
	if saved != "plot/pr-7-local" {
		t.Fatalf("branch = %q, want %q", saved, "plot/pr-7-local")
	}

	// A commit that does not descend from the saved branch gets its own name,
	// so the first branch is not rewritten.
	divergent, err := controller.saveBranch(context.Background(), directory, "plot/pr-7-local", second)
	if err != nil {
		t.Fatalf("saveBranch divergent: %v", err)
	}
	if divergent != "plot/pr-7-local-2" {
		t.Fatalf("branch = %q, want %q", divergent, "plot/pr-7-local-2")
	}
	if head := runGit(t, directory, "rev-parse", "plot/pr-7-local"); head != side {
		t.Fatalf("plot/pr-7-local = %q, want %q", head, side)
	}

	// A descendant fast-forwards the existing branch.
	runGit(t, directory, "checkout", "--quiet", "main")
	writeFile(t, path, "three")
	runGit(t, directory, "commit", "--quiet", "-am", "three")
	third := runGit(t, directory, "rev-parse", "HEAD")
	advanced, err := controller.saveBranch(context.Background(), directory, "plot/pr-7-local-2", third)
	if err != nil {
		t.Fatalf("saveBranch descendant: %v", err)
	}
	if advanced != "plot/pr-7-local-2" {
		t.Fatalf("branch = %q, want %q", advanced, "plot/pr-7-local-2")
	}
	if head := runGit(t, directory, "rev-parse", "plot/pr-7-local-2"); head != third {
		t.Fatalf("plot/pr-7-local-2 = %q, want %q", head, third)
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
