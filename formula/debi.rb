class Debi < Formula
  desc "Command-line interface for the Debi API"
  homepage "https://github.com/debipro/cli"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.intel?
      url "https://github.com/debipro/cli/releases/download/v0.1.0/debi_0.1.0_darwin_amd64.tar.gz"
      sha256 "PLACEHOLDER"
    end
    if Hardware::CPU.arm?
      url "https://github.com/debipro/cli/releases/download/v0.1.0/debi_0.1.0_darwin_arm64.tar.gz"
      sha256 "PLACEHOLDER"
    end
  end

  on_linux do
    if Hardware::CPU.intel?
      url "https://github.com/debipro/cli/releases/download/v0.1.0/debi_0.1.0_linux_amd64.tar.gz"
      sha256 "PLACEHOLDER"
    end
    if Hardware::CPU.arm?
      url "https://github.com/debipro/cli/releases/download/v0.1.0/debi_0.1.0_linux_arm64.tar.gz"
      sha256 "PLACEHOLDER"
    end
  end

  def install
    bin.install "debi"
  end

  test do
    assert_match "debi", shell_output("#{bin}/debi version")
  end
end
