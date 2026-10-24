// Package cli implements the Go/CLI interface that realizes the Plot Launcher.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
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
	defaultHost = "::"
	defaultPort = 8765
)

const usage = `Plot Launcher

Usage:
  plot-launcher <command> [arguments]

Commands:
  open <launcher-url>                        Receive an Open In Editor Requested request
  open --repository <owner/name> --pull-request <number>
                                             Receive an open request from flags
  serve [--host <host>] [--port <port>]      Run the Protocol Handler for browser requests
  settings show                              Print Launcher Settings as JSON
  settings set-editor <command>              Set editor
  settings set-base-directory <path>         Set base directory
  version                                    Print the Plot Launcher version

Launcher URL:
  plot-launcher://open?repository=<owner/name>&pull_request=<number>
`

// Runner executes Plot Launcher commands.
type Runner struct {
	stdout   io.Writer
	stderr   io.Writer
	logger   *slog.Logger
	settings *settings.Manager
	launcher *launcher.Launcher
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
	}, nil
}

// Run executes args and returns the process exit code.
func (r *Runner) Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(r.stderr, usage)
		return 2
	}

	switch args[0] {
	case "open":
		return r.runOpen(ctx, args[1:])
	case "serve":
		return r.runServe(ctx, args[1:])
	case "settings":
		return r.runSettings(args[1:])
	case "version", "-v", "--version":
		fmt.Fprintf(r.stdout, "plot-launcher %s\n", version.String())
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(r.stdout, usage)
		return 0
	default:
		// An operating system protocol invocation passes the launcher URL as
		// the first argument.
		if strings.HasPrefix(args[0], protocol.Scheme+":") {
			return r.runOpen(ctx, args)
		}
		fmt.Fprintf(r.stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func (r *Runner) runOpen(ctx context.Context, args []string) int {
	flags := flag.NewFlagSet("open", flag.ContinueOnError)
	flags.SetOutput(r.stderr)
	repositoryFlag := flags.String("repository", "", "repository as owner/name")
	pullRequestFlag := flags.Int("pull-request", 0, "pull request number")
	if err := flags.Parse(args); err != nil {
		return flagExitCode(err)
	}

	var pullRequest repository.PullRequest
	switch flags.NArg() {
	case 0:
		if *repositoryFlag == "" && *pullRequestFlag == 0 {
			fmt.Fprintln(r.stderr, "open requires a launcher URL or --repository and --pull-request")
			return 2
		}
		resolved, err := repository.NewPullRequest(*repositoryFlag, *pullRequestFlag)
		if err != nil {
			fmt.Fprintf(r.stderr, "open: %v\n", err)
			return 2
		}
		pullRequest = resolved
	case 1:
		if *repositoryFlag != "" || *pullRequestFlag != 0 {
			fmt.Fprintln(r.stderr, "open accepts either a launcher URL or --repository and --pull-request, not both")
			return 2
		}
		openRequest, err := protocol.ParseURL(flags.Arg(0))
		if err != nil {
			fmt.Fprintf(r.stderr, "open: %v\n", err)
			return 2
		}
		resolved, err := openRequest.PullRequest()
		if err != nil {
			fmt.Fprintf(r.stderr, "open: %v\n", err)
			return 2
		}
		pullRequest = resolved
	default:
		fmt.Fprintln(r.stderr, "open accepts a single launcher URL")
		return 2
	}

	workingDirectory, err := r.launcher.OpenInEditor(ctx, pullRequest)
	if err != nil {
		fmt.Fprintf(r.stderr, "open: %v\n", err)
		return 1
	}
	return r.writeJSON(protocol.NewOpenResult(pullRequest, workingDirectory))
}

func (r *Runner) runServe(ctx context.Context, args []string) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(r.stderr)
	host := flags.String("host", defaultHost, "host to listen on")
	port := flags.Int("port", defaultPort, "port to listen on")
	if err := flags.Parse(args); err != nil {
		return flagExitCode(err)
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(r.stderr, "serve accepts no positional arguments")
		return 2
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	listener, err := protocol.Listen(*host, *port)
	if err != nil {
		fmt.Fprintf(r.stderr, "serve: %v\n", err)
		return 1
	}
	r.logger.Info("protocol handler listening",
		"address", listener.Addr().String(),
		"version", version.Version,
		"event", protocol.EventOpenInEditorRequested,
	)

	server := protocol.NewServer(protocol.NewHandler(r.launcher, r.logger))
	if err := protocol.Serve(ctx, listener, server); err != nil {
		fmt.Fprintf(r.stderr, "serve: %v\n", err)
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

func flagExitCode(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	return 2
}
