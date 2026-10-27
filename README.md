# Plot Launcher

Opens a Plot pull request in your editor.

## Install

macOS and Linux:

```sh
curl -fsSL https://github.com/PlotAI/plot-launcher/releases/latest/download/install.sh | sh
```

Windows (PowerShell):

```powershell
Invoke-RestMethod https://github.com/PlotAI/plot-launcher/releases/latest/download/install.ps1 | Invoke-Expression
```

## Uninstall

macOS:

```sh
sudo rm -rf "/Applications/Plot Launcher.app"
rm -f ~/.local/bin/plot-launcher
```

Linux:

```sh
rm -f ~/.local/bin/plot-launcher ~/.local/share/applications/plot.desktop
```

Windows (PowerShell):

```powershell
Remove-Item -Recurse -Force "$env:LOCALAPPDATA\Plot\bin"
Remove-Item -Recurse -Force "HKCU:\Software\Classes\plot"
```

## Preferences

```sh
plot-launcher settings set-base-directory ~/plot-repositories
plot-launcher settings set-editor "/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code --reuse-window"
plot-launcher settings show
```

`set-base-directory` is the folder Plot checks repositories out into, one directory per repository. Use an absolute path: a relative one is resolved against the current directory when you set it, not when a link arrives, and the folder is created on first use.

`set-editor` is the command that opens the checkout. Give an absolute path to the executable and quote the whole command if it contains spaces. A browser link starts the launcher without your shell `PATH`, so a bare `code` that works in your terminal will not be found.

Settings live in `~/.config/plot-launcher/settings.json` on Linux, `~/Library/Application Support/plot-launcher/settings.json` on macOS, and `%AppData%\plot-launcher\settings.json` on Windows. Set `PLOT_LAUNCHER_CONFIG_DIR` to keep them elsewhere.

## Opening pull requests

A link checks the pull request out below the base directory, one directory per repository, on a `plot/pr-<number>` branch, and opens the editor there. Opening another pull request of the same repository switches that directory's branch.

Work in that directory is never a reason to refuse the checkout:

- Uncommitted changes, untracked files included, are stashed, and restored on the pull request. Changes that conflict with the pull request stay in a stash entry named `plot: changes on <branch> before opening <owner/name>#<number>`, and the result names the entry.
- Commits made on the `plot/pr-<number>` branch are moved to `plot/pr-<number>-local` before the branch is reset to the pull request head.

A repository that was renamed or transferred is followed: `origin` is pointed at the repository the link names, so an SSH remote stays an SSH remote. Any other remote is reported instead of being replaced.

A browser navigation to `/open` renders a page saying what happened, with the command that repairs a failure; a client that asks for JSON, as Plot does, gets `error`, `code`, `remedy`, and `command` instead. The macOS app also appends standard error to `~/Library/Logs/plot-launcher.log`.
