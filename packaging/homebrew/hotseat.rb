# typed: true
# frozen_string_literal: true

# Hotseat: several Claude accounts on one Mac, one workspace.
class Hotseat < Formula
  desc "Claude account switcher for macOS: switch accounts, keep your place"
  homepage "https://github.com/zahT3/hotseat"
  url "https://github.com/zahT3/hotseat/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"
  license "MIT"

  depends_on "go" => :build
  depends_on :macos

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/hotseat"
    # Built here rather than downloaded, the app carries no quarantine flag.
    cd "macos" do
      with_env(HOTSEAT_BIN: bin/"hotseat") do
        system "./build-app.sh"
      end
      prefix.install "build/Hotseat.app"
    end
  end

  def caveats
    <<~EOS
      To use the menu bar app:
        cp -R #{opt_prefix}/Hotseat.app ~/Applications/
        open ~/Applications/Hotseat.app
      It walks you through setup on first launch.
    EOS
  end

  test do
    assert_match "hotseat", shell_output("#{bin}/hotseat version")
  end
end
