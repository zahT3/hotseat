#!/bin/sh
# Runs the Swift tests. With only the Command Line Tools installed, SwiftPM
# sometimes leaves the swift-testing macro plugin off the compiler command
# line ("plugin for module 'TestingMacros' not found"); naming it explicitly
# makes every run see it.
set -eu
cd "$(dirname "$0")"
plugins="$(dirname "$(xcrun --find swift)")/../lib/swift/host/plugins/testing"
if [ -d "$plugins" ]; then
  exec swift test -Xswiftc -plugin-path -Xswiftc "$plugins" "$@"
fi
exec swift test "$@"
