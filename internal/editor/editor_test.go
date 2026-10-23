package editor

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
		wantErr bool
	}{
		{name: "executable only", command: "code", want: []string{"code"}},
		{name: "flags", command: "code --wait", want: []string{"code", "--wait"}},
		{name: "double quoted argument", command: `code -n "My Project"`, want: []string{"code", "-n", "My Project"}},
		{name: "single quoted executable", command: `'/opt/My Editor/editor' --new-window`, want: []string{"/opt/My Editor/editor", "--new-window"}},
		{name: "escaped quote", command: `code "a \"quoted\" name"`, want: []string{"code", `a "quoted" name`}},
		{name: "escaped space", command: `code a\ b`, want: []string{"code", "a b"}},
		{name: "backslash in single quotes", command: `code 'a\b'`, want: []string{"code", `a\b`}},
		{name: "empty argument", command: `code ""`, want: []string{"code", ""}},
		{name: "empty", command: "", wantErr: true},
		{name: "whitespace", command: "  \t ", wantErr: true},
		{name: "unterminated quote", command: `code "unterminated`, wantErr: true},
		{name: "unterminated escape", command: `code trailing\`, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseCommand(test.command)
			if test.wantErr {
				if err == nil {
					t.Fatalf("ParseCommand(%q) = %v, want error", test.command, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCommand(%q): %v", test.command, err)
			}
			if !slices.Equal(got, test.want) {
				t.Fatalf("ParseCommand(%q) = %v, want %v", test.command, got, test.want)
			}
		})
	}
}

func TestLaunch(t *testing.T) {
	directory := t.TempDir()
	marker := filepath.Join(t.TempDir(), "editor.log")
	t.Setenv("PLOT_EDITOR_MARKER", marker)

	script := filepath.Join(t.TempDir(), "editor.sh")
	content := "#!/bin/sh\nprintf 'dir=%s\\n' \"$PWD\" > \"$PLOT_EDITOR_MARKER\"\nfor argument in \"$@\"; do printf 'arg=%s\\n' \"$argument\" >> \"$PLOT_EDITOR_MARKER\"; done\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}

	launcher := NewLauncher(os.Stderr, os.Stderr)
	if err := launcher.Launch(context.Background(), script+" --new-window", directory); err != nil {
		t.Fatalf("Launch: %v", err)
	}

	output := waitForFile(t, marker)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 3 {
		t.Fatalf("editor log = %q, want 3 lines", output)
	}
	if lines[0] != "dir="+directory {
		t.Fatalf("editor working directory = %q, want %q", lines[0], "dir="+directory)
	}
	if lines[1] != "arg=--new-window" || lines[2] != "arg="+directory {
		t.Fatalf("editor arguments = %q, want --new-window and %s", lines[1:], directory)
	}
}

func TestLaunchRequiresWorkingDirectory(t *testing.T) {
	launcher := NewLauncher(os.Stderr, os.Stderr)
	if err := launcher.Launch(context.Background(), "/bin/true", ""); err == nil {
		t.Fatal("Launch with an empty working directory returned no error")
	}
}

func TestLaunchReportsMissingEditor(t *testing.T) {
	launcher := NewLauncher(os.Stderr, os.Stderr)
	if err := launcher.Launch(context.Background(), "/definitely/not/an/editor", t.TempDir()); err == nil {
		t.Fatal("Launch with a missing editor returned no error")
	}
}

func waitForFile(t *testing.T, path string) []byte {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(path)
		if err == nil {
			return content
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("editor did not write %s", path)
	return nil
}
