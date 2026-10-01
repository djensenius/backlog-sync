package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		logf("error: %v", err)
		os.Exit(2)
	}
	if cfg.Root == "" {
		root, err := gitTopLevel()
		if err != nil {
			logf("error: %v", err)
			os.Exit(2)
		}
		cfg.Root = root
	}
	cfg.Root, _ = filepath.Abs(cfg.Root)
	runner := OSCommandRunner{}
	app := App{
		Git:     ExecGit{Runner: runner},
		Backlog: ExecBacklog{Runner: runner},
		GitHub:  ExecGitHub{Runner: runner},
		Logf:    logf,
	}
	if _, err := app.Run(context.Background(), cfg); err != nil {
		logf("error: %v", err)
		os.Exit(1)
	}
}

func parseFlags(args []string) (Config, error) {
	fs := flag.NewFlagSet("backlog-sync", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	cfg := Config{}
	fs.StringVar(&cfg.Root, "root", "", "repository root to treat as the main worktree (default: git toplevel of cwd)")
	fs.StringVar(&cfg.Repo, "repo", "djensenius/canadian-ham", "GitHub repository owner/name")
	fs.StringVar(&cfg.ProjectOwner, "project-owner", "djensenius", "GitHub Project v2 owner login")
	fs.IntVar(&cfg.ProjectNumber, "project-number", 8, "GitHub Project v2 number")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "print planned writes without changing GitHub or Backlog")
	fs.BoolVar(&cfg.NoInbox, "no-inbox", false, "skip GitHub inbox import")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "print additional diagnostic logs")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() != 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	return cfg, nil
}

func gitTopLevel() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("resolve git top-level: %w", err)
	}
	return stringTrimSpace(out), nil
}

func stringTrimSpace(in []byte) string {
	start, end := 0, len(in)
	for start < end && (in[start] == ' ' || in[start] == '\n' || in[start] == '\t' || in[start] == '\r') {
		start++
	}
	for end > start && (in[end-1] == ' ' || in[end-1] == '\n' || in[end-1] == '\t' || in[end-1] == '\r') {
		end--
	}
	return string(in[start:end])
}

func logf(format string, args ...any) {
	fmt.Printf("%s "+format+"\n", append([]any{time.Now().Format(time.RFC3339)}, args...)...)
}
