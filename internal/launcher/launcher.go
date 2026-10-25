// Package launcher implements the Plot Launcher application component: it
// receives an open request, checks out the pull request, and launches the
// editor.
package launcher

import (
	"context"
	"errors"
	"fmt"

	"github.com/PlotAI/plot-launcher/internal/repository"
	"github.com/PlotAI/plot-launcher/internal/settings"
)

// SettingsReader reads Launcher Settings.
type SettingsReader interface {
	Load() (settings.LauncherSettings, error)
}

// EditorLauncher launches the configured editor.
type EditorLauncher interface {
	Launch(ctx context.Context, command, directory string) error
}

// Launcher is the Plot Launcher.
type Launcher struct {
	settings     SettingsReader
	repositories repository.Controller
	editors      EditorLauncher
}

// New returns a Plot Launcher.
func New(settingsReader SettingsReader, repositories repository.Controller, editors EditorLauncher) *Launcher {
	return &Launcher{settings: settingsReader, repositories: repositories, editors: editors}
}

// OpenInEditor checks out the pull request, prepares its working directory,
// and launches the editor there. It returns the prepared working directory.
func (l *Launcher) OpenInEditor(ctx context.Context, pullRequest repository.PullRequest) (string, error) {
	if err := pullRequest.Validate(); err != nil {
		return "", err
	}

	launcherSettings, err := l.settings.Load()
	if err != nil {
		return "", err
	}
	if launcherSettings.BaseDirectory == "" {
		return "", errors.New("base directory is not set: run plot-launcher settings set-base-directory <path>")
	}
	if launcherSettings.Editor == "" {
		return "", errors.New("editor is not set: run plot-launcher settings set-editor <command>")
	}

	workingDirectory, err := l.repositories.CheckoutPRAndPrepareWorkingDirectory(ctx, pullRequest, launcherSettings.BaseDirectory)
	if err != nil {
		return "", fmt.Errorf("checkout pull request %s#%d: %w", pullRequest.Repository(), pullRequest.Number, err)
	}
	if err := l.editors.Launch(ctx, launcherSettings.Editor, workingDirectory); err != nil {
		return "", err
	}
	return workingDirectory, nil
}
