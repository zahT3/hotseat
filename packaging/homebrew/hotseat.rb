# typed: strict
# frozen_string_literal: true

# Hotseat: several Claude accounts on one Mac, one workspace.
class Hotseat < Formula
  desc "Claude account switcher for macOS: switch accounts, keep your place"
  homepage "https://github.com/zahT3/hotseat"
  url "https://github.com/zahT3/hotseat/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "1f95e97fce7dcb81864b3b0866012c949ed91bc0f07ee6872d750a9536acbfbe"
  license "MIT"

  depends_on "go" => :build
  depends_on :macos

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/hotseat"
    # Built here rather than downloaded, the app carries no quarantine flag.
    cd "macos" do
      # Homebrew builds in a sandbox, in which SwiftPM cannot sandbox its
      # manifest compile.
      with_env(HOTSEAT_BIN: bin/"hotseat", SWIFT_FLAGS: "--disable-sandbox") do
        system "./build-app.sh"
      end
      prefix.install "build/Hotseat.app"
    end
  end

  def caveats
    <<~EOS
      To use the menu bar app:
        mkdir -p ~/Applications
        cp -R #{opt_prefix}/Hotseat.app ~/Applications/
        open ~/Applications/Hotseat.app
      It walks you through setup on first launch.
    EOS
  end

  test do
    assert_match "hotseat", shell_output("#{bin}/hotseat version")
  end
end
