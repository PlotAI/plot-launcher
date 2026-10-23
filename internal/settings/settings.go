// Package settings implements the Settings Manager and the Launcher Settings
// data object, which is persisted as JSON.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/PlotAI/plot-launcher/internal/editor"
)

const (
	appDirectory   = "plot-launcher"
	settingsFile   = "settings.json"
	filePermission = 0o600
	dirPermission  = 0o755
)

// LauncherSettings is the Launcher Settings data object.
type LauncherSettings struct {
	BaseDirectory string `json:"base_directory,omitempty"`
	Editor        string `json:"editor,omitempty"`
}

// Manager reads and writes Launcher Settings.
type Manager struct {
	path string
}

// NewDefaultManager returns a Manager for the user configuration directory.
func NewDefaultManager() (*Manager, error) {
	configDirectory, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user configuration directory: %w", err)
	}
	return NewManager(filepath.Join(configDirectory, appDirectory, settingsFile)), nil
}

// NewManager returns a Manager for the settings file at path.
func NewManager(path string) *Manager {
	return &Manager{path: path}
}

// Path returns the location of the Launcher Settings file.
func (m *Manager) Path() string {
	return m.path
}

// Load returns the Launcher Settings. A missing settings file is reported as
// empty settings so the caller can decide which values are required.
func (m *Manager) Load() (LauncherSettings, error) {
	data, err := os.ReadFile(m.path)
	if errors.Is(err, fs.ErrNotExist) {
		return LauncherSettings{}, nil
	}
	if err != nil {
		return LauncherSettings{}, fmt.Errorf("read launcher settings %s: %w", m.path, err)
	}

	var loaded LauncherSettings
	if err := json.Unmarshal(data, &loaded); err != nil {
		return LauncherSettings{}, fmt.Errorf("decode launcher settings %s: %w", m.path, err)
	}
	return loaded, nil
}

// Save writes the Launcher Settings atomically as JSON.
func (m *Manager) Save(launcherSettings LauncherSettings) error {
	directory := filepath.Dir(m.path)
	if err := os.MkdirAll(directory, dirPermission); err != nil {
		return fmt.Errorf("create launcher settings directory %s: %w", directory, err)
	}

	data, err := json.MarshalIndent(launcherSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode launcher settings: %w", err)
	}
	data = append(data, '\n')

	temporary, err := os.CreateTemp(directory, ".settings-*.json")
	if err != nil {
		return fmt.Errorf("create launcher settings file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(filePermission); err != nil {
		temporary.Close()
		return fmt.Errorf("set launcher settings permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write launcher settings: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close launcher settings: %w", err)
	}
	if err := os.Rename(temporaryPath, m.path); err != nil {
		return fmt.Errorf("replace launcher settings %s: %w", m.path, err)
	}
	return nil
}

// SetEditor implements the Set editor process.
func (m *Manager) SetEditor(command string) (LauncherSettings, error) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return LauncherSettings{}, errors.New("editor command is required")
	}
	executable, err := editor.Executable(trimmed)
	if err != nil {
		return LauncherSettings{}, fmt.Errorf("invalid editor command %q: %w", trimmed, err)
	}
	if _, err := exec.LookPath(executable); err != nil {
		return LauncherSettings{}, fmt.Errorf("editor %q is not executable: %w", executable, err)
	}

	current, err := m.Load()
	if err != nil {
		return LauncherSettings{}, err
	}
	current.Editor = trimmed
	if err := m.Save(current); err != nil {
		return LauncherSettings{}, err
	}
	return current, nil
}

// SetBaseDirectory implements the Set base directory process.
func (m *Manager) SetBaseDirectory(directory string) (LauncherSettings, error) {
	trimmed := strings.TrimSpace(directory)
	if trimmed == "" {
		return LauncherSettings{}, errors.New("base directory is required")
	}

	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		return LauncherSettings{}, fmt.Errorf("resolve base directory %q: %w", trimmed, err)
	}
	absolute = filepath.Clean(absolute)
	if !filepath.IsAbs(absolute) {
		return LauncherSettings{}, fmt.Errorf("base directory %q is not absolute", directory)
	}
	if info, statErr := os.Stat(absolute); statErr == nil {
		if !info.IsDir() {
			return LauncherSettings{}, fmt.Errorf("base directory %s is not a directory", absolute)
		}
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return LauncherSettings{}, fmt.Errorf("inspect base directory %s: %w", absolute, statErr)
	}

	current, err := m.Load()
	if err != nil {
		return LauncherSettings{}, err
	}
	current.BaseDirectory = absolute
	if err := m.Save(current); err != nil {
		return LauncherSettings{}, err
	}
	return current, nil
}
