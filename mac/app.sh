#!/bin/sh
# Builds Underwick.app, so on macOS the game shows in the Dock with its own icon and name: macOS takes those
# from an app bundle, which a bare binary (or go run) doesn't have. The icon is the hero sprite, made at
# build time. The app embeds the Oryx sprites, so it is gitignored like them.
set -eu
cd "$(dirname "$0")/.."

if [ ! -d assets/Character ]; then
	echo "assets/ is missing: copy the Oryx sprites in first, see README" >&2
	exit 1
fi

app=Underwick.app
rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
go build -o "$app/Contents/MacOS/underwick" .

icons=$(mktemp -d)
trap 'rm -rf "$icons"' EXIT
"$app/Contents/MacOS/underwick" -iconset "$icons/icon.iconset"
iconutil -c icns "$icons/icon.iconset" -o "$app/Contents/Resources/icon.icns"

cat > "$app/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key><string>Underwick</string>
	<key>CFBundleDisplayName</key><string>Underwick</string>
	<key>CFBundleIdentifier</key><string>com.github.dbackowski.underwick</string>
	<key>CFBundleExecutable</key><string>underwick</string>
	<key>CFBundleIconFile</key><string>icon</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleVersion</key><string>$(git describe --always --dirty)</string>
	<key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
PLIST
echo "Built $app. Run it with: open $app"
