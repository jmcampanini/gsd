class Gsd < Formula
  desc "Personal task CLI backed by SQLite"
  homepage "https://github.com/jmcampanini/gsd"
  license "MIT"
  head "https://github.com/jmcampanini/gsd.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/jmcampanini/gsd/cmd.Version=#{version}
    ]
    system "go", "build", "-buildvcs=false", *std_go_args(ldflags:)
    generate_completions_from_executable(bin/"gsd", "completion")
  end

  test do
    assert_match "gsd version HEAD-", shell_output("#{bin}/gsd --version")
    database = testpath/"gsd.db"
    shell_output("#{bin}/gsd --db #{database} add 'Brew smoke task'")
    assert_match "Brew smoke task", shell_output("#{bin}/gsd --db #{database} list")
  end
end
