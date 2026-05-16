package discover

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverFlat(t *testing.T) {
	root, _ := os.MkdirTemp("", "githand-discover-test-")
	defer os.RemoveAll(root)

	initRepoAt(t, filepath.Join(root, "repo-a"))
	initRepoAt(t, filepath.Join(root, "repo-b"))
	os.MkdirAll(filepath.Join(root, "not-git"), 0o755)

	repos, err := Discover(root, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	names := map[string]bool{}
	for _, r := range repos {
		names[r.Name] = true
	}
	if !names["repo-a"] || !names["repo-b"] {
		t.Errorf("expected repo-a and repo-b, got %v", names)
	}
}

func TestDiscoverRecursive(t *testing.T) {
	root, _ := os.MkdirTemp("", "githand-discover-test-")
	defer os.RemoveAll(root)

	initRepoAt(t, filepath.Join(root, "group1", "repo-x"))
	initRepoAt(t, filepath.Join(root, "group2", "repo-y"))

	repos, err := Discover(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
}

func TestDiscoverAutoGroup(t *testing.T) {
	root, _ := os.MkdirTemp("", "githand-discover-test-")
	defer os.RemoveAll(root)

	initRepoAt(t, filepath.Join(root, "nix", "expnix"))
	initRepoAt(t, filepath.Join(root, "agent", "githand"))
	// repo at root level — no group
	initRepoAt(t, filepath.Join(root, "standalone"))

	repos, err := Discover(root, true, true)
	if err != nil {
		t.Fatal(err)
	}

	groups := map[string]string{}
	for _, r := range repos {
		groups[r.Name] = r.Group
	}
	if groups["expnix"] != "nix" {
		t.Errorf("expnix group: expected nix, got %s", groups["expnix"])
	}
	if groups["githand"] != "agent" {
		t.Errorf("githand group: expected agent, got %s", groups["githand"])
	}
	if groups["standalone"] != "" {
		t.Errorf("standalone should have no group, got %s", groups["standalone"])
	}
}

func TestDiscoverSkipNestedGitDirs(t *testing.T) {
	root, _ := os.MkdirTemp("", "githand-discover-test-")
	defer os.RemoveAll(root)

	// repo-a with a nested git dir (submodule scenario)
	repoA := filepath.Join(root, "repo-a")
	initRepoAt(t, repoA)
	os.MkdirAll(filepath.Join(repoA, "subdir"), 0o755)
	initRepoAt(t, filepath.Join(repoA, "subdir", "repo-b"))

	// non-recursive: should only find repo-a
	repos, err := Discover(root, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Name != "repo-a" {
		t.Errorf("non-recursive should find only repo-a, got %v", repos)
	}
}

func TestDiscoverSkipHiddenDirs(t *testing.T) {
	root, _ := os.MkdirTemp("", "githand-discover-test-")
	defer os.RemoveAll(root)

	// hidden dir with a git repo — should be skipped
	initRepoAt(t, filepath.Join(root, ".hidden", "repo-secret"))
	// visible dir with a git repo — should be found
	initRepoAt(t, filepath.Join(root, "visible", "repo-public"))

	repos, err := Discover(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Name != "repo-public" {
		t.Errorf("should skip hidden dirs, got %v", repos)
	}
}

func TestDiscoverDeduplicate(t *testing.T) {
	root, _ := os.MkdirTemp("", "githand-discover-test-")
	defer os.RemoveAll(root)

	initRepoAt(t, filepath.Join(root, "repo-a"))

	// scan twice — should deduplicate
	repos1, _ := Discover(root, false, false)
	repos2, _ := Discover(root, false, false)
	if len(repos1) != 1 || len(repos2) != 1 {
		t.Error("should find 1 repo each time")
	}
}

func initRepoAt(t *testing.T, dir string) {
	t.Helper()
	os.MkdirAll(dir, 0o755)
	gitCmd(t, dir, "init")
	gitCmd(t, dir, "config", "user.email", "test@test.com")
	gitCmd(t, dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("init"), 0o644)
	gitCmd(t, dir, "add", "README.md")
	gitCmd(t, dir, "commit", "-m", "initial")
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}