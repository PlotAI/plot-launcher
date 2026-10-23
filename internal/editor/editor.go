// Package editor implements the Editor Launcher: it starts the configured
// editor for a prepared working directory.
package editor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Launcher starts an editor command in a working directory.
type Launcher struct {
	stdout io.Writer
	stderr io.Writer
}

// NewLauncher returns an Editor Launcher whose editor process inherits the
// supplied output streams.
func NewLauncher(stdout, stderr io.Writer) *Launcher {
	return &Launcher{stdout: stdout, stderr: stderr}
}

// Launch starts command with the working directory as its final argument.
// The editor is detached: startup is reported as soon as the process is
// running, so a browser request never waits for the editor to exit.
func (l *Launcher) Launch(ctx context.Context, command, directory string) error {
	args, err := ParseCommand(command)
	if err != nil {
		return fmt.Errorf("parse editor command %q: %w", command, err)
	}
	if directory == "" {
		return errors.New("working directory is required to launch the editor")
	}

	// The request context ends when the browser request is answered, so it
	// must not kill the detached editor process.
	cmd := exec.CommandContext(context.WithoutCancel(ctx), args[0], append(args[1:], directory)...)
	cmd.Dir = directory
	cmd.Stdout = l.stdout
	cmd.Stderr = l.stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch editor %q in %s: %w", command, directory, err)
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("release editor process: %w", err)
	}
	return nil
}

// Executable returns the executable named by an editor command after
// validating it.
func Executable(command string) (string, error) {
	args, err := ParseCommand(command)
	if err != nil {
		return "", err
	}
	return args[0], nil
}

// ParseCommand splits an editor command into arguments. Arguments are
// separated by unquoted whitespace; single and double quotes group text, and
// a backslash escapes the following character outside single quotes.
func ParseCommand(command string) ([]string, error) {
	var (
		args    []string
		current strings.Builder
		quote   rune
		escaped bool
		started bool
	)

	flush := func() {
		args = append(args, current.String())
		current.Reset()
		started = false
	}

	for _, r := range command {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
			started = true
		case quote == '\'':
			if r == '\'' {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			started = true
		case quote == '"':
			switch r {
			case '"':
				quote = 0
			case '\\':
				escaped = true
			default:
				current.WriteRune(r)
			}
			started = true
		case r == '\\':
			escaped = true
			started = true
		case r == '\'' || r == '"':
			quote = r
			started = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if started {
				flush()
			}
		default:
			current.WriteRune(r)
			started = true
		}
	}

	if escaped {
		return nil, errors.New("unterminated escape")
	}
	if quote != 0 {
		return nil, errors.New("unterminated quote")
	}
	if started {
		flush()
	}
	if len(args) == 0 {
		return nil, errors.New("command is empty")
	}
	return args, nil
}
