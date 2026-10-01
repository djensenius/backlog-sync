package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomebrewTapUpdateGeneratesFormulaAndReadme(t *testing.T) {
	dir := t.TempDir()
	checksums := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(checksums, []byte(strings.Join([]string{
		"1111111111111111111111111111111111111111111111111111111111111111  backlog-sync_0.1.0_darwin_amd64.tar.gz",
		"2222222222222222222222222222222222222222222222222222222222222222  backlog-sync_0.1.0_darwin_arm64.tar.gz",
		"3333333333333333333333333333333333333333333333333333333333333333  backlog-sync_0.1.0_linux_amd64.tar.gz",
		"4444444444444444444444444444444444444444444444444444444444444444  backlog-sync_0.1.0_linux_arm64.tar.gz",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatalf("write checksums: %v", err)
	}
	outDir := filepath.Join(dir, "tap")

	runHomebrewTapUpdate(t, "--version", "v0.1.0", "--checksums", checksums, "--out-dir", outDir)
	formulaPath := filepath.Join(outDir, "Formula", "backlog-sync.rb")
	formula := readFile(t, formulaPath)
	readme := readFile(t, filepath.Join(outDir, "README.md"))

	for _, want := range []string{
		`class BacklogSync < Formula`,
		`version "0.1.0"`,
		`license "Apache-2.0"`,
		`https://github.com/djensenius/backlog-sync/releases/download/v0.1.0/backlog-sync_0.1.0_darwin_arm64.tar.gz`,
		`sha256 "2222222222222222222222222222222222222222222222222222222222222222"`,
		`https://github.com/djensenius/backlog-sync/releases/download/v0.1.0/backlog-sync_0.1.0_linux_amd64.tar.gz`,
		`sha256 "3333333333333333333333333333333333333333333333333333333333333333"`,
		`bin.install "backlog-sync"`,
		`assert_match version.to_s, shell_output("#{bin}/backlog-sync --version")`,
	} {
		if !strings.Contains(formula, want) {
			t.Fatalf("formula missing %q:\n%s", want, formula)
		}
	}
	for _, want := range []string{
		"# Homebrew tap\n",
		"<!-- backlog-sync formula section: start -->",
		"brew install djensenius/tap/backlog-sync",
		"<!-- backlog-sync formula section: end -->",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README missing %q:\n%s", want, readme)
		}
	}

	firstFormula := formula
	firstReadme := readme
	runHomebrewTapUpdate(t, "--version", "v0.1.0", "--checksums", checksums, "--tap-dir", outDir)
	if got := readFile(t, formulaPath); got != firstFormula {
		t.Fatalf("formula is not deterministic\nfirst:\n%s\nsecond:\n%s", firstFormula, got)
	}
	if got := readFile(t, filepath.Join(outDir, "README.md")); got != firstReadme {
		t.Fatalf("README is not deterministic\nfirst:\n%s\nsecond:\n%s", firstReadme, got)
	}
}

func TestHomebrewTapUpdateReplacesExistingReadmeBlock(t *testing.T) {
	dir := t.TempDir()
	checksums := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(checksums, []byte(strings.Join([]string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  backlog-sync_1.2.3_darwin_amd64.tar.gz",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb  backlog-sync_1.2.3_darwin_arm64.tar.gz",
		"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc  backlog-sync_1.2.3_linux_amd64.tar.gz",
		"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd  backlog-sync_1.2.3_linux_arm64.tar.gz",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatalf("write checksums: %v", err)
	}
	tapDir := filepath.Join(dir, "tap")
	if err := os.MkdirAll(tapDir, 0o755); err != nil {
		t.Fatalf("mkdir tap: %v", err)
	}
	initialREADME := "# Existing tap\n\nIntro.\n\n<!-- backlog-sync formula section: start -->\nold\n<!-- backlog-sync formula section: end -->\n\nFooter.\n"
	if err := os.WriteFile(filepath.Join(tapDir, "README.md"), []byte(initialREADME), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	runHomebrewTapUpdate(t, "--version", "refs/tags/v1.2.3", "--checksums", checksums, "--tap-dir", tapDir)
	readme := readFile(t, filepath.Join(tapDir, "README.md"))
	for _, want := range []string{"# Existing tap", "Intro.", "brew install djensenius/tap/backlog-sync", "Footer."} {
		if !strings.Contains(readme, want) {
			t.Fatalf("updated README missing %q:\n%s", want, readme)
		}
	}
	if strings.Contains(readme, "old") {
		t.Fatalf("updated README kept stale generated block:\n%s", readme)
	}
}

func TestHomebrewTapUpdateFailsWhenRequiredArchiveChecksumIsMissing(t *testing.T) {
	dir := t.TempDir()
	checksums := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(checksums, []byte("1111111111111111111111111111111111111111111111111111111111111111  backlog-sync_0.1.0_darwin_arm64.tar.gz\n"), 0o644); err != nil {
		t.Fatalf("write checksums: %v", err)
	}
	cmd := exec.Command("go", "run", "./scripts/homebrew-tap-update.go", "--version", "v0.1.0", "--checksums", checksums, "--out-dir", filepath.Join(dir, "tap"))
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing checksum failure, got success with output:\n%s", output)
	}
	if !strings.Contains(string(output), "missing required release assets") || !strings.Contains(string(output), "backlog-sync_0.1.0_linux_amd64.tar.gz") {
		t.Fatalf("failure did not explain missing archives:\n%s", output)
	}
}

func runHomebrewTapUpdate(t *testing.T, args ...string) {
	t.Helper()
	cmdArgs := append([]string{"run", "./scripts/homebrew-tap-update.go"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s failed: %v\n%s", strings.Join(cmdArgs, " "), err, output)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
