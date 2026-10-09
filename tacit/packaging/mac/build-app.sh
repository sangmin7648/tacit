#!/bin/sh
# Assembles and signs Tacit.app from binaries `make app` has already built.
#
#   build-app.sh <app> <desktop-binary> <cli-binary> <ten_vad.framework> <version> <sign-identity>
#
# Tacit.app is the menu-bar app (app/desktop) with the CLI (app/cli) bundled
# beside it, which it runs as `tacit listen`:
#
#   Contents/MacOS/Tacit                    menu-bar app, frontend embedded
#   Contents/Helpers/tacit                  the CLI
#   Contents/Frameworks/ten_vad.framework
#
# The CLI is in Helpers, not beside the app in MacOS: the default macOS
# filesystem is case-insensitive, so MacOS/tacit would overwrite MacOS/Tacit.
# The framework lives in Frameworks, where codesign expects nested code, so the
# bundled CLI gets an extra rpath to find it there.
#
# Signing is ad-hoc ("-") by default. macOS keys the microphone grant to an
# ad-hoc build's exact hash, so every rebuild asks again; sign with a stable
# identity (e.g. a self-signed "Code Signing" certificate made in Keychain
# Access) to keep grants across rebuilds:
#
#   make app SIGN_IDENTITY="Tacit Dev"
#
# Symptom of an ad-hoc rebuild: System Settings shows Tacit allowed for the
# microphone, yet the daemon it starts hears nothing — the grant belongs to an
# earlier build. Reset it and grant again:
#
#   tccutil reset Microphone io.github.sangmin7648.tacit
set -eu

app=$1 desktop=$2 cli=$3 framework=$4 version=$5 identity=$6
here=$(dirname "$0")

# CFBundleShortVersionString takes numbers only: v0.11.0-16-gfe768d8 -> 0.11.0.
plist_version=$(echo "$version" | sed -nE 's/^v?([0-9]+\.[0-9]+\.[0-9]+).*/\1/p')
plist_version=${plist_version:-0.0.0}

rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Helpers" "$app/Contents/Frameworks"
cp "$desktop" "$app/Contents/MacOS/Tacit"
cp "$cli" "$app/Contents/Helpers/tacit"
install_name_tool -add_rpath @executable_path/../Frameworks "$app/Contents/Helpers/tacit"
cp -R "$framework" "$app/Contents/Frameworks/"
cp "$here/Info.plist" "$app/Contents/Info.plist"
plutil -replace CFBundleShortVersionString -string "$plist_version" "$app/Contents/Info.plist"
plutil -replace CFBundleVersion -string "$plist_version" "$app/Contents/Info.plist"

codesign --force --sign "$identity" "$app/Contents/Frameworks/ten_vad.framework"
codesign --force --sign "$identity" "$app/Contents/Helpers/tacit"
codesign --force --sign "$identity" "$app"
codesign --verify --strict --deep "$app"
echo "Built $app"
