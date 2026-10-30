#!/bin/sh
# Builds Hotseat.app (menu bar) with SwiftPM and signs it ad hoc. Only the
# Command Line Tools are needed. With --install, replaces
# ~/Applications/Hotseat.app (the old copy goes to the Trash) and opens it.
set -eu
cd "$(dirname "$0")"
version=$(sed -n 's/^const Version = "\(.*\)"$/\1/p' ../internal/cli/cli.go)
version=${version:-0.0.0}

# The app carries its own hotseat CLI. HOTSEAT_BIN lets a release build
# pass a universal binary instead of one for this Mac only.
mkdir -p build
if [ -n "${HOTSEAT_BIN:-}" ]; then
  cli="$HOTSEAT_BIN"
else
  (cd .. && go build -o macos/build/hotseat ./cmd/hotseat)
  cli=build/hotseat
fi

swift build -c release
app=build/Hotseat.app
rm -rf build/Hotseat.app
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources/bin"
cp "$cli" "$app/Contents/Resources/bin/hotseat"
cp "$(swift build -c release --show-bin-path)/Hotseat" "$app/Contents/MacOS/Hotseat"
cp Resources/AppIcon.icns "$app/Contents/Resources/AppIcon.icns"
cat > "$app/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleIdentifier</key><string>io.github.zahT3.hotseat</string>
  <key>CFBundleName</key><string>Hotseat</string>
  <key>CFBundleExecutable</key><string>Hotseat</string>
  <key>CFBundleIconFile</key><string>AppIcon</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>$version</string>
  <key>CFBundleVersion</key><string>$version</string>
  <key>LSUIElement</key><true/>
  <key>LSMinimumSystemVersion</key><string>14.0</string>
</dict>
</plist>
PLIST
codesign --force --sign - "$app"
codesign --verify "$app"
echo "built $(pwd)/$app ($version)"

if [ "${1:-}" = "--install" ]; then
  osascript -e 'quit app id "io.github.zahT3.hotseat"' 2>/dev/null || true
  dest="$HOME/Applications/Hotseat.app"
  if [ -e "$dest" ]; then
    trash "$dest"
  fi
  mkdir -p "$HOME/Applications"
  ditto "$app" "$dest"
  open "$dest"
  echo "installed $dest"
fi
