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
