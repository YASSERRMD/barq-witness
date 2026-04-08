class BarqWitness < Formula
  desc "Local-first AI code provenance recorder for Claude Code sessions"
  homepage "https://github.com/yasserrmd/barq-witness"
  version "1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/yasserrmd/barq-witness/releases/download/v1.0.0/barq-witness-darwin-arm64"
      sha256 "PLACEHOLDER_SHA256_DARWIN_ARM64"
    end
    on_intel do
      url "https://github.com/yasserrmd/barq-witness/releases/download/v1.0.0/barq-witness-darwin-amd64"
      sha256 "PLACEHOLDER_SHA256_DARWIN_AMD64"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/yasserrmd/barq-witness/releases/download/v1.0.0/barq-witness-linux-arm64"
      sha256 "PLACEHOLDER_SHA256_LINUX_ARM64"
    end
    on_intel do
      url "https://github.com/yasserrmd/barq-witness/releases/download/v1.0.0/barq-witness-linux-amd64"
      sha256 "PLACEHOLDER_SHA256_LINUX_AMD64"
    end
  end

  def install
    bin.install Dir["barq-witness*"].first => "barq-witness"
  end

  test do
    system "#{bin}/barq-witness", "version"
  end
end
