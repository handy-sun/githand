package status

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/handy-sun/githand/internal/config"
)

func TestCollectCleanRepo(t *testing.T) {
	dir := initTestRepo(t)
	reg := &config.Registry{Repos: []config.Repo{{Name: "test", Path: dir}}}

	results, err := Collect(reg, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Dirty {
		t.Error("clean repo should not be dirty")
	}
	if results[0].Detached {
		t.Error("on a branch should not be detached")
	}
}

func TestCollectDirtyRepo(t *testing.T) {
	dir := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("dirty"), 0o644)

	reg := &config.Registry{Repos: []config.Repo{{Name: "test", Path: dir}}}
	results, err := Collect(reg, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !results[0].Dirty {
		t.Error("repo with untracked file should be dirty")
	}
}

func TestFilterDirty(t *testing.T) {
	results := []RepoStatus{
		{Repo: config.Repo{Name: "a"}, Dirty: true},
		{Repo: config.Repo{Name: "b"}, Dirty: false},
		{Repo: config.Repo{Name: "c"}, Dirty: true},
	}
	filtered := FilterByFlag(results, "dirty")
	if len(filtered) != 2 {
		t.Errorf("expected 2 dirty, got %d", len(filtered))
	}
}

func TestFilterStash(t *testing.T) {
	results := []RepoStatus{
		{Repo: config.Repo{Name: "a"}, StashCount: 2},
		{Repo: config.Repo{Name: "b"}, StashCount: 0},
	}
	filtered := FilterByFlag(results, "stash")
	if len(filtered) != 1 || filtered[0].Repo.Name != "a" {
		t.Errorf("expected a, got %v", filtered)
	}
}

func TestFilterDetached(t *testing.T) {
	results := []RepoStatus{
		{Repo: config.Repo{Name: "a"}, Detached: false},
		{Repo: config.Repo{Name: "b"}, Detached: true},
	}
	filtered := FilterByFlag(results, "detached")
	if len(filtered) != 1 || filtered[0].Repo.Name != "b" {
		t.Errorf("expected b, got %v", filtered)
	}
}

func TestFilterByGroup(t *testing.T) {
	reg := &config.Registry{
		Repos: []config.Repo{
			{Name: "a", Group: "x"},
			{Name: "b", Group: "y"},
			{Name: "c", Group: "x"},
		},
		Groups: map[string][]string{"x": {"a", "c"}},
	}
	results := []RepoStatus{
		{Repo: reg.Repos[0]},
		{Repo: reg.Repos[1]},
		{Repo: reg.Repos[2]},
	}
	filtered := FilterByGroup(results, reg, "x")
	if len(filtered) != 2 {
		t.Errorf("expected 2 in group x, got %d", len(filtered))
	}
}

func TestFilterByUser(t *testing.T) {
	results := []RepoStatus{
		{Repo: config.Repo{Name: "a"}, Remotes: []RemoteInfo{{Name: "origin", URL: "https://github.com/handy-sun/a.git"}}},
		{Repo: config.Repo{Name: "b"}, Remotes: []RemoteInfo{{Name: "origin", URL: "https://github.com/other/b.git"}}},
	}
	filtered := FilterByUser(results, "handy-sun")
	if len(filtered) != 1 || filtered[0].Repo.Name != "a" {
		t.Errorf("expected a, got %v", filtered)
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "githand-status-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	mustGit(t, dir, "init")
	mustGit(t, dir, "config", "user.email", "test@test.com")
	mustGit(t, dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0o644)
	mustGit(t, dir, "add", "README.md")
	mustGit(t, dir, "commit", "-m", "initial")
	return dir
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}