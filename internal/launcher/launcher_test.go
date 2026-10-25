package launcher

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/PlotAI/plot-launcher/internal/repository"
	"github.com/PlotAI/plot-launcher/internal/settings"
)

type fakeSettings struct {
	loaded settings.LauncherSettings
	err    error
	calls  int
}

func (f *fakeSettings) Load() (settings.LauncherSettings, error) {
	f.calls++
	return f.loaded, f.err
}

type fakeRepositories struct {
	pullRequest      repository.PullRequest
	baseDirectory    string
	workingDirectory string
	err              error
	calls            int
}

func (f *fakeRepositories) CheckoutPRAndPrepareWorkingDirectory(_ context.Context, pullRequest repository.PullRequest, baseDirectory string) (string, error) {
	f.calls++
	f.pullRequest = pullRequest
	f.baseDirectory = baseDirectory
	return f.workingDirectory, f.err
}

type fakeEditor struct {
	command   string
	directory string
	err       error
	calls     int
}

func (f *fakeEditor) Launch(_ context.Context, command, directory string) error {
	f.calls++
	f.command = command
	f.directory = directory
	return f.err
}

func TestOpenInEditor(t *testing.T) {
	settingsReader := &fakeSettings{loaded: settings.LauncherSettings{BaseDirectory: "/base", Editor: "code --wait"}}
	repositories := &fakeRepositories{workingDirectory: "/base/name"}
	editor := &fakeEditor{}
	plotLauncher := New(settingsReader, repositories, editor)

	pullRequest := repository.PullRequest{Owner: "owner", Name: "name", Number: 7}
	workingDirectory, err := plotLauncher.OpenInEditor(context.Background(), pullRequest)
	if err != nil {
		t.Fatalf("OpenInEditor: %v", err)
	}
	if workingDirectory != repositories.workingDirectory {
		t.Fatalf("working directory = %q, want %q", workingDirectory, repositories.workingDirectory)
	}
	if repositories.pullRequest != pullRequest {
		t.Fatalf("repository pull request = %+v, want %+v", repositories.pullRequest, pullRequest)
	}
	if repositories.baseDirectory != "/base" {
		t.Fatalf("base directory = %q, want %q", repositories.baseDirectory, "/base")
	}
	if editor.calls != 1 {
		t.Fatalf("editor calls = %d, want 1", editor.calls)
	}
	if editor.command != "code --wait" || editor.directory != repositories.workingDirectory {
		t.Fatalf("editor launched with %q in %q", editor.command, editor.directory)
	}
}

func TestOpenInEditorRequiresSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings settings.LauncherSettings
		want     string
	}{
		{
			name:     "base directory",
			settings: settings.LauncherSettings{Editor: "code"},
			want:     "base directory is not set",
		},
		{
			name:     "editor",
			settings: settings.LauncherSettings{BaseDirectory: "/base"},
			want:     "editor is not set",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repositories := &fakeRepositories{}
			editor := &fakeEditor{}
			plotLauncher := New(&fakeSettings{loaded: test.settings}, repositories, editor)

			_, err := plotLauncher.OpenInEditor(context.Background(), repository.PullRequest{Owner: "owner", Name: "name", Number: 7})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("OpenInEditor error = %v, want %q", err, test.want)
			}
			if repositories.calls != 0 || editor.calls != 0 {
				t.Fatalf("repository calls = %d, editor calls = %d, want 0", repositories.calls, editor.calls)
			}
		})
	}
}

func TestOpenInEditorPropagatesErrors(t *testing.T) {
	t.Run("settings", func(t *testing.T) {
		settingsError := errors.New("settings unavailable")
		plotLauncher := New(&fakeSettings{err: settingsError}, &fakeRepositories{}, &fakeEditor{})
		_, err := plotLauncher.OpenInEditor(context.Background(), repository.PullRequest{Owner: "owner", Name: "name", Number: 7})
		if !errors.Is(err, settingsError) {
			t.Fatalf("OpenInEditor error = %v, want %v", err, settingsError)
		}
	})

	t.Run("checkout", func(t *testing.T) {
		checkoutError := errors.New("checkout failed")
		editor := &fakeEditor{}
		plotLauncher := New(
			&fakeSettings{loaded: settings.LauncherSettings{BaseDirectory: "/base", Editor: "code"}},
			&fakeRepositories{err: checkoutError},
			editor,
		)
		_, err := plotLauncher.OpenInEditor(context.Background(), repository.PullRequest{Owner: "owner", Name: "name", Number: 7})
		if !errors.Is(err, checkoutError) {
			t.Fatalf("OpenInEditor error = %v, want %v", err, checkoutError)
		}
		if editor.calls != 0 {
			t.Fatalf("editor calls = %d, want 0", editor.calls)
		}
	})

	t.Run("editor", func(t *testing.T) {
		editorError := errors.New("editor failed")
		plotLauncher := New(
			&fakeSettings{loaded: settings.LauncherSettings{BaseDirectory: "/base", Editor: "code"}},
			&fakeRepositories{workingDirectory: "/base/name"},
			&fakeEditor{err: editorError},
		)
		_, err := plotLauncher.OpenInEditor(context.Background(), repository.PullRequest{Owner: "owner", Name: "name", Number: 7})
		if !errors.Is(err, editorError) {
			t.Fatalf("OpenInEditor error = %v, want %v", err, editorError)
		}
	})
}

func TestOpenInEditorValidatesPullRequest(t *testing.T) {
	settingsReader := &fakeSettings{loaded: settings.LauncherSettings{BaseDirectory: "/base", Editor: "code"}}
	repositories := &fakeRepositories{}
	plotLauncher := New(settingsReader, repositories, &fakeEditor{})

	_, err := plotLauncher.OpenInEditor(context.Background(), repository.PullRequest{Owner: "owner", Name: "name"})
	if err == nil {
		t.Fatal("OpenInEditor with pull request number 0 returned no error")
	}
	if repositories.calls != 0 {
		t.Fatalf("repository calls = %d, want 0", repositories.calls)
	}
}
