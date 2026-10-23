package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestRunner(t *testing.T) (*Runner, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
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

	if code := runner.Run(context.Background(), []string{"settings", "set-base-directory", "workspaces"}); code != 0 {
		t.Fatalf("set-base-directory exit code = %d, want 0: %s", code, stderr)
	}
	if code := runner.Run(context.Background(), []string{"settings", "set-editor", "/bin/true", "--wait"}); code != 0 {
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
	if decoded["editor"] != "/bin/true --wait" {
		t.Fatalf("editor = %q, want %q", decoded["editor"], "/bin/true --wait")
	}
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
	path := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "plot-launcher", "settings.json")
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

func TestOpenDispatchesLauncherURLBeforeCheckout(t *testing.T) {
	runner, _, stderr := newTestRunner(t)
	code := runner.Run(context.Background(), []string{"plot-launcher://open?repository=owner/name&pull_request=7"})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1: %s", code, stderr)
	}
	if !strings.Contains(stderr.String(), "base directory is not set") {
		t.Fatalf("stderr = %q, want the missing settings error", stderr.String())
	}
}

func TestOpenRejectsInvalidArguments(t *testing.T) {
	runner, _, _ := newTestRunner(t)

	tests := [][]string{
		{"open"},
		{"open", "not-a-launcher-url"},
		{"open", "--repository", "owner/name"},
		{"open", "plot-launcher://open?repository=owner/name&pull_request=0"},
		{"open", "plot-launcher://open?repository=owner/name&pull_request=7", "--repository", "owner/name"},
	}
	for _, args := range tests {
		if code := runner.Run(context.Background(), args); code != 2 {
			t.Fatalf("%v exit code = %d, want 2", args, code)
		}
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
