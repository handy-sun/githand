package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupRepo creates a temp git repo with one initial commit.
func setupRepo(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "githand-git-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	must(t, dir, "init")
	must(t, dir, "config", "user.email", "test@test.com")
	must(t, dir, "config", "user.name", "Test")
	write(t, filepath.Join(dir, "README.md"), "hello")
	must(t, dir, "add", "README.md")
	must(t, dir, "commit", "-m", "initial")
	return dir
}

func must(t *testing.T, dir string, args ...string) {
	t.Helper()
	if err := RunSilent(dir, args...); err != nil {
		t.Fatalf("git %s in %s: %v", strings.Join(args, " "), dir, err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsRepo(t *testing.T) {
	dir := setupRepo(t)
	if !IsRepo(dir) {
		t.Error("should be a git repo")
	}
	empty, _ := os.MkdirTemp("", "not-a-repo-")
	defer os.RemoveAll(empty)
	if IsRepo(empty) {
		t.Error("empty dir should not be a git repo")
	}
}

func TestTopLevel(t *testing.T) {
	dir := setupRepo(t)
	top, err := TopLevel(dir)
	if err != nil {
		t.Fatal(err)
	}
	// resolve both to handle macOS /var -> /private/var symlink
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	resolvedTop, _ := filepath.EvalSymlinks(top)
	if resolvedTop != resolvedDir {
		t.Errorf("TopLevel: expected %s, got %s", resolvedDir, resolvedTop)
	}
}

func TestCurrentBranch(t *testing.T) {
	dir := setupRepo(t)
	branch := CurrentBranch(dir)
	if branch == "" {
		t.Error("should have a branch name")
	}
	if branch != "main" && branch != "master" {
		t.Errorf("expected main or master, got %s", branch)
	}
}

func TestHEADCommit(t *testing.T) {
	dir := setupRepo(t)
	commit, err := HEADCommit(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(commit) != 40 {
		t.Errorf("commit hash should be 40 chars, got %d", len(commit))
	}
}

func TestIsDirty(t *testing.T) {
	dir := setupRepo(t)
	if IsDirty(dir) {
		t.Error("clean repo should not be dirty")
	}
	write(t, filepath.Join(dir, "new.txt"), "change")
	if !IsDirty(dir) {
		t.Error("repo with new file should be dirty")
	}
}

func TestStagedPatch(t *testing.T) {
	dir := setupRepo(t)
	write(t, filepath.Join(dir, "staged.txt"), "staged content")
	must(t, dir, "add", "staged.txt")

	patch, err := StagedPatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if patch == "" {
		t.Error("staged patch should not be empty")
	}
	if !strings.Contains(patch, "staged.txt") {
		t.Error("staged patch should mention staged.txt")
	}
}

func TestUnstagedPatch(t *testing.T) {
	dir := setupRepo(t)
	write(t, filepath.Join(dir, "README.md"), "modified")

	patch, err := UnstagedPatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if patch == "" {
		t.Error("unstaged patch should not be empty")
	}
}

func TestStashList(t *testing.T) {
	dir := setupRepo(t)

	list, err := StashList(dir)
	if err != nil {
		t.Fatal(err)
	}
	if list != "" {
		t.Error("empty repo should have no stashes")
	}

	write(t, filepath.Join(dir, "stash.txt"), "stash me")
	must(t, dir, "add", "stash.txt")
	must(t, dir, "stash")

	list, err = StashList(dir)
	if err != nil {
		t.Fatal(err)
	}
	if list == "" {
		t.Error("should have one stash entry")
	}
}

func TestStashCount(t *testing.T) {
	dir := setupRepo(t)
	if StashCount(dir) != 0 {
		t.Error("should have 0 stashes")
	}

	write(t, filepath.Join(dir, "s1.txt"), "one")
	must(t, dir, "add", "s1.txt")
	must(t, dir, "stash")

	if StashCount(dir) != 1 {
		t.Error("should have 1 stash")
	}
}

func TestUntrackedFiles(t *testing.T) {
	dir := setupRepo(t)
	write(t, filepath.Join(dir, "untracked.txt"), "new")

	out, err := UntrackedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "untracked.txt") {
		t.Error("should list untracked.txt")
	}
}

func TestRemotes(t *testing.T) {
	dir := setupRepo(t)
	must(t, dir, "remote", "add", "origin", "https://example.com/repo.git")

	out, err := Remotes(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "origin") || !strings.Contains(out, "example.com") {
		t.Error("should list origin remote")
	}
}

func TestBranches(t *testing.T) {
	dir := setupRepo(t)
	must(t, dir, "checkout", "-b", "feature")
	must(t, dir, "checkout", "-b", "bugfix")

	out, err := Branches(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "feature") || !strings.Contains(out, "bugfix") {
		t.Errorf("branches should include feature and bugfix, got: %s", out)
	}
}

func TestIsDetached(t *testing.T) {
	dir := setupRepo(t)
	if IsDetached(dir) {
		t.Error("on a branch should not be detached")
	}

	commit, _ := HEADCommit(dir)
	must(t, dir, "checkout", commit)
	if !IsDetached(dir) {
		t.Error("after checkout by hash should be detached")
	}
}

func TestAheadBehind(t *testing.T) {
	dir := setupRepo(t)
	ahead, behind, err := AheadBehind(dir)
	if err != nil {
		t.Fatal(err)
	}
	// no upstream set, should return 0, 0
	if ahead != 0 || behind != 0 {
		t.Errorf("no upstream: expected 0/0, got %d/%d", ahead, behind)
	}
}