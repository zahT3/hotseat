## Install

**Homebrew** (builds on your Mac, so macOS does not block the app):

```sh
brew install zahT3/tap/hotseat
mkdir -p ~/Applications
cp -R "$(brew --prefix)/opt/hotseat/Hotseat.app" ~/Applications/
open ~/Applications/Hotseat.app
```

**Download** `Hotseat-*-macos.zip`, unzip it, move `Hotseat.app` to `~/Applications` (create the folder if it does not exist), then run once:

```sh
xattr -dr com.apple.quarantine ~/Applications/Hotseat.app
```

Hotseat is free and not notarized by Apple; without that command macOS refuses to open an app downloaded from the internet. The app carries its own `hotseat` command line tool and installs it from its Setup window.

`hotseat-*-darwin-universal.tar.gz` is the command line tool alone; after unpacking, run `xattr -d com.apple.quarantine hotseat` once for the same reason.
