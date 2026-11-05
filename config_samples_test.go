package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestSampleConfigsParseStrictlyWithoutNetwork(t *testing.T) {
	samples := []struct {
		path      string
		wantRepos int
	}{
		{path: "examples/minimal-single-repo.json", wantRepos: 0},
		{path: "examples/multi-repo.json", wantRepos: 3},
	}
	for _, sample := range samples {
		t.Run(sample.path, func(t *testing.T) {
			data, err := os.ReadFile(sample.path)
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(data) {
				t.Fatalf("sample must be pure JSON: %s", sample.path)
			}

			var raw map[string]any
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("unmarshal sample for placeholder checks: %v", err)
			}
			assertOmittedRootOnlyKeys(t, sample.path, raw)
			assertNoConsumerSpecificValues(t, sample.path, raw)

			root := t.TempDir()
			cfg, err := parseFlags([]string{"--root", root, "--config", sample.path})
			if err != nil {
				t.Fatalf("parseFlags strict decode/normalize/validate failed: %v", err)
			}
			if cfg.Root != root {
				t.Fatalf("cfg.Root=%q, want --root %q", cfg.Root, root)
			}
			if len(cfg.Repos) != sample.wantRepos {
				t.Fatalf("len(cfg.Repos)=%d, want %d", len(cfg.Repos), sample.wantRepos)
			}
			if !strings.Contains(cfg.ProjectOwner, "OWNER") || !strings.Contains(cfg.DefaultRepo, "/") {
				t.Fatalf("sample should use placeholder owner/repo values, got projectOwner=%q defaultRepo=%q", cfg.ProjectOwner, cfg.DefaultRepo)
			}
		})
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{
  "projectOwner": "OWNER_LOGIN",
  "projectOwnerType": "user",
  "projectNumber": 1,
  "defaultRepo": "OWNER_LOGIN/REPOSITORY_NAME",
  "mainBranch": "main",
  "unexpectedField": true
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := loadConfig(path)
	if err == nil {
		t.Fatal("loadConfig accepted an unknown field")
	}
	if !strings.Contains(err.Error(), "unknown field") || !strings.Contains(err.Error(), "unexpectedField") {
		t.Fatalf("loadConfig error %q, want unknown field unexpectedField", err)
	}
}

func TestLoadConfigRejectsTrailingData(t *testing.T) {
	base := `{
  "projectOwner": "OWNER_LOGIN",
  "projectOwnerType": "user",
  "projectNumber": 1,
  "defaultRepo": "OWNER_LOGIN/REPOSITORY_NAME",
  "mainBranch": "main"
}`
	for _, tc := range []struct {
		name string
		data string
	}{
		{name: "second-json-value", data: base + ` {"projectOwner":"OTHER"}`},
		{name: "comment", data: base + ` // comment`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(tc.data), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := loadConfig(path)
			if err == nil {
				t.Fatalf("loadConfig accepted trailing data for %s", tc.name)
			}
		})
	}
}

func writeRetryTestConfig(t *testing.T, extra string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	data := `{"projectOwner": "OWNER_LOGIN", "projectNumber": 1, "defaultRepo": "OWNER_LOGIN/REPOSITORY_NAME"` + extra + `}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCLIRetriesConfig(t *testing.T) {
	t.Setenv(envTimeoutSeconds, "")
	t.Setenv(envCLIRetries, "")
	for _, tc := range []struct {
		name    string
		extra   string
		want    int
		wantErr string
	}{
		{name: "default", want: 2},
		{name: "explicit", extra: `, "cliRetries": 5`, want: 5},
		{name: "disabled", extra: `, "cliRetries": 0`, want: 0},
		{name: "maximum", extra: `, "cliRetries": 10`, want: 10},
		{name: "negative", extra: `, "cliRetries": -1`, wantErr: "cliRetries must be a non-negative integer"},
		{name: "above maximum", extra: `, "cliRetries": 11`, wantErr: "cliRetries must be at most 10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := parseFlags([]string{"--root", t.TempDir(), "--config", writeRetryTestConfig(t, tc.extra)})
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("parseFlags err=%v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.CLIRetryCount() != tc.want || cfg.TimeoutSeconds != 60 {
				t.Fatalf("CLIRetryCount=%d TimeoutSeconds=%d, want %d and default 60", cfg.CLIRetryCount(), cfg.TimeoutSeconds, tc.want)
			}
		})
	}
}

func TestEnvOverridesTimeoutAndCLIRetries(t *testing.T) {
	path := writeRetryTestConfig(t, `, "timeoutSeconds": 30, "cliRetries": 3`)
	args := []string{"--root", t.TempDir(), "--config", path}

	t.Setenv(envTimeoutSeconds, "")
	t.Setenv(envCLIRetries, "")
	cfg, err := parseFlags(args)
	if err != nil || cfg.TimeoutSeconds != 30 || cfg.CLIRetryCount() != 3 {
		t.Fatalf("empty env: TimeoutSeconds=%d CLIRetryCount=%d err=%v, want file values 30 and 3", cfg.TimeoutSeconds, cfg.CLIRetryCount(), err)
	}

	t.Setenv(envTimeoutSeconds, "120")
	t.Setenv(envCLIRetries, "0")
	cfg, err = parseFlags(args)
	if err != nil || cfg.TimeoutSeconds != 120 || cfg.CLIRetryCount() != 0 {
		t.Fatalf("env override: TimeoutSeconds=%d CLIRetryCount=%d err=%v, want 120 and 0", cfg.TimeoutSeconds, cfg.CLIRetryCount(), err)
	}

	t.Setenv(envCLIRetries, "10")
	if cfg, err = parseFlags(args); err != nil || cfg.CLIRetryCount() != 10 {
		t.Fatalf("env maximum: CLIRetryCount=%d err=%v, want 10", cfg.CLIRetryCount(), err)
	}

	for _, tc := range []struct{ name, value, want string }{
		{envTimeoutSeconds, "abc", "must be a non-negative integer"},
		{envTimeoutSeconds, "-5", "must be a non-negative integer"},
		{envCLIRetries, "1.5", "must be a non-negative integer"},
		{envCLIRetries, "-1", "must be a non-negative integer"},
		{envCLIRetries, "11", "must be at most 10, got 11"},
	} {
		t.Run(tc.name+"="+tc.value, func(t *testing.T) {
			t.Setenv(envTimeoutSeconds, "")
			t.Setenv(envCLIRetries, "")
			t.Setenv(tc.name, tc.value)
			_, err := parseFlags(args)
			want := tc.name + " " + tc.want
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("parseFlags err=%v, want %q", err, want)
			}
		})
	}
}

func assertOmittedRootOnlyKeys(t *testing.T, samplePath string, raw map[string]any) {
	t.Helper()
	for _, key := range []string{"root", "lockFile"} {
		if _, ok := raw[key]; ok {
			t.Fatalf("%s should omit %q so consumers do not copy environment-specific paths", samplePath, key)
		}
	}
}

func assertNoConsumerSpecificValues(t *testing.T, samplePath string, value any) {
	t.Helper()
	for _, s := range collectJSONStringValues(value) {
		for _, forbidden := range []string{"/Users/", "djensenius", "canadian-ham", "ArkhamHorror"} {
			if strings.Contains(s, forbidden) {
				t.Fatalf("%s contains consumer-specific value %q", samplePath, s)
			}
		}
		if filepath.IsAbs(s) {
			t.Fatalf("%s contains absolute path %q", samplePath, s)
		}
	}
}

func collectJSONStringValues(value any) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case []any:
		var values []string
		for _, elem := range v {
			values = append(values, collectJSONStringValues(elem)...)
		}
		return values
	case map[string]any:
		var values []string
		for key, elem := range v {
			values = append(values, key)
			values = append(values, collectJSONStringValues(elem)...)
		}
		return values
	default:
		return nil
	}
}

func TestReadmeLinksToSampleConfigs(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"examples/minimal-single-repo.json", "examples/multi-repo.json"} {
		if !strings.Contains(string(readme), "("+path+")") {
			t.Fatalf("README.md does not link to %s", path)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("README.md links to missing sample %s: %v", path, err)
		}
	}
}

func TestReadmeConfigExamplesMatchSampleFiles(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		heading string
		path    string
	}{
		{heading: "### Single-repo config example", path: "examples/minimal-single-repo.json"},
		{heading: "### Multi-repo config example", path: "examples/multi-repo.json"},
	} {
		t.Run(sample.path, func(t *testing.T) {
			got, ok := readmeJSONBlockAfterHeading(string(readme), sample.heading)
			if !ok {
				t.Fatalf("README.md missing fenced json block after %q", sample.heading)
			}
			want, err := os.ReadFile(sample.path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(got) != strings.TrimSpace(string(want)) {
				t.Fatalf("README.md fenced json block after %q does not match %s", sample.heading, sample.path)
			}
		})
	}
}

func readmeJSONBlockAfterHeading(readme, heading string) (string, bool) {
	start := strings.Index(readme, heading)
	if start < 0 {
		return "", false
	}
	afterHeading := readme[start+len(heading):]
	fenceStart := strings.Index(afterHeading, "```json")
	if fenceStart < 0 {
		return "", false
	}
	afterFence := afterHeading[fenceStart+len("```json"):]
	if strings.HasPrefix(afterFence, "\r\n") {
		afterFence = strings.TrimPrefix(afterFence, "\r\n")
	} else {
		afterFence = strings.TrimPrefix(afterFence, "\n")
	}
	fenceEnd := strings.Index(afterFence, "\n```")
	if fenceEnd < 0 {
		return "", false
	}
	return afterFence[:fenceEnd], true
}

func TestReadmeLocalMarkdownLinksResolve(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	linkRE := regexp.MustCompile(`!?\[[^\]]+\]\(([^)]+)\)`)
	matches := linkRE.FindAllStringSubmatch(string(readme), -1)
	if len(matches) == 0 {
		t.Fatal("README.md should contain Markdown links")
	}
	for _, match := range matches {
		target := strings.TrimSpace(match[1])
		if target == "" {
			t.Fatalf("empty Markdown link target in %q", match[0])
		}
		if isExternalMarkdownLink(target) {
			continue
		}
		pathPart := strings.SplitN(target, "#", 2)[0]
		pathPart = strings.SplitN(pathPart, "?", 2)[0]
		if pathPart == "" {
			continue
		}
		if filepath.IsAbs(pathPart) {
			t.Fatalf("README.md Markdown link %q should be relative", target)
		}
		if _, err := os.Stat(filepath.Clean(pathPart)); err != nil {
			t.Fatalf("README.md links to missing local path %q: %v", target, err)
		}
	}
}

func TestReadmeHasNoConsumerSpecificPaths(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"/Users/", "canadian-ham", "ArkhamHorror"} {
		if strings.Contains(string(readme), forbidden) {
			t.Fatalf("README.md contains consumer-specific text %q", forbidden)
		}
	}
}

func isExternalMarkdownLink(target string) bool {
	return strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "mailto:")
}
