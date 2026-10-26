package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PlotAI/plot-launcher/internal/launcher"
	"github.com/PlotAI/plot-launcher/internal/repository"
	"github.com/PlotAI/plot-launcher/internal/settings"
)

func newTestRunner(t *testing.T) (*Runner, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	t.Setenv("PLOT_LAUNCHER_CONFIG_DIR", filepath.Join(t.TempDir(), "config"))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	runner, err := NewRunner(stdout, stderr)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	return runner, stdout, stderr
}

func TestSettingsShowEmpty(t *testing.T) {
	runner, stdout, stderr := newTestRunner(t)
	if code := runner.Run(context.Background(), []string{"settings", "show"}); code != 0 {
		t.Fatalf("exit code = %d, want 0: %s", code, stderr)
	}
	if strings.TrimSpace(stdout.String()) != "{}" {
		t.Fatalf("stdout = %q, want {}", stdout.String())
	}
}

func TestSettingsSetAndShow(t *testing.T) {
	runner, stdout, stderr := newTestRunner(t)
	workingDirectory := t.TempDir()
	t.Chdir(workingDirectory)
	editor := testExecutable(t)

	if code := runner.Run(context.Background(), []string{"settings", "set-base-directory", "workspaces"}); code != 0 {
		t.Fatalf("set-base-directory exit code = %d, want 0: %s", code, stderr)
	}
	if code := runner.Run(context.Background(), []string{"settings", "set-editor", editor, "--wait"}); code != 0 {
		t.Fatalf("set-editor exit code = %d, want 0: %s", code, stderr)
	}

	stdout.Reset()
	if code := runner.Run(context.Background(), []string{"settings", "show"}); code != 0 {
		t.Fatalf("show exit code = %d, want 0: %s", code, stderr)
	}
	var decoded map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if decoded["base_directory"] != filepath.Join(workingDirectory, "workspaces") {
		t.Fatalf("base_directory = %q, want %q", decoded["base_directory"], filepath.Join(workingDirectory, "workspaces"))
	}
	if want := editor + " --wait"; decoded["editor"] != want {
		t.Fatalf("editor = %q, want %q", decoded["editor"], want)
	}
}

// testExecutable returns an executable on every supported platform.
func testExecutable(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	return executable
}

func TestSettingsRejectsInvalidValues(t *testing.T) {
	runner, _, stderr := newTestRunner(t)

	if code := runner.Run(context.Background(), []string{"settings", "set-editor", "/definitely/not/an/editor"}); code == 0 {
		t.Fatalf("set-editor exit code = 0, want failure")
	}
	if !strings.Contains(stderr.String(), "not executable") {
		t.Fatalf("stderr = %q, want not executable", stderr.String())
	}

	stderr.Reset()
	if code := runner.Run(context.Background(), []string{"settings", "set-base-directory"}); code != 2 {
		t.Fatalf("set-base-directory exit code = %d, want 2", code)
	}
}

func TestSettingsShowRejectsMalformedSettings(t *testing.T) {
	runner, _, stderr := newTestRunner(t)
	path := filepath.Join(os.Getenv("PLOT_LAUNCHER_CONFIG_DIR"), "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	if code := runner.Run(context.Background(), []string{"settings", "show"}); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "launcher settings") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestHelpAndUnknownCommand(t *testing.T) {
	runner, stdout, stderr := newTestRunner(t)

	if code := runner.Run(context.Background(), []string{"help"}); code != 0 {
		t.Fatalf("help exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Plot Launcher") {
		t.Fatalf("help output = %q", stdout.String())
	}

	stderr.Reset()
	if code := runner.Run(context.Background(), []string{"unknown"}); code != 2 {
		t.Fatalf("unknown exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestVersion(t *testing.T) {
	runner, stdout, stderr := newTestRunner(t)

	for _, args := range [][]string{{"version"}, {"--version"}, {"-v"}} {
		stdout.Reset()
		if code := runner.Run(context.Background(), args); code != 0 {
			t.Fatalf("%v exit code = %d, want 0: %s", args, code, stderr)
		}
		if !strings.HasPrefix(stdout.String(), "plot-launcher ") {
			t.Fatalf("%v stdout = %q, want a version line", args, stdout.String())
		}
	}
}

func TestDefaultCommandStartsProtocolHandler(t *testing.T) {
	runner, _, stderr := newTestRunner(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	runner.listen = func() (net.Listener, error) { return listener, nil }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- runner.Run(ctx, nil) }()

	response := getWithRetry(t, "http://"+listener.Addr().String()+"/")
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	cancel()
	if code := <-done; code != 0 {
		t.Fatalf("exit code = %d, want 0: %s", code, stderr)
	}
}

func getWithRetry(t *testing.T, target string) *http.Response {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for {
		response, err := client.Get(target)
		if err == nil {
			return response
		}
		if time.Now().After(deadline) {
			t.Fatalf("GET %s: %v", target, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestRunWithoutArgumentsReportsListenErrors(t *testing.T) {
	runner, _, stderr := newTestRunner(t)
	runner.listen = func() (net.Listener, error) { return nil, errors.New("listen failed") }

	if code := runner.Run(context.Background(), nil); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "listen failed") {
		t.Fatalf("stderr = %q, want the listen error", stderr.String())
	}
}

func TestServeCommandIsRemoved(t *testing.T) {
	runner, _, _ := newTestRunner(t)

	// The Protocol Handler starts without a command, so serve is not a command.
	for _, args := range [][]string{
		{"serve"},
		{"serve", "--host", "::"},
		{"serve", "extra"},
	} {
		if code := runner.Run(context.Background(), args); code != 2 {
			t.Fatalf("%v exit code = %d, want 2", args, code)
		}
	}
}

func TestOpenCommandIsRemoved(t *testing.T) {
	runner, _, _ := newTestRunner(t)

	// A browser link is handled as a bare argument; there is no open command.
	for _, args := range [][]string{
		{"open", "plot://open?repository=owner/name&pull_request=1"},
		{"open", "--repository", "owner/name", "--pull-request", "1"},
	} {
		if code := runner.Run(context.Background(), args); code != 2 {
			t.Fatalf("%v exit code = %d, want 2", args, code)
		}
	}
}

type stubSettings struct{ value settings.LauncherSettings }

func (s stubSettings) Load() (settings.LauncherSettings, error) { return s.value, nil }

type stubRepositories struct{ workingDirectory string }

func (s stubRepositories) CheckoutPRAndPrepareWorkingDirectory(context.Context, repository.PullRequest, string) (string, error) {
	return s.workingDirectory, nil
}

type stubEditors struct{ directory string }

func (s *stubEditors) Launch(_ context.Context, _, directory string) error {
	s.directory = directory
	return nil
}

func TestSchemeURLOpensPullRequest(t *testing.T) {
	runner, stdout, stderr := newTestRunner(t)
	base := t.TempDir()
	workingDirectory := filepath.Join(base, "graph-canvas")
	editors := &stubEditors{}
	runner.launcher = launcher.New(
		stubSettings{value: settings.LauncherSettings{BaseDirectory: base, Editor: "code"}},
		stubRepositories{workingDirectory: workingDirectory},
		editors,
	)

	code := runner.Run(context.Background(), []string{"plot://open?url=https://github.com/PlotAI/graph-canvas/pull/7"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0: %s", code, stderr)
	}
	if editors.directory != workingDirectory {
		t.Fatalf("editor directory = %q, want %q", editors.directory, workingDirectory)
	}
	if strings.TrimSpace(stdout.String()) != workingDirectory {
		t.Fatalf("stdout = %q, want %q", stdout.String(), workingDirectory)
	}
}

func TestSchemeURLRejectsInvalidRequests(t *testing.T) {
	runner, _, _ := newTestRunner(t)

	if code := runner.Run(context.Background(), []string{"plot://open?repository=owner/name"}); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if code := runner.Run(context.Background(), []string{"plot://settings?repository=owner/name&pull_request=1"}); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if code := runner.Run(context.Background(), []string{"plot://open?repository=owner/name&pull_request=1", "extra"}); code != 2 {
		t.Fatalf("exit code with extra arguments = %d, want 2", code)
	}
}
