package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		logf("error: %v", err)
		os.Exit(2)
	}
	cfg = cfg.Normalized()
	unlock, err := acquireRunLock(cfg)
	if err != nil {
		if errors.Is(err, errLocked) {
			logf("locked, skipping: %s", cfg.LockFile)
			return
		}
		logf("error: %v", err)
		os.Exit(1)
	}
	defer unlock()
	runner := OSCommandRunner{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second}
	gh := ExecGitHub{Runner: runner, AllowedRepos: cfg.ConfiguredRepos()}
	app := App{Git: ExecGit{Runner: runner}, Backlog: ExecBacklog{Runner: runner}, GitHub: gh, Logf: logf}
	if _, err := app.Run(context.Background(), cfg); err != nil {
		logf("error: %v", err)
		os.Exit(1)
	}
}

func parseFlags(args []string) (Config, error) {
	fs := flag.NewFlagSet("backlog-sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var rootFlag, configFlag, repoFlag, ownerFlag, ownerTypeFlag, mainBranchFlag string
	var projectNumberFlag int
	var dryRun, noInbox, verbose bool
	fs.StringVar(&rootFlag, "root", "", "repository root (default: git toplevel of cwd)")
	fs.StringVar(&configFlag, "config", "", "JSON config path (default: <root>/.backlog-sync.json)")
	fs.StringVar(&repoFlag, "repo", "", "override default GitHub repository owner/name")
	fs.StringVar(&ownerFlag, "project-owner", "", "override GitHub Project v2 owner login")
	fs.StringVar(&ownerTypeFlag, "project-owner-type", "", "override Project owner type: user or org")
	fs.IntVar(&projectNumberFlag, "project-number", 0, "override GitHub Project v2 number")
	fs.StringVar(&mainBranchFlag, "main-branch", "", "override main branch name")
	fs.BoolVar(&dryRun, "dry-run", false, "print planned writes without changing GitHub, Backlog, or git")
	fs.BoolVar(&noInbox, "no-inbox", false, "skip GitHub inbox import")
	fs.BoolVar(&verbose, "verbose", false, "print additional diagnostic logs")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() != 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	root := rootFlag
	if root == "" {
		var err error
		root, err = gitTopLevel()
		if err != nil {
			return Config{}, err
		}
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Config{}, err
	}
	root = absRoot
	if configFlag == "" {
		configFlag = filepath.Join(root, ".backlog-sync.json")
	}
	if !filepath.IsAbs(configFlag) {
		configFlag, err = filepath.Abs(configFlag)
		if err != nil {
			return Config{}, err
		}
	}
	cfg, err := loadConfig(configFlag)
	if err != nil {
		return Config{}, err
	}
	cfg.ConfigPath = configFlag
	if cfg.Root == "" {
		cfg.Root = root
	}
	if rootFlag != "" {
		cfg.Root = root
	}
	cfg.Root, err = filepath.Abs(cfg.Root)
	if err != nil {
		return Config{}, err
	}
	if repoFlag != "" {
		cfg.DefaultRepo = repoFlag
	}
	if ownerFlag != "" {
		cfg.ProjectOwner = ownerFlag
	}
	if ownerTypeFlag != "" {
		cfg.ProjectOwnerType = ownerTypeFlag
	}
	if projectNumberFlag != 0 {
		cfg.ProjectNumber = projectNumberFlag
	}
	if mainBranchFlag != "" {
		cfg.MainBranch = mainBranchFlag
	}
	cfg.DryRun = dryRun
	cfg.NoInbox = noInbox
	cfg.Verbose = verbose
	cfg = cfg.Normalized()
	if cfg.LockFile == "" {
		cfg.LockFile = defaultLockFile(configFlag)
	}
	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

func validateConfig(cfg Config) error {
	var missing []string
	if cfg.Root == "" {
		missing = append(missing, "root")
	}
	if cfg.ProjectOwner == "" {
		missing = append(missing, "projectOwner")
	}
	if cfg.ProjectOwnerType != "user" && cfg.ProjectOwnerType != "org" {
		return fmt.Errorf("projectOwnerType must be user or org")
	}
	if cfg.ProjectNumber == 0 {
		missing = append(missing, "projectNumber")
	}
	if cfg.DefaultRepo == "" {
		missing = append(missing, "defaultRepo")
	}
	if len(missing) > 0 {
		return fmt.Errorf("config missing required fields: %s", strings.Join(missing, ", "))
	}
	if !strings.Contains(cfg.DefaultRepo, "/") {
		return fmt.Errorf("defaultRepo must be owner/name")
	}
	for project, repo := range cfg.Repos {
		if project == "" || !strings.Contains(repo, "/") {
			return fmt.Errorf("repos entries must map non-empty project names to owner/name repos")
		}
	}
	for _, label := range cfg.Labels.AddAlways {
		if !labelAllowedByManaged(label, cfg.Labels.Managed) {
			return fmt.Errorf("labels.addAlways %q is not covered by labels.managed", label)
		}
	}
	return nil
}

func gitTopLevel() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("resolve git top-level: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func defaultLockFile(configPath string) string {
	sum := sha256.Sum256([]byte(configPath))
	cache, err := os.UserCacheDir()
	if err != nil || cache == "" {
		cache = os.TempDir()
	}
	return filepath.Join(cache, "backlog-sync-"+hex.EncodeToString(sum[:8])+".lock")
}

var errLocked = errors.New("locked")

func acquireRunLock(cfg Config) (func(), error) {
	if cfg.LockFile == "" {
		return func() {}, nil
	}
	if err := os.MkdirAll(filepath.Dir(cfg.LockFile), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(cfg.LockFile, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, errLocked
		}
		return nil, err
	}
	_, _ = f.Seek(0, 0)
	_ = f.Truncate(0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}

func logf(format string, args ...any) {
	fmt.Printf("%s "+format+"\n", append([]any{time.Now().Format(time.RFC3339)}, args...)...)
}
