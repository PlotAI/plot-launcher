// Package cli implements the Go/CLI interface that realizes the Plot Launcher.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/PlotAI/plot-launcher/internal/editor"
	"github.com/PlotAI/plot-launcher/internal/launcher"
	"github.com/PlotAI/plot-launcher/internal/protocol"
	"github.com/PlotAI/plot-launcher/internal/repository"
	"github.com/PlotAI/plot-launcher/internal/settings"
	"github.com/PlotAI/plot-launcher/internal/version"
)

const (
	// Plot Launcher is a local companion to Plot, so it always listens where
	// Plot expects to find it.
	listenHost = "127.0.0.1"
	listenPort = 8765
)

const usage = `Plot Launcher

Usage:
  plot-launcher [command]

Run plot-launcher without a command to start the Protocol Handler on
http://127.0.0.1:8765. It runs in the foreground until it is stopped.

Commands:
  serve                                      Start the Protocol Handler (default)
  settings show                              Print Launcher Settings as JSON
  settings set-editor <command>              Set editor
  settings set-base-directory <path>         Set base directory
  version                                    Print the Plot Launcher version
`

// Runner executes Plot Launcher commands.
type Runner struct {
	stdout   io.Writer
	stderr   io.Writer
	logger   *slog.Logger
	settings *settings.Manager
	launcher *launcher.Launcher
	listen   func() (net.Listener, error)
}

// NewRunner returns a Runner backed by the user's Launcher Settings, the
// GitHub Controller, and the Editor Launcher.
func NewRunner(stdout, stderr io.Writer) (*Runner, error) {
	settingsManager, err := settings.NewDefaultManager()
	if err != nil {
		return nil, err
	}
	logger := slog.New(slog.NewTextHandler(stderr, nil))
	return &Runner{
		stdout:   stdout,
		stderr:   stderr,
		logger:   logger,
		settings: settingsManager,
		launcher: launcher.New(settingsManager, repository.NewGitHubController(), editor.NewLauncher(stdout, stderr)),
		listen: func() (net.Listener, error) {
			return protocol.Listen(listenHost, listenPort)
		},
	}, nil
}

// Run executes args and returns the process exit code. Running plot-launcher
// without a command starts the Protocol Handler.
func (r *Runner) Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return r.runServe(ctx)
	}

	switch args[0] {
	case "serve":
		if len(args) != 1 {
			fmt.Fprintln(r.stderr, "serve accepts no arguments")
			return 2
		}
		return r.runServe(ctx)
	case "settings":
		return r.runSettings(args[1:])
	case "version", "-v", "--version":
		fmt.Fprintf(r.stdout, "plot-launcher %s\n", version.String())
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(r.stdout, usage)
		return 0
	default:
		fmt.Fprintf(r.stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func (r *Runner) runServe(ctx context.Context) int {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	listener, err := r.listen()
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			fmt.Fprintf(r.stderr, "plot-launcher: another Plot Launcher is already listening on http://%s:%d\n", listenHost, listenPort)
			return 1
		}
		fmt.Fprintf(r.stderr, "plot-launcher: %v\n", err)
		return 1
	}
	r.logger.Info("plot launcher listening",
		"address", listener.Addr().String(),
		"version", version.Version,
		"event", protocol.EventOpenInEditorRequested,
	)

	server := protocol.NewServer(protocol.NewHandler(r.launcher, r.logger))
	if err := protocol.Serve(ctx, listener, server); err != nil {
		fmt.Fprintf(r.stderr, "plot-launcher: %v\n", err)
		return 1
	}
	return 0
}

func (r *Runner) runSettings(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.stderr, "settings requires show, set-editor, or set-base-directory")
		return 2
	}

	switch args[0] {
	case "show":
		if len(args) != 1 {
			fmt.Fprintln(r.stderr, "settings show accepts no arguments")
			return 2
		}
		launcherSettings, err := r.settings.Load()
		if err != nil {
			fmt.Fprintf(r.stderr, "settings show: %v\n", err)
			return 1
		}
		return r.writeJSON(launcherSettings)

	case "set-editor":
		if len(args) < 2 {
			fmt.Fprintln(r.stderr, "settings set-editor requires an editor command")
			return 2
		}
		updated, err := r.settings.SetEditor(strings.Join(args[1:], " "))
		if err != nil {
			fmt.Fprintf(r.stderr, "settings set-editor: %v\n", err)
			return 1
		}
		return r.writeJSON(updated)

	case "set-base-directory":
		if len(args) != 2 {
			fmt.Fprintln(r.stderr, "settings set-base-directory requires one directory")
			return 2
		}
		updated, err := r.settings.SetBaseDirectory(args[1])
		if err != nil {
			fmt.Fprintf(r.stderr, "settings set-base-directory: %v\n", err)
			return 1
		}
		return r.writeJSON(updated)

	default:
		fmt.Fprintf(r.stderr, "unknown settings command %q\n", args[0])
		return 2
	}
}

func (r *Runner) writeJSON(value any) int {
	encoder := json.NewEncoder(r.stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintf(r.stderr, "write output: %v\n", err)
		return 1
	}
	return 0
}
