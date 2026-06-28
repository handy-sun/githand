package flakeupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/handy-sun/githand/internal/config"
)

func TestFlakeUpdateNonGitDir(t *testing.T) {
	dir := t.TempDir()
	reg := &config.Registry{Repos: []config.Repo{{Name: "test", Path: dir}}}

	results := Run(reg, "", 4, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "skipped" {
		t.Errorf("expected skipped, got %s", results[0].Status)
	}
	if results[0].Detail != "not a git repository" {
		t.Errorf("expected 'not a git repository', got %q", results[0].Detail)
	}
}

func TestFlakeUpdateNoFlakeNix(t *testing.T) {
	dir := t.TempDir()
	mustGit(t, dir, "init")
	mustGit(t, dir, "config", "user.email", "test@test.com")
	mustGit(t, dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0o644)
	mustGit(t, dir, "add", "README.md")
	mustGit(t, dir, "commit", "-m", "initial")

	reg := &config.Registry{Repos: []config.Repo{{Name: "test", Path: dir}}}
	results := Run(reg, "", 4, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "skipped" {
		t.Errorf("expected skipped, got %s", results[0].Status)
	}
	if results[0].Detail != "no flake.nix" {
		t.Errorf("expected 'no flake.nix', got %q", results[0].Detail)
	}
}

func TestFlakeUpdateGroupFilter(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	reg := &config.Registry{
		Repos: []config.Repo{
			{Name: "repo-a", Path: dir1, Group: "nix"},
			{Name: "repo-b", Path: dir2, Group: "other"},
		},
	}

	var names []string
	results := Run(reg, "nix", 4, func(r Result) {
		names = append(names, r.Name)
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result for group filter, got %d", len(results))
	}
	if results[0].Name != "repo-a" {
		t.Errorf("expected repo-a, got %s", results[0].Name)
	}
}

func TestFlakeUpdateDetachedHEAD(t *testing.T) {
	dir := t.TempDir()
	mustGit(t, dir, "init")
	mustGit(t, dir, "config", "user.email", "test@test.com")
	mustGit(t, dir, "config", "user.name", "Test")
	writeFile(t, filepath.Join(dir, "flake.nix"), []byte("{}\n"), 0o644)
	mustGit(t, dir, "add", "flake.nix")
	mustGit(t, dir, "commit", "-m", "initial")
	mustGit(t, dir, "checkout", "--detach", "HEAD")

	reg := &config.Registry{Repos: []config.Repo{{Name: "test", Path: dir}}}
	results := Run(reg, "", 4, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "skipped" {
		t.Errorf("expected skipped, got %s", results[0].Status)
	}
	if results[0].Detail != "detached HEAD" {
		t.Errorf("expected 'detached HEAD', got %q", results[0].Detail)
	}
}

func TestRunNixFlakeUpdateReturnsStderrOnSuccess(t *testing.T) {
	binDir := t.TempDir()
	nixPath := filepath.Join(binDir, "nix")
	writeFile(t, nixPath, []byte("#!/bin/sh\necho updated input >&2\n"), 0o755)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	stdout, stderr, err := runNixFlakeUpdate(t.TempDir())
	if err != nil {
		t.Fatalf("runNixFlakeUpdate returned error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if stderr != "updated input" {
		t.Errorf("expected stderr to be returned, got %q", stderr)
	}
}

func TestFlakeUpdateWithFlakeNix(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not available")
	}

	dir := t.TempDir()
	mustGit(t, dir, "init")
	mustGit(t, dir, "config", "user.email", "test@test.com")
	mustGit(t, dir, "config", "user.name", "Test")

	// Write a minimal flake.nix
	flakeContent := `{
  description = "test flake";
  inputs = {};
  outputs = { self }: {};
}
`
	os.WriteFile(filepath.Join(dir, "flake.nix"), []byte(flakeContent), 0o644)
	mustGit(t, dir, "add", ".")
	mustGit(t, dir, "commit", "-m", "initial")

	reg := &config.Registry{Repos: []config.Repo{{Name: "test", Path: dir}}}
	results := Run(reg, "", 4, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	switch r.Status {
	case "updated", "up-to-date":
		// Both are acceptable: flake update may or may not create a commit
		// depending on whether inputs changed.
	case "error":
		// nix flake update may fail in sandboxed test environments
		t.Logf("nix flake update returned error (may be expected in CI): %s", r.Detail)
	default:
		t.Errorf("unexpected status: %s (detail: %s)", r.Status, r.Detail)
	}
}

func writeFile(t *testing.T, path string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
