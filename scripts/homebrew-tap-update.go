//go:build ignore

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	formulaName     = "backlog-sync"
	formulaFileName = "backlog-sync.rb"
	defaultRepo     = "djensenius/backlog-sync"
	defaultTap      = "djensenius/tap"
	readmeStart     = "<!-- backlog-sync formula section: start -->"
	readmeEnd       = "<!-- backlog-sync formula section: end -->"
)

var requiredArchives = []archiveTarget{
	{OS: "darwin", Arch: "amd64"},
	{OS: "darwin", Arch: "arm64"},
	{OS: "linux", Arch: "amd64"},
	{OS: "linux", Arch: "arm64"},
}

type archiveTarget struct {
	OS   string
	Arch string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "homebrew tap update: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("homebrew-tap-update", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	versionFlag := fs.String("version", "", "release version or tag, for example v0.1.0")
	checksumsPath := fs.String("checksums", "", "path to GoReleaser checksums.txt")
	tapDir := fs.String("tap-dir", "", "Homebrew tap checkout to update in place")
	outDir := fs.String("out-dir", "", "output directory for a generated tap preview")
	sourceRepo := fs.String("source-repo", defaultRepo, "GitHub owner/repo for release asset URLs")
	tapName := fs.String("tap", defaultTap, "Homebrew tap name used in README install command")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *versionFlag == "" {
		return errors.New("--version is required")
	}
	if *checksumsPath == "" {
		return errors.New("--checksums is required")
	}
	if (*tapDir == "") == (*outDir == "") {
		return errors.New("provide exactly one of --tap-dir or --out-dir")
	}

	version, tag, err := normalizeVersion(*versionFlag)
	if err != nil {
		return err
	}
	checksums, err := parseChecksums(*checksumsPath)
	if err != nil {
		return err
	}
	archives, err := collectArchives(version, checksums)
	if err != nil {
		return err
	}

	destDir := *tapDir
	if destDir == "" {
		destDir = *outDir
	}
	formula := renderFormula(version, tag, *sourceRepo, archives)
	readmeBlock := renderReadmeBlock(*tapName)

	formulaPath := filepath.Join(destDir, "Formula", formulaFileName)
	if err := os.MkdirAll(filepath.Dir(formulaPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(formulaPath, []byte(formula), 0o644); err != nil {
		return err
	}

	readmePath := filepath.Join(destDir, "README.md")
	updatedREADME, err := updateReadme(readmePath, readmeBlock)
	if err != nil {
		return err
	}
	if err := os.WriteFile(readmePath, []byte(updatedREADME), 0o644); err != nil {
		return err
	}
	return nil
}

func normalizeVersion(input string) (version string, tag string, err error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", "", errors.New("empty version")
	}
	version = strings.TrimPrefix(trimmed, "refs/tags/")
	version = strings.TrimPrefix(version, "v")
	valid := regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+([-.+][0-9A-Za-z][0-9A-Za-z.-]*)?$`)
	if !valid.MatchString(version) {
		return "", "", fmt.Errorf("version %q must look like a semantic version", input)
	}
	return version, "v" + version, nil
}

func parseChecksums(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	checksums := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	hex := regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("%s:%d: expected '<sha256>  <asset>'", path, lineNumber)
		}
		if !hex.MatchString(fields[0]) {
			return nil, fmt.Errorf("%s:%d: invalid sha256 %q", path, lineNumber, fields[0])
		}
		asset := filepath.Base(fields[1])
		checksums[asset] = strings.ToLower(fields[0])
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return checksums, nil
}

func collectArchives(version string, checksums map[string]string) (map[archiveTarget]string, error) {
	archives := make(map[archiveTarget]string, len(requiredArchives))
	var missing []string
	for _, target := range requiredArchives {
		asset := assetName(version, target)
		sha, ok := checksums[asset]
		if !ok {
			missing = append(missing, asset)
			continue
		}
		archives[target] = sha
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("checksums file is missing required release assets: %s", strings.Join(missing, ", "))
	}
	return archives, nil
}

func assetName(version string, target archiveTarget) string {
	return fmt.Sprintf("%s_%s_%s_%s.tar.gz", formulaName, version, target.OS, target.Arch)
}

func releaseURL(repo string, tag string, version string, target archiveTarget) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, assetName(version, target))
}

func renderFormula(version string, tag string, repo string, archives map[archiveTarget]string) string {
	darwinAMD64 := archiveTarget{OS: "darwin", Arch: "amd64"}
	darwinARM64 := archiveTarget{OS: "darwin", Arch: "arm64"}
	linuxAMD64 := archiveTarget{OS: "linux", Arch: "amd64"}
	linuxARM64 := archiveTarget{OS: "linux", Arch: "arm64"}

	return fmt.Sprintf(`class BacklogSync < Formula
  desc "Mirror Backlog.md tasks to GitHub Issues and Projects"
  homepage "https://github.com/%[1]s"
  version "%[2]s"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "%[3]s"
      sha256 "%[4]s"
    elsif Hardware::CPU.intel?
      url "%[5]s"
      sha256 "%[6]s"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "%[7]s"
      sha256 "%[8]s"
    elsif Hardware::CPU.intel?
      url "%[9]s"
      sha256 "%[10]s"
    end
  end

  def install
    bin.install "backlog-sync"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/backlog-sync --version")
  end
end
`, repo, version,
		releaseURL(repo, tag, version, darwinARM64), archives[darwinARM64],
		releaseURL(repo, tag, version, darwinAMD64), archives[darwinAMD64],
		releaseURL(repo, tag, version, linuxARM64), archives[linuxARM64],
		releaseURL(repo, tag, version, linuxAMD64), archives[linuxAMD64])
}

func renderReadmeBlock(tap string) string {
	return fmt.Sprintf(`%s
## backlog-sync

Mirror Backlog.md tasks one-way to GitHub Issues and GitHub Projects.

`+"```sh\n"+`brew install %s/backlog-sync
`+"```\n\n"+`See https://github.com/djensenius/backlog-sync for configuration and release details.
%s
`, readmeStart, tap, readmeEnd)
}

func updateReadme(path string, block string) (string, error) {
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "# Homebrew tap\n\n" + block, nil
		}
		return "", err
	}
	content := string(contentBytes)
	start := strings.Index(content, readmeStart)
	end := strings.Index(content, readmeEnd)
	if start >= 0 || end >= 0 {
		if start < 0 || end < 0 || end < start {
			return "", fmt.Errorf("README.md has an incomplete %s block", formulaName)
		}
		end += len(readmeEnd)
		updated := strings.TrimRight(content[:start], " \t\r\n") + "\n\n" + strings.TrimRight(block, "\n") + "\n" + strings.TrimLeft(content[end:], " \t\r\n")
		return ensureTrailingNewline(updated), nil
	}
	return ensureTrailingNewline(strings.TrimRight(content, " \t\r\n") + "\n\n" + strings.TrimRight(block, "\n")), nil
}

func ensureTrailingNewline(s string) string {
	return strings.TrimRight(s, " \t\r\n") + "\n"
}
