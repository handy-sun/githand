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

func TestPrimarySource(t *testing.T) {
	tests := []struct {
		name    string
		remotes []RemoteInfo
		want    string
	}{
		{
			name: "prefers origin",
			remotes: []RemoteInfo{
				{Name: "upstream", URL: "https://codeberg.org/handy-sun/githand.git"},
				{Name: "origin", URL: "https://github.com/handy-sun/githand.git"},
			},
			want: "github.com",
		},
		{
			name: "falls back to first remote",
			remotes: []RemoteInfo{
				{Name: "upstream", URL: "https://codeberg.org/handy-sun/githand.git"},
				{Name: "mirror", URL: "https://github.com/handy-sun/githand.git"},
			},
			want: "codeberg.org",
		},
		{
			name:    "extracts scp-like SSH host",
			remotes: []RemoteInfo{{Name: "origin", URL: "git@github.com:handy-sun/githand.git"}},
			want:    "github.com",
		},
		{
			name:    "extracts SSH URL host",
			remotes: []RemoteInfo{{Name: "origin", URL: "ssh://git@codeberg.org/handy-sun/githand.git"}},
			want:    "codeberg.org",
		},
		{
			name:    "shows dash for local remote",
			remotes: []RemoteInfo{{Name: "origin", URL: "/srv/git/githand.git"}},
			want:    "-",
		},
		{
			name:    "shows dash for file URL remote",
			remotes: []RemoteInfo{{Name: "origin", URL: "file:///srv/git/githand.git"}},
			want:    "-",
		},
		{
			name:    "shows dash for malformed explicit URL",
			remotes: []RemoteInfo{{Name: "origin", URL: "https://%zz/githand.git"}},
			want:    "-",
		},
		{
			name:    "shows dash for Windows local remote",
			remotes: []RemoteInfo{{Name: "origin", URL: `C:\repos\githand.git`}},
			want:    "-",
		},
		{
			name: "shows dash without remotes",
			want: "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrimarySource(tt.remotes); got != tt.want {
				t.Fatalf("PrimarySource() = %q, want %q", got, tt.want)
			}
		})
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

func TestSyncRegistryRemovesMissing(t *testing.T) {
	// Create a temporary workspace
	workspace, err := os.MkdirTemp("", "githand-sync-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(workspace) })

	// Create a registry with a repo that doesn't exist
	reg := &config.Registry{
		BasePath: workspace,
		Repos: []config.Repo{
			{Name: "missing", Path: "/nonexistent/path/to/repo"},
		},
	}

	result, err := SyncRegistry(reg, false, false)
	if err != nil {
		t.Fatal(err)
	}

	if result.Removed != 1 {
		t.Errorf("expected 1 removed, got %d", result.Removed)
	}
	if len(reg.Repos) != 0 {
		t.Errorf("expected 0 repos after sync, got %d", len(reg.Repos))
	}
}

func TestSyncRegistryAddsNew(t *testing.T) {
	// Create a workspace with a git repo
	workspace, err := os.MkdirTemp("", "githand-sync-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(workspace) })

	// Create a git repo in the workspace
	repoDir := filepath.Join(workspace, "newrepo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repoDir, "init")
	mustGit(t, repoDir, "config", "user.email", "test@test.com")
	mustGit(t, repoDir, "config", "user.name", "Test")

	// Create a registry with BasePath set but no repos
	reg := &config.Registry{
		BasePath: workspace,
		Repos:    []config.Repo{},
		Groups:   make(map[string][]string),
	}

	result, err := SyncRegistry(reg, false, false)
	if err != nil {
		t.Fatal(err)
	}

	if result.Added != 1 {
		t.Errorf("expected 1 added, got %d", result.Added)
	}
	if len(reg.Repos) != 1 {
		t.Errorf("expected 1 repo after sync, got %d", len(reg.Repos))
	}
	if reg.Repos[0].Name != "newrepo" {
		t.Errorf("expected repo name 'newrepo', got %q", reg.Repos[0].Name)
	}
}

func TestSyncRegistryBoth(t *testing.T) {
	// Create a workspace with one git repo
	workspace, err := os.MkdirTemp("", "githand-sync-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(workspace) })

	// Create an existing repo
	existingDir := filepath.Join(workspace, "existing")
	if err := os.Mkdir(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, existingDir, "init")
	mustGit(t, existingDir, "config", "user.email", "test@test.com")
	mustGit(t, existingDir, "config", "user.name", "Test")

	// Create a new repo
	newDir := filepath.Join(workspace, "newrepo")
	if err := os.Mkdir(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, newDir, "init")
	mustGit(t, newDir, "config", "user.email", "test@test.com")
	mustGit(t, newDir, "config", "user.name", "Test")

	// Create a registry with one existing repo and one missing repo
	reg := &config.Registry{
		BasePath: workspace,
		Repos: []config.Repo{
			{Name: "existing", Path: existingDir},
			{Name: "missing", Path: "/nonexistent/path"},
		},
		Groups: make(map[string][]string),
	}

	result, err := SyncRegistry(reg, false, false)
	if err != nil {
		t.Fatal(err)
	}

	// Debug output
	t.Logf("Added: %d, Removed: %d", result.Added, result.Removed)
	for i, repo := range reg.Repos {
		t.Logf("Repo %d: %s at %s", i, repo.Name, repo.Path)
	}

	if result.Added != 1 {
		t.Errorf("expected 1 added, got %d", result.Added)
	}
	if result.Removed != 1 {
		t.Errorf("expected 1 removed, got %d", result.Removed)
	}
	if len(reg.Repos) != 2 {
		t.Errorf("expected 2 repos after sync, got %d", len(reg.Repos))
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
