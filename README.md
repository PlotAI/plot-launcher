# Plot Launcher

Plot Launcher is the Go/CLI application that opens a pull request in your editor.
A browser sends an **Open In Editor Requested** request to the Protocol Handler,
the GitHub Controller checks out the pull request and prepares its working
directory, and the Editor Launcher starts the configured editor there.

## Components

- **Plot Launcher** (`internal/launcher`) orchestrates the open flow.
- **Protocol Handler** (`internal/protocol`) receives browser requests over HTTP
  and `plot-launcher://` URLs. `Open In Editor Requested` triggers the flow.
- **Repository Controller** (`internal/repository`) checks out a pull request and
  prepares its working directory. The **GitHub Controller** is its GitHub
  specialization and uses `git` to fetch `refs/pull/<number>/head`.
- **Editor Launcher** (`internal/editor`) starts the configured editor with the
  prepared working directory as its argument.
- **Settings Manager** (`internal/settings`) implements *Set editor* and *Set base
  directory* and persists the **Launcher Settings** data object as JSON.

## Build and test

```sh
go build -o bin/plot-launcher ./cmd/plot-launcher
go test ./...
```

Go 1.27.3 and `git` are required.

## Settings

Launcher Settings live in the user configuration directory, for example
`~/.config/plot-launcher/settings.json`:

```sh
plot-launcher settings set-base-directory ~/plot-workspaces
plot-launcher settings set-editor "code --reuse-window"
plot-launcher settings show
```

```json
{
  "base_directory": "/home/you/plot-workspaces",
  "editor": "code --reuse-window"
}
```

A pull request is prepared under
`<base_directory>/<owner>/<name>/pull-<number>` on branch `plot/pr-<number>`.
Git authentication is provided by the environment's git credential
configuration.

## Open In Editor Requested

Run the Protocol Handler for browser requests:

```sh
plot-launcher serve --host :: --port 8765
```

Then request:

```text
GET http://localhost:8765/open?repository=<owner/name>&pull_request=<number>
```

An operating system protocol invocation passes the launcher URL directly to the
CLI:

```sh
plot-launcher "plot-launcher://open?repository=owner/name&pull_request=42"
plot-launcher open --repository owner/name --pull-request 42
```

Successful requests return the prepared working directory as JSON.
