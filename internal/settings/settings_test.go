package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingReturnsEmptySettings(t *testing.T) {
	manager := NewManager(filepath.Join(t.TempDir(), "missing", "settings.json"))
	loaded, err := manager.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded != (LauncherSettings{}) {
		t.Fatalf("Load = %+v, want empty settings", loaded)
	}
}

func TestSetBaseDirectory(t *testing.T) {
	workingDirectory := t.TempDir()
	t.Chdir(workingDirectory)

	manager := NewManager(filepath.Join(t.TempDir(), "config", "settings.json"))
	updated, err := manager.SetBaseDirectory("workspaces")
	if err != nil {
		t.Fatalf("SetBaseDirectory: %v", err)
	}
	want := filepath.Join(workingDirectory, "workspaces")
	if updated.BaseDirectory != want {
		t.Fatalf("BaseDirectory = %q, want %q", updated.BaseDirectory, want)
	}

	loaded, err := manager.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.BaseDirectory != want {
		t.Fatalf("loaded BaseDirectory = %q, want %q", loaded.BaseDirectory, want)
	}
}

func TestSetBaseDirectoryRejectsInvalidValues(t *testing.T) {
	manager := NewManager(filepath.Join(t.TempDir(), "settings.json"))
	if _, err := manager.SetBaseDirectory("  "); err == nil {
		t.Fatal("SetBaseDirectory with an empty path returned no error")
	}

	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := manager.SetBaseDirectory(file); err == nil {
		t.Fatal("SetBaseDirectory with a file returned no error")
	}
}

func TestSetEditor(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	command := executable + " --wait"

	manager := NewManager(filepath.Join(t.TempDir(), "settings.json"))
	updated, err := manager.SetEditor(command)
	if err != nil {
		t.Fatalf("SetEditor: %v", err)
	}
	if updated.Editor != command {
		t.Fatalf("Editor = %q, want %q", updated.Editor, command)
	}

	loaded, err := manager.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Editor != command {
		t.Fatalf("loaded Editor = %q, want %q", loaded.Editor, command)
	}
}

func TestSetEditorWithQuotedExecutable(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "my editor")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write editor: %v", err)
	}

	manager := NewManager(filepath.Join(t.TempDir(), "settings.json"))
	command := `"` + executable + `" --flag`
	if _, err := manager.SetEditor(command); err != nil {
		t.Fatalf("SetEditor: %v", err)
	}
}

func TestSetEditorRejectsInvalidValues(t *testing.T) {
	manager := NewManager(filepath.Join(t.TempDir(), "settings.json"))
	if _, err := manager.SetEditor(""); err == nil {
		t.Fatal("SetEditor with an empty command returned no error")
	}
	if _, err := manager.SetEditor("/definitely/not/an/editor"); err == nil {
		t.Fatal("SetEditor with a missing executable returned no error")
	}
	if _, err := manager.SetEditor(`/bin/true "unterminated`); err == nil {
		t.Fatal("SetEditor with an unterminated quote returned no error")
	}
}

func TestLoadRejectsMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	manager := NewManager(path)
	if _, err := manager.Load(); err == nil {
		t.Fatal("Load of malformed JSON returned no error")
	}
}

func TestSaveWritesJSONLauncherSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "settings.json")
	manager := NewManager(path)
	want := LauncherSettings{BaseDirectory: "/workspace", Editor: "code --wait"}
	if err := manager.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatalf("settings file does not end with a newline: %q", data)
	}

	var decoded map[string]string
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("settings file is not valid JSON: %v", err)
	}
	if decoded["base_directory"] != want.BaseDirectory || decoded["editor"] != want.Editor {
		t.Fatalf("settings JSON = %v, want %+v", decoded, want)
	}
}
