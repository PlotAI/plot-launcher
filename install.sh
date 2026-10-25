#!/bin/sh
#
# Plot Launcher installer for macOS and Linux.
#
#   curl -fsSL https://github.com/PlotAI/plot-launcher/releases/latest/download/install.sh | sh
#
# Options:
#   --version <tag>   Install a specific release (default: latest)
#   --dir <path>      Install the binary into <path> (default: ~/.local/bin)
#   -h, --help        Show this help
#
# Environment:
#   PLOT_LAUNCHER_VERSION      Same as --version
#   PLOT_LAUNCHER_INSTALL_DIR  Same as --dir
#   PLOT_LAUNCHER_REPOSITORY   Release repository (default: PlotAI/plot-launcher)
#   PLOT_LAUNCHER_RELEASE_BASE_URL  Mirror serving release assets for a tag;
#                              requires an explicit --version
#   GITHUB_TOKEN / GH_TOKEN    Token used when the repository is private
#   NO_COLOR                   Disable colored output
#
# The archive is verified against the release's checksums.txt before the binary
# is installed, and an existing installation is replaced atomically.

set -eu

REPOSITORY=${PLOT_LAUNCHER_REPOSITORY:-PlotAI/plot-launcher}
VERSION=${PLOT_LAUNCHER_VERSION:-latest}
INSTALL_DIR=${PLOT_LAUNCHER_INSTALL_DIR:-"$HOME/.local/bin"}
RELEASE_BASE_URL=${PLOT_LAUNCHER_RELEASE_BASE_URL:-}
TOKEN=${GITHUB_TOKEN:-${GH_TOKEN:-}}
BINARY=plot-launcher

if [ -t 2 ] && [ -z "${NO_COLOR:-}" ]; then
	bold=$(printf '\033[1m')
	dim=$(printf '\033[2m')
	red=$(printf '\033[31m')
	green=$(printf '\033[32m')
	reset=$(printf '\033[0m')
else
	bold=''
	dim=''
	red=''
	green=''
	reset=''
fi

usage() {
	cat <<'EOF'
Plot Launcher installer for macOS and Linux.

Usage:
  install.sh [--version <tag>] [--dir <path>]

Options:
  --version <tag>   Install a specific release (default: latest)
  --dir <path>      Install the binary into <path> (default: ~/.local/bin)
  -h, --help        Show this help

Environment:
  PLOT_LAUNCHER_VERSION      Same as --version
  PLOT_LAUNCHER_INSTALL_DIR  Same as --dir
  PLOT_LAUNCHER_REPOSITORY   Release repository (default: PlotAI/plot-launcher)
  PLOT_LAUNCHER_RELEASE_BASE_URL  Mirror serving release assets for a tag;
                             requires an explicit --version
  GITHUB_TOKEN / GH_TOKEN    Token used when the repository is private
  NO_COLOR                   Disable colored output

Examples:
  curl -fsSL https://github.com/PlotAI/plot-launcher/releases/latest/download/install.sh | sh
  curl -fsSL https://github.com/PlotAI/plot-launcher/releases/latest/download/install.sh | sh -s -- --version v0.1.0
EOF
}

step() {
	printf '%s→%s %s\n' "$dim" "$reset" "$*" >&2
}

success() {
	printf '%s✓%s %s\n' "$green" "$reset" "$*" >&2
}

warn() {
	printf '%s!%s %s\n' "$bold" "$reset" "$*" >&2
}

fail() {
	printf '%s✗%s %s\n' "$red" "$reset" "$*" >&2
	exit 1
}

while [ $# -gt 0 ]; do
	case $1 in
	--version)
		[ $# -ge 2 ] || fail "--version requires a value"
		VERSION=$2
		shift 2
		;;
	--version=*)
		VERSION=${1#*=}
		shift
		;;
	--dir)
		[ $# -ge 2 ] || fail "--dir requires a value"
		INSTALL_DIR=$2
		shift 2
		;;
	--dir=*)
		INSTALL_DIR=${1#*=}
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		fail "unknown option: $1 (use --help)"
		;;
	esac
done

[ -n "$VERSION" ] || fail "a version is required"
[ -n "$INSTALL_DIR" ] || fail "an installation directory is required"

if command -v curl >/dev/null 2>&1; then
	DOWNLOADER=curl
elif command -v wget >/dev/null 2>&1; then
	DOWNLOADER=wget
else
	fail "curl or wget is required to download Plot Launcher"
fi

fetch() {
	url=$1
	if [ "$DOWNLOADER" = curl ]; then
		if [ -n "$TOKEN" ]; then
			curl -fsSL --retry 3 --connect-timeout 15 -H "Authorization: Bearer $TOKEN" -H "Accept: application/vnd.github+json" "$url"
		else
			curl -fsSL --retry 3 --connect-timeout 15 -H "Accept: application/vnd.github+json" "$url"
		fi
	else
		if [ -n "$TOKEN" ]; then
			wget -q -O - --header="Authorization: Bearer $TOKEN" --header="Accept: application/vnd.github+json" "$url"
		else
			wget -q -O - --header="Accept: application/vnd.github+json" "$url"
		fi
	fi
}

download() {
	url=$1
	output=$2
	if [ -n "$TOKEN" ] && [ -z "$RELEASE_BASE_URL" ]; then
		if [ "$DOWNLOADER" = curl ]; then
			curl -fsSL --retry 3 --connect-timeout 15 -H "Authorization: Bearer $TOKEN" -o "$output" "$url"
		else
			wget -q -O "$output" --header="Authorization: Bearer $TOKEN" "$url"
		fi
	else
		if [ "$DOWNLOADER" = curl ]; then
			curl -fsSL --retry 3 --connect-timeout 15 -o "$output" "$url"
		else
			wget -q -O "$output" "$url"
		fi
	fi
}

# download_api fetches a release asset through the API, which is the only way
# GitHub serves assets of a private repository to a token.
download_api() {
	url=$1
	output=$2
	if [ "$DOWNLOADER" = curl ]; then
		curl -fsSL --retry 3 --connect-timeout 15 -H "Authorization: Bearer $TOKEN" -H "Accept: application/octet-stream" -o "$output" "$url"
	else
		wget -q -O "$output" --header="Authorization: Bearer $TOKEN" --header="Accept: application/octet-stream" "$url"
	fi
}

# asset_id prints the release asset identifier for an asset name.
asset_id() {
	printf '%s\n' "$RELEASE_JSON" | awk -v asset="$1" '
		match($0, /releases\/assets\/[0-9]+/) {
			id = substr($0, RSTART + 16, RLENGTH - 16)
		}
		/"name":/ {
			name = $0
			sub(/.*"name": *"/, "", name)
			sub(/".*/, "", name)
			if (name == asset && id != "") {
				print id
				exit
			}
			id = ""
		}
	'
}

case $(uname -s) in
Darwin) os=darwin ;;
Linux) os=linux ;;
*)
	fail "unsupported operating system: $(uname -s). On Windows, run install.ps1 instead."
	;;
esac

case $(uname -m) in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) fail "unsupported architecture: $(uname -m)" ;;
esac

RELEASE_JSON=
if [ "$VERSION" = latest ]; then
	if [ -n "$RELEASE_BASE_URL" ]; then
		fail "PLOT_LAUNCHER_RELEASE_BASE_URL requires an explicit --version"
	fi
	step "Resolving the latest Plot Launcher release"
	RELEASE_JSON=$(fetch "https://api.github.com/repos/$REPOSITORY/releases/latest") ||
		fail "could not query releases for $REPOSITORY. If the repository is private, set GITHUB_TOKEN."
	VERSION=$(printf '%s\n' "$RELEASE_JSON" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)
	[ -n "$VERSION" ] || fail "could not determine the latest version of $REPOSITORY"
fi

case $VERSION in
v*) TAG=$VERSION ;;
*) TAG=v$VERSION ;;
esac
VERSION_NUMBER=${TAG#v}

ASSET="plot-launcher_${VERSION_NUMBER}_${os}_${arch}.tar.gz"
BASE_URL=${RELEASE_BASE_URL:-"https://github.com/$REPOSITORY/releases/download/$TAG"}

if [ -z "$RELEASE_BASE_URL" ] && [ -n "$TOKEN" ] && [ -z "$RELEASE_JSON" ]; then
	RELEASE_JSON=$(fetch "https://api.github.com/repos/$REPOSITORY/releases/tags/$TAG") ||
		fail "could not read release $TAG of $REPOSITORY"
fi

TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/plot-launcher-install.XXXXXX")
STAGED=
cleanup() {
	rm -rf "$TMP_DIR"
	if [ -n "$STAGED" ]; then
		rm -f "$STAGED"
	fi
}
trap cleanup EXIT HUP INT TERM

download_asset() {
	asset=$1
	if [ -n "$TOKEN" ] && [ -z "$RELEASE_BASE_URL" ]; then
		id=$(asset_id "$asset")
		[ -n "$id" ] || fail "release $TAG does not contain $asset"
		download_api "https://api.github.com/repos/$REPOSITORY/releases/assets/$id" "$TMP_DIR/$asset" && return 0
	elif download "$BASE_URL/$asset" "$TMP_DIR/$asset" 2>/dev/null; then
		return 0
	fi
	if command -v gh >/dev/null 2>&1; then
		step "Downloading $asset with the GitHub CLI"
		if gh release download "$TAG" --repo "$REPOSITORY" --pattern "$asset" --dir "$TMP_DIR" --clobber >/dev/null 2>&1; then
			return 0
		fi
	fi
	return 1
}

step "Downloading Plot Launcher $TAG for $os/$arch"
download_asset "$ASSET" ||
	fail "could not download $ASSET. If the repository is private, set GITHUB_TOKEN."
download_asset checksums.txt ||
	fail "could not download checksums.txt. If the repository is private, set GITHUB_TOKEN."

expected=$(awk -v name="$ASSET" '$2 == name { print $1 }' "$TMP_DIR/checksums.txt" | head -n 1)
[ -n "$expected" ] || fail "checksums.txt does not list $ASSET"

if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$TMP_DIR/$ASSET" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "$TMP_DIR/$ASSET" | awk '{ print $1 }')
else
	fail "sha256sum or shasum is required to verify the download"
fi
[ "$expected" = "$actual" ] || fail "checksum verification failed for $ASSET"
step "Verified SHA-256 checksum"

tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR" || fail "could not extract $ASSET"
[ -f "$TMP_DIR/$BINARY" ] || fail "$ASSET did not contain $BINARY"

mkdir -p "$INSTALL_DIR" || fail "could not create $INSTALL_DIR"
TARGET="$INSTALL_DIR/$BINARY"
STAGED="$INSTALL_DIR/.$BINARY.$$"
if cp "$TMP_DIR/$BINARY" "$STAGED" && chmod 0755 "$STAGED"; then
	:
else
	fail "could not write to $INSTALL_DIR. Choose a writable directory with --dir."
fi
mv -f "$STAGED" "$TARGET" || fail "could not replace $TARGET"
STAGED=

if [ "$os" = darwin ] && command -v xattr >/dev/null 2>&1; then
	xattr -d com.apple.quarantine "$TARGET" 2>/dev/null || true
fi

# A browser link reaches the launcher through the operating system, so the web
# page needs no knowledge of the loopback address or port.
register_scheme() {
	applications="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
	desktop="$applications/plot-launcher.desktop"
	if ! mkdir -p "$applications" 2>/dev/null || ! cat >"$desktop" 2>/dev/null <<EOF
[Desktop Entry]
Type=Application
Name=Plot Launcher
Exec="$TARGET" %u
Terminal=false
MimeType=x-scheme-handler/plot-launcher
EOF
	then
		warn "could not write $desktop, so plot-launcher:// links stay unregistered"
		return
	fi
	if command -v xdg-mime >/dev/null 2>&1; then
		xdg-mime default plot-launcher.desktop x-scheme-handler/plot-launcher 2>/dev/null ||
			warn "could not make Plot Launcher the plot-launcher:// handler"
	fi
	if command -v update-desktop-database >/dev/null 2>&1; then
		update-desktop-database "$applications" >/dev/null 2>&1 || true
	fi
}

installed=$("$TARGET" version 2>/dev/null) || fail "the installed binary at $TARGET could not run"

printf '\n'
success "${bold}Plot Launcher $TAG${reset} installed"
printf '  binary    %s\n' "$TARGET"
printf '  version   %s\n' "$installed"
printf '  platform  %s/%s\n' "$os" "$arch"

case ":$PATH:" in
*":$INSTALL_DIR:"*) ;;
*)
	printf '\n'
	warn "$INSTALL_DIR is not on your PATH"
	profile="$HOME/.profile"
	case ${SHELL##*/} in
	zsh) profile="$HOME/.zshrc" ;;
	bash) profile="$HOME/.bashrc" ;;
	esac
	printf '  Add it for future shells:\n'
	printf '    echo %s >> %s\n' "'export PATH=\"$INSTALL_DIR:\$PATH\"'" "$profile"
	;;
esac

printf '\n%sNext steps%s\n' "$bold" "$reset"
printf '  plot-launcher settings set-base-directory ~/plot-workspaces\n'
printf '  plot-launcher settings set-editor "code --reuse-window"\n'
printf '  plot-launcher\n'

if [ "$os" = linux ]; then
	register_scheme
fi
printf '\nUpgrade later by running this installer again; uninstall with\n  rm %s\n' "$TARGET"
