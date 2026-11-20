#!/bin/sh
set -eu
cd "$(dirname "$0")/.."

arch="${1:-}"
case "$arch" in
  arm64|amd64) ;;
  *) echo "Usage: scripts/package-mac.sh {arm64|amd64} [output-directory]" >&2; exit 2 ;;
esac
output="${2:-build/mac-$arch}"
app="$output/Dinoscript.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
CGO_ENABLED=0 GOOS=darwin GOARCH="$arch" go build -trimpath \
  -o "$app/Contents/MacOS/Dinoscript" ./cmd/dinoscript
cp LICENSE NOTICE release/THIRD-PARTY-NOTICES.txt "$app/Contents/Resources/"
cp release/Dinoscript.icns "$app/Contents/Resources/"

cat > "$app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>Dinoscript</string>
    <key>CFBundleIdentifier</key>
    <string>com.fclaude.dinoscript</string>
    <key>CFBundleIconFile</key>
    <string>Dinoscript.icns</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>CFBundleName</key>
    <string>Dinoscript</string>
    <key>CFBundleDevelopmentRegion</key>
    <string>en</string>
    <key>CFBundleLocalizations</key>
    <array>
        <string>en</string>
        <string>es</string>
        <string>pl</string>
    </array>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleVersion</key>
    <string>0.7.0</string>
    <key>CFBundleShortVersionString</key>
    <string>0.7.0</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
PLIST

if [ "$(uname -s)" = Darwin ]; then
  codesign --force --sign - "$app"
  staging="$(mktemp -d)"
  trap 'rm -rf "$staging"' EXIT
  ditto "$app" "$staging/Dinoscript.app"
  ln -s /Applications "$staging/Applications"
  cp book/Dinoscript-with-Pip.pdf book/Dinoscript-con-Pip.pdf book/Dinoscript-z-Pip.pdf LICENSE NOTICE release/THIRD-PARTY-NOTICES.txt "$staging/"
  hdiutil create -ov -format UDZO -volname Dinoscript -srcfolder "$staging" "$output/Dinoscript-$arch.dmg"
fi
