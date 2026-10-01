package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
