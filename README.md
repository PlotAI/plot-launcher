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
rm -rf ~/Applications/"Plot Launcher.app" ~/.local/bin/plot-launcher
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
plot-launcher settings set-base-directory ~/plot-workspaces
plot-launcher settings set-editor "/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code --reuse-window"
plot-launcher settings show
```

Settings live in `~/.config/plot-launcher/settings.json` on Linux, `~/Library/Application Support/plot-launcher/settings.json` on macOS, and `%AppData%\plot-launcher\settings.json` on Windows. Set `PLOT_LAUNCHER_CONFIG_DIR` to keep them elsewhere.

Use an absolute editor path: a link opened from a browser runs without your shell `PATH`.
