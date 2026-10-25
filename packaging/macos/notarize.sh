#!/usr/bin/env bash
# Notarizes Plot Launcher.app with Apple, staples the ticket, and writes the zip
# that ships in the release.
set -euo pipefail

usage() {
	cat <<'EOF'
usage: notarize.sh --app <path> --zip <path> \
         (--key <p8> --key-id <id> --issuer <uuid> |
          --apple-id <email> --password <app password> --team-id <id>)
EOF
}

app=
zip=
key=
key_id=
issuer=
apple_id=
password=
team_id=

while [ $# -gt 0 ]; do
	case $1 in
	--app)
		app=${2:?--app requires a value}
		shift 2
		;;
	--zip)
		zip=${2:?--zip requires a value}
		shift 2
		;;
	--key)
		key=${2:?--key requires a value}
		shift 2
		;;
	--key-id)
		key_id=${2:?--key-id requires a value}
		shift 2
		;;
	--issuer)
		issuer=${2:?--issuer requires a value}
		shift 2
		;;
	--apple-id)
		apple_id=${2:?--apple-id requires a value}
		shift 2
		;;
	--password)
		password=${2:?--password requires a value}
		shift 2
		;;
	--team-id)
		team_id=${2:?--team-id requires a value}
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

if [ -z "$app" ] || [ -z "$zip" ]; then
	usage >&2
	exit 2
fi
[ -d "$app" ] || {
	echo "$app is not an app bundle" >&2
	exit 1
}

if [ -n "$key" ]; then
	credentials=(--key "$key" --key-id "$key_id" --issuer "$issuer")
elif [ -n "$apple_id" ]; then
	credentials=(--apple-id "$apple_id" --password "$password" --team-id "$team_id")
else
	echo "notarization needs --key/--key-id/--issuer or --apple-id/--password/--team-id" >&2
	exit 2
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

echo "→ packing the bundle for the notary service"
ditto -c -k --keepParent "$app" "$work/upload.zip"

echo "→ submitting to the notary service"
xcrun notarytool submit "$work/upload.zip" "${credentials[@]}" --wait

echo "→ stapling the ticket"
xcrun stapler staple "$app"
xcrun stapler validate "$app"

echo "→ checking Gatekeeper acceptance"
spctl --assess --type execute --verbose=2 "$app"

echo "→ writing $zip"
rm -f "$zip"
mkdir -p "$(dirname "$zip")"
ditto -c -k --keepParent "$app" "$zip"
