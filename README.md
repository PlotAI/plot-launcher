# Plot Launcher

Plot Launcher is the Go/CLI application that opens a pull request in your editor.
A browser sends an **Open In Editor Requested** request to the Protocol Handler,
the GitHub Controller checks out the pull request and prepares its working
directory, and the Editor Launcher starts the configured editor there.

## Install

Releases are published for macOS, Linux, and Windows on both `amd64` and
`arm64`.

macOS and Linux:

```sh
curl -fsSL https://github.com/PlotAI/plot-launcher/releases/latest/download/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://github.com/PlotAI/plot-launcher/releases/latest/download/install.ps1 | iex
```

The installer resolves the latest release (or a pinned `--version`), downloads
the archive for your operating system and architecture, verifies it against the
release's `checksums.txt`, and fails closed on a mismatch. It installs
`plot-launcher` into `~/.local/bin` (`%LOCALAPPDATA%\Plot\bin` on Windows),
replacing an existing installation atomically, then prints the installed version
and the next setup commands.

Pin a version or choose an installation directory:

```sh
curl -fsSL https://github.com/PlotAI/plot-launcher/releases/latest/download/install.sh \
  | sh -s -- --version v0.1.1 --dir "$HOME/bin"
```

`install.sh` accepts `--version <tag>`, `--dir <path>`, and `--help`. It also
honors `PLOT_LAUNCHER_VERSION`, `PLOT_LAUNCHER_INSTALL_DIR`,
`PLOT_LAUNCHER_REPOSITORY`, `PLOT_LAUNCHER_RELEASE_BASE_URL` (internal mirror
serving the assets of one tag), `GITHUB_TOKEN`/`GH_TOKEN` (private repository),
and `NO_COLOR`.

When the release repository is private, download the installer with the GitHub
CLI and give it a token so it can read the release assets:

```sh
gh release download v0.1.1 --repo PlotAI/plot-launcher --pattern install.sh
GITHUB_TOKEN=$(gh auth token) sh install.sh
```

Verifying by hand works too:

```sh
curl -fsSLO https://github.com/PlotAI/plot-launcher/releases/download/v0.1.1/plot-launcher_0.1.1_darwin_arm64.tar.gz
curl -fsSLO https://github.com/PlotAI/plot-launcher/releases/download/v0.1.1/checksums.txt
grep plot-launcher_0.1.1_darwin_arm64.tar.gz checksums.txt
shasum -a 256 plot-launcher_0.1.1_darwin_arm64.tar.gz
```

Upgrade by running the installer again. Uninstall by deleting the binary, for
example `rm ~/.local/bin/plot-launcher`.

Check what is installed:

```sh
plot-launcher version
```

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

Launcher Settings live in the user configuration directory: for example
`~/.config/plot-launcher/settings.json` on Linux,
`~/Library/Application Support/plot-launcher/settings.json` on macOS, and
`%AppData%\plot-launcher\settings.json` on Windows. Set
`PLOT_LAUNCHER_CONFIG_DIR` to keep them somewhere else, for example in portable
or test installations:

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

Then request a pull request by URL:

```text
GET http://localhost:8765/open?url=<https://github.com/owner/name/pull/number>
```

or by repository and number:

```text
GET http://localhost:8765/open?repository=<owner/name>&pull_request=<number>
```

A `url` request must name a `github.com` pull request; trailing path segments
and query values, such as the links to a pull request's files or commits, are
accepted. Supplying both forms in one request is rejected.

An operating system protocol invocation passes the launcher URL directly to the
CLI:

```sh
plot-launcher "plot-launcher://open?url=https%3A%2F%2Fgithub.com%2Fowner%2Fname%2Fpull%2F42"
plot-launcher "plot-launcher://open?repository=owner/name&pull_request=42"
plot-launcher open --repository owner/name --pull-request 42
```

Successful requests return the prepared working directory as JSON.

## Releases

Releases are cut from a tag, and CI publishes them:

1. Merge the change to `main`.
2. Tag and push:

   ```sh
   git tag -a v0.2.0 -m "v0.2.0"
   git push origin v0.2.0
   ```

3. `.github/workflows/release.yml` runs the tests and then
   [GoReleaser](https://goreleaser.com) publishes a release containing
   `plot-launcher_<version>_{darwin,linux,windows}_{amd64,arm64}` archives,
   `checksums.txt`, the `install.sh` and `install.ps1` belonging to that tag, and
   a changelog built from conventional commits. The workflow fails if the tests
   fail, so a broken tag never produces a downloadable release.
4. Confirm the workflow is green and that the install commands in this README
   work against the published tag.

Release builds inject the tag, commit, and commit date with `-ldflags`, which is
what `plot-launcher version` reports. Reproduce the artifacts locally with:

```sh
goreleaser release --snapshot --clean --skip=publish
```

Hardening to apply before distributing to external customers:

- Sign the macOS binaries with a Developer ID certificate and notarize them
  (GoReleaser `notarize`), then ship a `.pkg` or `.dmg` so Gatekeeper accepts the
  download even when a browser adds the quarantine attribute.
- Set the repository variable `ENABLE_ARTIFACT_ATTESTATIONS` to `true` to have
  the release workflow attest `checksums.txt` with
  `actions/attest-build-provenance`; customers can then run
  `gh attestation verify`.
- Serve the installers from a stable download domain instead of
  `raw.githubusercontent.com`, and keep the published URL stable across
  releases.
- Keep the release repository public, or require `GITHUB_TOKEN` for downloads as
  the installer already does for private repositories.
