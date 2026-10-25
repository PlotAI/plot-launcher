# Plot Launcher

Plot Launcher is the Go/CLI companion to Plot: it opens a pull request in your editor from a `plot://` browser link or from the loopback Protocol Handler on `http://127.0.0.1:8765`.

## Install

Published for macOS, Linux, and Windows on `amd64` and `arm64`.

macOS and Linux:

```sh
curl -fsSL https://github.com/PlotAI/plot-launcher/releases/latest/download/install.sh | sh
```

Windows (PowerShell):

```powershell
Invoke-RestMethod https://github.com/PlotAI/plot-launcher/releases/latest/download/install.ps1 | Invoke-Expression
```

Both installers verify the archive against the release `checksums.txt` and install `plot-launcher` into `~/.local/bin` (`%LOCALAPPDATA%\Plot\bin` on Windows). Re-run the installer to upgrade, or delete the binary to uninstall.

Pin a release or choose a directory:

```sh
curl -fsSL https://github.com/PlotAI/plot-launcher/releases/latest/download/install.sh \
  | sh -s -- --version v0.2.0 --dir "$HOME/bin"
```

`install.sh` also accepts `--help` and honors `PLOT_LAUNCHER_VERSION`, `PLOT_LAUNCHER_INSTALL_DIR`, `PLOT_LAUNCHER_REPOSITORY`, `PLOT_LAUNCHER_RELEASE_BASE_URL`, `GITHUB_TOKEN`/`GH_TOKEN`, and `NO_COLOR`. For a private repository, fetch the installer with `gh release download v0.2.0 --repo PlotAI/plot-launcher --pattern install.sh` and run it with a token.

## Settings

Settings live in `~/.config/plot-launcher/settings.json` on Linux, `~/Library/Application Support/plot-launcher/settings.json` on macOS, and `%AppData%\plot-launcher\settings.json` on Windows. Set `PLOT_LAUNCHER_CONFIG_DIR` to store them elsewhere.

```sh
plot-launcher settings set-base-directory ~/plot-workspaces
plot-launcher settings set-editor "code --reuse-window"
plot-launcher settings show
```

Each repository keeps one directory below the base directory, with one branch per pull request, so git switches between pull requests for you. Git authentication comes from your git credential configuration.

## Browser links

```text
plot://open?url=<pull request URL>
```

The installers register that scheme, so a link opens a pull request without the page knowing the loopback address or port. macOS cannot register a scheme for a bare binary, so call `plot-launcher "plot://open?url=<pull request URL>"` there.

## Build and test

```sh
go build -o bin/plot-launcher ./cmd/plot-launcher
go test ./...
```

Go 1.27.3 and `git` are required.
