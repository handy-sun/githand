package sync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/handy-sun/githand/internal/config"
)

func TestSyncCleanUpToDate(t *testing.T) {
	_, pusher, clone := setupOriginWithPusher(t)

	_ = pusher
	reg := &config.Registry{Repos: []config.Repo{{Name: "test", Path: clone}}}
	results := Run(reg, "", "origin", 4, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "up-to-date" {
		t.Errorf("expected up-to-date, got %s: %s", results[0].Status, results[0].Detail)
	}
}

func TestSyncPullsNewCommit(t *testing.T) {
	_, pusher, clone := setupOriginWithPusher(t)

	// Push a new commit from pusher
	mustGit(t, pusher, "config", "user.email", "test@test.com")
	mustGit(t, pusher, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(pusher, "new.txt"), []byte("new content"), 0o644)
	mustGit(t, pusher, "add", "new.txt")
	mustGit(t, pusher, "commit", "-m", "new commit")
	mustGit(t, pusher, "push")

	reg := &config.Registry{Repos: []config.Repo{{Name: "test", Path: clone}}}
	results := Run(reg, "", "origin", 4, nil)

	if results[0].Status != "updated" {
		t.Errorf("expected updated, got %s: %s", results[0].Status, results[0].Detail)
	}
	if results[0].OldHash == "" || results[0].NewHash == "" {
		t.Errorf("expected hash range, got old=%q new=%q", results[0].OldHash, results[0].NewHash)
	}
	if results[0].OldHash == results[0].NewHash {
		t.Errorf("hash should change on update, both are %s", results[0].OldHash)
	}
	if _, err := os.Stat(filepath.Join(clone, "new.txt")); err != nil {
		t.Errorf("new.txt should exist after pull: %v", err)
	}
}

func TestSyncDirtyWorktree(t *testing.T) {
	_, pusher, clone := setupOriginWithPusher(t)

	// Push a new commit from pusher
	mustGit(t, pusher, "config", "user.email", "test@test.com")
	mustGit(t, pusher, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(pusher, "upstream.txt"), []byte("upstream"), 0o644)
	mustGit(t, pusher, "add", "upstream.txt")
	mustGit(t, pusher, "commit", "-m", "upstream commit")
	mustGit(t, pusher, "push")

	// Make clone dirty
	os.WriteFile(filepath.Join(clone, "local.txt"), []byte("local change"), 0o644)

	reg := &config.Registry{Repos: []config.Repo{{Name: "test", Path: clone}}}
	results := Run(reg, "", "origin", 4, nil)

	if results[0].Status != "updated" {
		t.Errorf("expected updated, got %s: %s", results[0].Status, results[0].Detail)
	}
	// Local change should be preserved (autostash)
	data, err := os.ReadFile(filepath.Join(clone, "local.txt"))
	if err != nil {
		t.Fatalf("local.txt should still exist: %v", err)
	}
	if string(data) != "local change" {
		t.Errorf("local change should be preserved, got %q", string(data))
	}
}

func TestSyncDetachedHEAD(t *testing.T) {
	_, pusher, clone := setupOriginWithPusher(t)

	// Detach HEAD in clone
	mustGit(t, clone, "checkout", "--detach")

	// Push a new commit from pusher
	mustGit(t, pusher, "config", "user.email", "test@test.com")
	mustGit(t, pusher, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(pusher, "new.txt"), []byte("new"), 0o644)
	mustGit(t, pusher, "add", "new.txt")
	mustGit(t, pusher, "commit", "-m", "new")
	mustGit(t, pusher, "push")

	reg := &config.Registry{Repos: []config.Repo{{Name: "test", Path: clone}}}
	results := Run(reg, "", "origin", 4, nil)

	if results[0].Status != "fetched" {
		t.Errorf("expected fetched for detached HEAD, got %s: %s", results[0].Status, results[0].Detail)
	}
}

func TestSyncNonGitDir(t *testing.T) {
	dir := t.TempDir()

	reg := &config.Registry{Repos: []config.Repo{{Name: "notagit", Path: dir}}}
	results := Run(reg, "", "origin", 4, nil)

	if results[0].Status != "skipped" {
		t.Errorf("expected skipped, got %s", results[0].Status)
	}
}

func TestSyncGroupFilter(t *testing.T) {
	_, _, clone1 := setupOriginWithPusher(t)
	_, _, clone2 := setupOriginWithPusher(t)

	reg := &config.Registry{
		Repos: []config.Repo{
			{Name: "repo1", Path: clone1, Group: "a"},
			{Name: "repo2", Path: clone2, Group: "b"},
		},
		Groups: map[string][]string{
			"a": {"repo1"},
			"b": {"repo2"},
		},
	}

	results := Run(reg, "a", "origin", 4, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for group 'a', got %d", len(results))
	}
	if results[0].Name != "repo1" {
		t.Errorf("expected repo1, got %s", results[0].Name)
	}
}

func TestSyncPullRebase(t *testing.T) {
	_, pusher, clone := setupOriginWithPusher(t)

	// Set pull.rebase=true in clone
	mustGit(t, clone, "config", "pull.rebase", "true")

	// Push a new commit from pusher
	mustGit(t, pusher, "config", "user.email", "test@test.com")
	mustGit(t, pusher, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(pusher, "new.txt"), []byte("new"), 0o644)
	mustGit(t, pusher, "add", "new.txt")
	mustGit(t, pusher, "commit", "-m", "new")
	mustGit(t, pusher, "push")

	reg := &config.Registry{Repos: []config.Repo{{Name: "test", Path: clone}}}
	results := Run(reg, "", "origin", 4, nil)

	if results[0].Status != "updated" {
		t.Errorf("expected updated, got %s: %s", results[0].Status, results[0].Detail)
	}

	// Verify rebase was used: log should show linear history (no merge commit)
	out := mustGitOutput(t, clone, "log", "--oneline")
	if strings.Contains(out, "Merge") {
		t.Error("expected rebase (no merge commits), but found merge")
	}
}

func TestSyncDefaultRemote(t *testing.T) {
	_, _, clone := setupOriginWithPusher(t)

	// Use empty remote — should default to origin
	reg := &config.Registry{Repos: []config.Repo{{Name: "test", Path: clone}}}
	results := Run(reg, "", "", 4, nil)

	if results[0].Status != "up-to-date" {
		t.Errorf("expected up-to-date with default remote, got %s: %s", results[0].Status, results[0].Detail)
	}
}

// setupOriginWithPusher creates a bare origin, a working pusher clone, and a
// test clone. Returns (origin_bare, pusher_workdir, clone_workdir).
func setupOriginWithPusher(t *testing.T) (string, string, string) {
	t.Helper()

	origin := t.TempDir()
	mustGit(t, origin, "init", "--bare")

	// Pusher: working clone used to push new commits to origin
	pusher := filepath.Join(t.TempDir(), "pusher")
	cmd := exec.Command("git", "clone", origin, pusher)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone pusher: %v\n%s", err, out)
	}
	mustGit(t, pusher, "config", "user.email", "test@test.com")
	mustGit(t, pusher, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(pusher, "README.md"), []byte("hello"), 0o644)
	mustGit(t, pusher, "add", "README.md")
	mustGit(t, pusher, "commit", "-m", "initial")
	mustGit(t, pusher, "push", "-u", "origin", "main")

	// Clone: the repo under test
	clone := filepath.Join(t.TempDir(), "clone")
	cmd = exec.Command("git", "clone", origin, clone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone clone: %v\n%s", err, out)
	}

	return origin, pusher, clone
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func mustGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
