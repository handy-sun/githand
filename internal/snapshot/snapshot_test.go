package snapshot

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/handy-sun/githand/internal/config"
	"github.com/handy-sun/githand/internal/git"
)

func TestSnapshotCleanRepo(t *testing.T) {
	dir := initTestRepo(t, "repo-a")
	parent := filepath.Dir(dir)
	reg := &config.Registry{
		Version:  1,
		BasePath: parent,
		Repos:    []config.Repo{{Name: "repo-a", Path: dir}},
	}

	snap, err := Take(reg, reg.Repos, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Repos) != 1 {
		t.Fatalf("expected 1 repo in snapshot, got %d", len(snap.Repos))
	}
	rs := snap.Repos[0]
	if rs.Name != "repo-a" {
		t.Errorf("expected repo-a, got %s", rs.Name)
	}
	if rs.Dirty {
		t.Error("clean repo should not be marked dirty")
	}
	if rs.CurrentBranch == "" {
		t.Error("should have a current branch")
	}
	if len(rs.Remotes) == 0 {
		t.Error("should have at least origin remote")
	}
}

func TestSnapshotDirtyRepo(t *testing.T) {
	dir := initTestRepo(t, "repo-dirty")
	// staged change
	os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged"), 0o644)
	gitRun(t, dir, "add", "staged.txt")
	// unstaged change: modify tracked file
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("modified"), 0o644)

	parent := filepath.Dir(dir)
	reg := &config.Registry{
		Version:  1,
		BasePath: parent,
		Repos:    []config.Repo{{Name: "repo-dirty", Path: dir}},
	}

	snap, err := Take(reg, reg.Repos, true)
	if err != nil {
		t.Fatal(err)
	}
	rs := snap.Repos[0]
	if !rs.Dirty {
		t.Error("dirty repo should be marked dirty")
	}
	if rs.StagedPatch == "" {
		t.Error("should have staged patch")
	}
	if rs.UnstagedPatch == "" {
		t.Error("should have unstaged patch")
	}
}

func TestSnapshotWithStash(t *testing.T) {
	dir := initTestRepo(t, "repo-stash")
	os.WriteFile(filepath.Join(dir, "stash.txt"), []byte("stash content"), 0o644)
	gitRun(t, dir, "add", "stash.txt")
	gitRun(t, dir, "stash")

	parent := filepath.Dir(dir)
	reg := &config.Registry{
		Version:  1,
		BasePath: parent,
		Repos:    []config.Repo{{Name: "repo-stash", Path: dir}},
	}

	snap, err := Take(reg, reg.Repos, true)
	if err != nil {
		t.Fatal(err)
	}
	rs := snap.Repos[0]
	if len(rs.Stashes) != 1 {
		t.Errorf("expected 1 stash, got %d", len(rs.Stashes))
	}
	if rs.Stashes[0].Patch == "" {
		t.Error("stash patch should not be empty")
	}
}

func TestSnapshotWithUntracked(t *testing.T) {
	dir := initTestRepo(t, "repo-untracked")
	os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("new file"), 0o644)

	parent := filepath.Dir(dir)
	reg := &config.Registry{
		Version:  1,
		BasePath: parent,
		Repos:    []config.Repo{{Name: "repo-untracked", Path: dir}},
	}

	snap, err := Take(reg, reg.Repos, true)
	if err != nil {
		t.Fatal(err)
	}
	rs := snap.Repos[0]
	if len(rs.Untracked) == 0 {
		t.Error("should have untracked files")
	}
	found := false
	for _, f := range rs.Untracked {
		if f == "untracked.txt" {
			found = true
		}
	}
	if !found {
		t.Error("untracked.txt should be in the list")
	}
}

func TestSnapshotExcludeClean(t *testing.T) {
	cleanDir := initTestRepo(t, "clean-repo")
	dirtyDir := initTestRepo(t, "dirty-repo")
	os.WriteFile(filepath.Join(dirtyDir, "dirty.txt"), []byte("change"), 0o644)

	parent := filepath.Dir(cleanDir)
	reg := &config.Registry{
		Version:  1,
		BasePath: parent,
		Repos: []config.Repo{
			{Name: "clean-repo", Path: cleanDir},
			{Name: "dirty-repo", Path: dirtyDir},
		},
	}

	snap, err := Take(reg, reg.Repos, false) // includeClean = false
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Repos) != 1 {
		t.Fatalf("should only include dirty repo, got %d", len(snap.Repos))
	}
	if snap.Repos[0].Name != "dirty-repo" {
		t.Errorf("expected dirty-repo, got %s", snap.Repos[0].Name)
	}
}

func TestSnapshotWriteJSON(t *testing.T) {
	dir := initTestRepo(t, "repo-write")
	parent := filepath.Dir(dir)
	reg := &config.Registry{
		Version:  1,
		BasePath: parent,
		Repos:    []config.Repo{{Name: "repo-write", Path: dir}},
	}

	snap, err := Take(reg, reg.Repos, true)
	if err != nil {
		t.Fatal(err)
	}

	tmpDir, _ := os.MkdirTemp("", "githand-snap-test-")
	defer os.RemoveAll(tmpDir)
	snapDir := filepath.Join(tmpDir, "githand-snapshot.0101-120000")

	if err := Write(snap, snapDir, parent); err != nil {
		t.Fatal(err)
	}

	jsonPath := filepath.Join(snapDir, SnapshotJSONName)
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var loaded Snapshot
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}
	if loaded.Schema != 1 {
		t.Error("schema should be 1")
	}
	if len(loaded.Repos) != 1 {
		t.Error("should have 1 repo")
	}
}

func TestSnapshotDetachedHEAD(t *testing.T) {
	dir := initTestRepo(t, "repo-detached")
	commit, _ := git.HEADCommit(dir)
	gitRun(t, dir, "checkout", commit) // detach

	parent := filepath.Dir(dir)
	reg := &config.Registry{
		Version:  1,
		BasePath: parent,
		Repos:    []config.Repo{{Name: "repo-detached", Path: dir}},
	}

	snap, err := Take(reg, reg.Repos, true)
	if err != nil {
		t.Fatal(err)
	}
	rs := snap.Repos[0]
	if !rs.Detached {
		t.Error("should be detached")
	}
	if rs.CurrentBranch != "" {
		t.Errorf("detached HEAD should have empty current_branch, got %s", rs.CurrentBranch)
	}
}

func TestFilterDirty(t *testing.T) {
	repos := []RepoSnap{
		{Name: "a", Dirty: true},
		{Name: "b", Dirty: false},
		{Name: "c", Dirty: true},
	}
	filtered := Filter(repos, "dirty")
	if len(filtered) != 2 {
		t.Errorf("expected 2 dirty repos, got %d", len(filtered))
	}
}

func TestFilterDetached(t *testing.T) {
	repos := []RepoSnap{
		{Name: "a", Detached: false},
		{Name: "b", Detached: true},
	}
	filtered := Filter(repos, "detached")
	if len(filtered) != 1 || filtered[0].Name != "b" {
		t.Errorf("expected b, got %v", filtered)
	}
}

func TestRelPath(t *testing.T) {
	r := relPath("/Users/qi/work", "/Users/qi/work/githand")
	if r != "githand" {
		t.Errorf("expected githand, got %s", r)
	}
	r = relPath("/Users/qi/work", "/Users/qi/work/nix/expnix")
	if r != "nix/expnix" {
		t.Errorf("expected nix/expnix, got %s", r)
	}
}

func TestWriteUntrackedFiles(t *testing.T) {
	dir := initTestRepo(t, "repo-ut-write")
	os.WriteFile(filepath.Join(dir, "newfile.txt"), []byte("hello"), 0o644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(dir, "subdir", "nested.txt"), []byte("nested"), 0o644)

	parent := filepath.Dir(dir)
	reg := &config.Registry{
		Version:  1,
		BasePath: parent,
		Repos:    []config.Repo{{Name: "repo-ut-write", Path: dir}},
	}

	snap, err := Take(reg, reg.Repos, true)
	if err != nil {
		t.Fatal(err)
	}

	tmpDir, _ := os.MkdirTemp("", "githand-ut-test-")
	defer os.RemoveAll(tmpDir)
	snapDir := filepath.Join(tmpDir, "githand-snapshot.test")

	if err := Write(snap, snapDir, parent); err != nil {
		t.Fatal(err)
	}

	// verify untracked files are copied directly (not tar.gz)
	utDir := filepath.Join(snapDir, "untracked", "repo-ut-write")
	if _, err := os.Stat(filepath.Join(utDir, "newfile.txt")); err != nil {
		t.Errorf("untracked newfile.txt should be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(utDir, "subdir", "nested.txt")); err != nil {
		t.Errorf("untracked subdir/nested.txt should be copied: %v", err)
	}

	// verify no tar.gz archive exists
	if _, err := os.Stat(filepath.Join(snapDir, "untracked", "repo-ut-write.tar.gz")); !os.IsNotExist(err) {
		t.Error("tar.gz archive should not exist")
	}
}

func initTestRepo(t *testing.T, name string) string {
	t.Helper()
	parent, err := os.MkdirTemp("", "githand-snap-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(parent) })

	repoDir := filepath.Join(parent, name)
	os.MkdirAll(repoDir, 0o755)
	gitRun(t, repoDir, "init")
	gitRun(t, repoDir, "config", "user.email", "test@test.com")
	gitRun(t, repoDir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello"), 0o644)
	gitRun(t, repoDir, "add", "README.md")
	gitRun(t, repoDir, "commit", "-m", "initial")

	// add origin remote pointing to a dummy bare repo
	bareDir := filepath.Join(parent, name + "-bare.git")
	os.MkdirAll(bareDir, 0o755)
	gitRun(t, bareDir, "init", "--bare")
	gitRun(t, repoDir, "remote", "add", "origin", bareDir)

	return repoDir
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}