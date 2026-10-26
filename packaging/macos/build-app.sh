#!/usr/bin/env bash
# Builds Plot Launcher.app: a universal Plot Launcher binary plus the small shim
# that turns a plot:// Apple Event into the argument the launcher expects.
set -euo pipefail

usage() {
	cat <<'EOF'
usage: build-app.sh --version <tag> [--output <dir>] [--identity <codesign id>] [--bundle-id <id>]

  --version    Release tag, for example v0.2.0. Its digits make the bundle version.
  --output     Directory that receives Plot Launcher.app (default: dist/macos).
  --identity   codesign identity. Defaults to an ad-hoc signature, which is
               enough to test locally but is not accepted by Gatekeeper.
  --bundle-id  CFBundleIdentifier (default: ai.plot.launcher).
EOF
}

version=
output=dist/macos
identity=-
bundle_id=ai.plot.launcher

while [ $# -gt 0 ]; do
	case $1 in
	--version)
		version=${2:?--version requires a value}
		shift 2
		;;
	--output)
		output=${2:?--output requires a value}
		shift 2
		;;
	--identity)
		identity=${2:?--identity requires a value}
		shift 2
		;;
	--bundle-id)
		bundle_id=${2:?--bundle-id requires a value}
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "unknown argument: $1" >&2
		usage >&2
		exit 2
		;;
	esac
done

[ -n "$version" ] || {
	usage >&2
	exit 2
}

case $(uname -s) in
Darwin) ;;
*)
	echo "this script must run on macOS" >&2
	exit 1
	;;
esac

root=$(cd "$(dirname "$0")/../.." && pwd)
bundle_version=${version#v}
commit=$(git -C "$root" rev-parse --short HEAD)
built=$(date -u +%Y-%m-%dT%H:%M:%SZ)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

echo "→ building plot-launcher $version for darwin/amd64 and darwin/arm64"
for arch in amd64 arm64; do
	CGO_ENABLED=0 GOOS=darwin GOARCH=$arch go build -trimpath \
		-ldflags "-s -w \
			-X github.com/PlotAI/plot-launcher/internal/version.Version=$version \
			-X github.com/PlotAI/plot-launcher/internal/version.Commit=$commit \
			-X github.com/PlotAI/plot-launcher/internal/version.Date=$built" \
		-o "$work/plot-launcher-$arch" \
		"$root/cmd/plot-launcher"
done
lipo -create "$work/plot-launcher-amd64" "$work/plot-launcher-arm64" \
	-output "$work/plot-launcher"

echo "→ building the URL shim"
swiftc -O -swift-version 5 \
	-target "$(uname -m)-apple-macos11" \
	-o "$work/PlotLauncher" \
	"$root/packaging/macos/shim/main.swift"

echo "→ building the app icon"
iconset="$work/AppIcon.iconset"
mkdir -p "$iconset"
for size in 16 32 128 256 512; do
	sips -z $size $size "$root/packaging/macos/icon/AppIcon-1024.png" \
		--out "$iconset/icon_${size}x${size}.png" >/dev/null
	double=$((size * 2))
	sips -z $double $double "$root/packaging/macos/icon/AppIcon-1024.png" \
		--out "$iconset/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$iconset" -o "$work/AppIcon.icns"

app="$output/Plot Launcher.app"
rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp "$work/plot-launcher" "$app/Contents/MacOS/plot-launcher"
cp "$work/PlotLauncher" "$app/Contents/MacOS/PlotLauncher"
cp "$work/AppIcon.icns" "$app/Contents/Resources/AppIcon.icns"
sed -e "s|__VERSION__|$bundle_version|g" -e "s|__BUNDLE_ID__|$bundle_id|g" \
	"$root/packaging/macos/Info.plist" >"$app/Contents/Info.plist"
printf 'APPL????' >"$app/Contents/PkgInfo"

echo "→ signing with $identity"
if [ "$identity" = "-" ]; then
	sign=(--force --options runtime --sign -)
else
	sign=(--force --options runtime --timestamp --sign "$identity")
fi
codesign "${sign[@]}" "$app/Contents/MacOS/plot-launcher"
codesign "${sign[@]}" "$app/Contents/MacOS/PlotLauncher"
codesign "${sign[@]}" "$app"
codesign --verify --deep --strict "$app"

echo "$app"
