class Fleet < Formula
  desc "Sync Git repositories and list issues and pull requests across them"
  homepage "https://github.com/jmcampanini/fleet"
  license "MIT"
  head "https://github.com/jmcampanini/fleet.git", branch: "main"

  depends_on "go" => :build
  depends_on "gh"
  depends_on "git"

  def install
    ldflags = %W[
      -s -w
      -X github.com/jmcampanini/fleet/cmd.Version=#{version}
    ]
    system "go", "build", "-buildvcs=false", *std_go_args(ldflags:)
    generate_completions_from_executable(bin/"fleet", "completion")
  end

  test do
    assert_match "fleet version HEAD-", shell_output("#{bin}/fleet --version")
  end
end
