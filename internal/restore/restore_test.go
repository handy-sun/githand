package restore

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/handy-sun/githand/internal/config"
	"github.com/handy-sun/githand/internal/snapshot"
)

// TestRestoreCleanRepo clones a repo from a snapshot and verifies it exists.
func TestRestoreCleanRepo(t *testing.T) {
	bareDir := initBareRepo(t, "source-repo")
	commit := getHEAD(t, bareDir)
	snapPath := writeTestSnapshot(t, snapshot.RepoSnap{
		Name:          "source-repo",
		RelPath:       "source-repo",
		CurrentBranch: "main",
		HeadCommit:    commit,
		Remotes:       []snapshot.RemoteSnap{{Name: "origin", URL: bareDir}},
		Branches:      []snapshot.BranchSnap{{Name: "main", Upstream: "origin/main"}},
	})

	targetDir, _ := os.MkdirTemp("", "githand-restore-test-")
	defer os.RemoveAll(targetDir)

	err := Run(snapPath, targetDir, "", false)
	if err != nil {
		t.Fatal(err)
	}

	repoDir := filepath.Join(targetDir, "source-repo")
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil {
		t.Errorf("restored repo should exist: %v", err)
	}
}

// TestRestoreFromDirectory verifies that a snapshot directory works as input.
func TestRestoreFromDirectory(t *testing.T) {
	bareDir := initBareRepo(t, "dir-repo")
	commit := getHEAD(t, bareDir)
	// writeTestSnapshot returns the directory path
	snapDir := writeTestSnapshot(t, snapshot.RepoSnap{
		Name:          "dir-repo",
		RelPath:       "dir-repo",
		CurrentBranch: "main",
		HeadCommit:    commit,
		Remotes:       []snapshot.RemoteSnap{{Name: "origin", URL: bareDir}},
	})

	targetDir, _ := os.MkdirTemp("", "githand-restore-test-")
	defer os.RemoveAll(targetDir)

	// pass the directory path (not the json file) — should find snapshot.json inside
	err := Run(snapDir, targetDir, "", false)
	if err != nil {
		t.Fatal(err)
	}

	repoDir := filepath.Join(targetDir, "dir-repo")
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil {
		t.Errorf("restored repo should exist: %v", err)
	}
}

// TestRestoreWithPatch uses the real snapshot.Take to generate patches,
// then restores them to verify round-trip fidelity.
func TestRestoreWithPatch(t *testing.T) {
	// create a source repo with staged + unstaged changes
	srcDir := initWorkRepo(t, "patched-repo")
	os.WriteFile(filepath.Join(srcDir, "new.txt"), []byte("new content\n"), 0o644)
	gitCmd(t, srcDir, "add", "new.txt")
	os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("modified\n"), 0o644)

	parent := filepath.Dir(srcDir)
	reg := &config.Registry{
		Version:  1,
		BasePath: parent,
		Repos:    []config.Repo{{Name: "patched-repo", Path: srcDir}},
	}

	snap, err := snapshot.Take(reg, reg.Repos, true)
	if err != nil {
		t.Fatal(err)
	}

	// write snapshot to temp directory using the new folder structure
	tmpDir, _ := os.MkdirTemp("", "githand-restore-patch-")
	defer os.RemoveAll(tmpDir)
	snapDir := filepath.Join(tmpDir, "githand-snapshot.test")
	if err := snapshot.Write(snap, snapDir, parent); err != nil {
		t.Fatal(err)
	}

	// restore into a new target
	targetDir, _ := os.MkdirTemp("", "githand-restore-target-")
	defer os.RemoveAll(targetDir)

	// pass directory path
	err = Run(snapDir, targetDir, "", false)
	if err != nil {
		t.Fatal(err)
	}

	repoDir := filepath.Join(targetDir, "patched-repo")
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil {
		t.Fatalf("restored repo should exist: %v", err)
	}
	// staged file should exist
	if _, err := os.Stat(filepath.Join(repoDir, "new.txt")); err != nil {
		t.Errorf("staged file new.txt should exist: %v", err)
	}
}

// TestRestoreDryRun verifies no files are created.
func TestRestoreDryRun(t *testing.T) {
	bareDir := initBareRepo(t, "dryrun-repo")
	commit := getHEAD(t, bareDir)
	snapPath := writeTestSnapshot(t, snapshot.RepoSnap{
		Name:          "dryrun-repo",
		RelPath:       "dryrun-repo",
		CurrentBranch: "main",
		HeadCommit:    commit,
		Remotes:       []snapshot.RemoteSnap{{Name: "origin", URL: bareDir}},
	})

	targetDir, _ := os.MkdirTemp("", "githand-restore-test-")
	defer os.RemoveAll(targetDir)

	err := Run(snapPath, targetDir, "", true)
	if err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(targetDir)
	if len(entries) != 0 {
		t.Error("dry-run should not create any files")
	}
}

// TestRestoreDetachedHEAD verifies checkout of a specific commit.
func TestRestoreDetachedHEAD(t *testing.T) {
	bareDir := initBareRepo(t, "detached-repo")
	commit := getHEAD(t, bareDir)
	snapPath := writeTestSnapshot(t, snapshot.RepoSnap{
		Name:       "detached-repo",
		RelPath:    "detached-repo",
		Detached:   true,
		HeadCommit: commit,
		Remotes:    []snapshot.RemoteSnap{{Name: "origin", URL: bareDir}},
	})

	targetDir, _ := os.MkdirTemp("", "githand-restore-test-")
	defer os.RemoveAll(targetDir)

	err := Run(snapPath, targetDir, "", false)
	if err != nil {
		t.Fatal(err)
	}

	repoDir := filepath.Join(targetDir, "detached-repo")
	out := gitOut(t, repoDir, "rev-parse", "HEAD")
	if out != commit {
		t.Errorf("HEAD should be %s, got %s", commit, out)
	}
}

// TestRestoreWithStash verifies stash recreation.
func TestRestoreWithStash(t *testing.T) {
	srcDir := initWorkRepo(t, "stash-repo")
	os.WriteFile(filepath.Join(srcDir, "stash.txt"), []byte("stash content\n"), 0o644)
	gitCmd(t, srcDir, "add", "stash.txt")
	gitCmd(t, srcDir, "stash")

	parent := filepath.Dir(srcDir)
	reg := &config.Registry{
		Version:  1,
		BasePath: parent,
		Repos:    []config.Repo{{Name: "stash-repo", Path: srcDir}},
	}

	snap, err := snapshot.Take(reg, reg.Repos, true)
	if err != nil {
		t.Fatal(err)
	}

	tmpDir, _ := os.MkdirTemp("", "githand-restore-stash-")
	defer os.RemoveAll(tmpDir)
	snapDir := filepath.Join(tmpDir, "githand-snapshot.test")
	if err := snapshot.Write(snap, snapDir, parent); err != nil {
		t.Fatal(err)
	}

	targetDir, _ := os.MkdirTemp("", "githand-restore-target-")
	defer os.RemoveAll(targetDir)

	err = Run(snapDir, targetDir, "", false)
	if err != nil {
		t.Fatal(err)
	}

	repoDir := filepath.Join(targetDir, "stash-repo")
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil {
		t.Fatalf("restored repo should exist: %v", err)
	}
}

// TestRestoreNestedPaths verifies that repos in group subdirectories
// are restored to the correct nested path under targetDir.
func TestRestoreNestedPaths(t *testing.T) {
	parent, _ := os.MkdirTemp("", "githand-restore-nested-")
	defer os.RemoveAll(parent)

	// create two repos in nested dirs like a real workspace
	nixDir := filepath.Join(parent, "nix", "expnix")
	os.MkdirAll(nixDir, 0o755)
	gitCmd(t, nixDir, "init")
	gitCmd(t, nixDir, "config", "user.email", "test@test.com")
	gitCmd(t, nixDir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(nixDir, "README.md"), []byte("nix\n"), 0o644)
	gitCmd(t, nixDir, "add", "README.md")
	gitCmd(t, nixDir, "commit", "-m", "initial")

	agentDir := filepath.Join(parent, "agent", "githand")
	os.MkdirAll(agentDir, 0o755)
	gitCmd(t, agentDir, "init")
	gitCmd(t, agentDir, "config", "user.email", "test@test.com")
	gitCmd(t, agentDir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(agentDir, "README.md"), []byte("agent\n"), 0o644)
	gitCmd(t, agentDir, "add", "README.md")
	gitCmd(t, agentDir, "commit", "-m", "initial")

	// create bare clones as remotes (needed for restore to clone)
	nixBare := filepath.Join(parent, "expnix.git")
	gitCmd(t, parent, "clone", "--bare", nixDir, nixBare)
	gitCmd(t, nixDir, "remote", "add", "origin", nixBare)
	gitCmd(t, nixDir, "push", "-u", "origin", "HEAD")

	agentBare := filepath.Join(parent, "githand.git")
	gitCmd(t, parent, "clone", "--bare", agentDir, agentBare)
	gitCmd(t, agentDir, "remote", "add", "origin", agentBare)
	gitCmd(t, agentDir, "push", "-u", "origin", "HEAD")

	reg := &config.Registry{
		Version:  1,
		BasePath: parent,
		Repos: []config.Repo{
			{Name: "expnix", Path: nixDir, Group: "nix"},
			{Name: "githand", Path: agentDir, Group: "agent"},
		},
	}

	snap, err := snapshot.Take(reg, reg.Repos, true)
	if err != nil {
		t.Fatal(err)
	}

	tmpDir, _ := os.MkdirTemp("", "githand-restore-nested-snap-")
	defer os.RemoveAll(tmpDir)
	snapDir := filepath.Join(tmpDir, "githand-snapshot.test")
	if err := snapshot.Write(snap, snapDir, parent); err != nil {
		t.Fatal(err)
	}

	targetDir, _ := os.MkdirTemp("", "githand-restore-target-")
	defer os.RemoveAll(targetDir)

	err = Run(snapDir, targetDir, "", false)
	if err != nil {
		t.Fatal(err)
	}

	// verify nested paths: nix/expnix and agent/githand
	if _, err := os.Stat(filepath.Join(targetDir, "nix", "expnix", ".git")); err != nil {
		t.Errorf("nix/expnix should be restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "agent", "githand", ".git")); err != nil {
		t.Errorf("agent/githand should be restored: %v", err)
	}
}

// --- helpers ---

// initWorkRepo creates a working git repo (not bare) with an initial commit
// and an origin remote pointing to a bare clone.
func initWorkRepo(t *testing.T, name string) string {
	t.Helper()
	parent, _ := os.MkdirTemp("", "githand-restore-src-")
	t.Cleanup(func() { os.RemoveAll(parent) })

	workDir := filepath.Join(parent, name)
	os.MkdirAll(workDir, 0o755)
	gitCmd(t, workDir, "init")
	gitCmd(t, workDir, "config", "user.email", "test@test.com")
	gitCmd(t, workDir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(workDir, "README.md"), []byte("hello\n"), 0o644)
	gitCmd(t, workDir, "add", "README.md")
	gitCmd(t, workDir, "commit", "-m", "initial")

	// create bare clone as "remote"
	bareDir := filepath.Join(parent, name+".git")
	gitCmd(t, parent, "clone", "--bare", workDir, bareDir)
	gitCmd(t, workDir, "remote", "add", "origin", bareDir)
	gitCmd(t, workDir, "push", "-u", "origin", "HEAD")

	return workDir
}

// initBareRepo creates just a bare repo for snapshot references.
func initBareRepo(t *testing.T, name string) string {
	t.Helper()
	parent, _ := os.MkdirTemp("", "githand-restore-src-")
	t.Cleanup(func() { os.RemoveAll(parent) })

	workDir := filepath.Join(parent, name)
	os.MkdirAll(workDir, 0o755)
	gitCmd(t, workDir, "init")
	gitCmd(t, workDir, "config", "user.email", "test@test.com")
	gitCmd(t, workDir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(workDir, "README.md"), []byte("hello\n"), 0o644)
	gitCmd(t, workDir, "add", "README.md")
	gitCmd(t, workDir, "commit", "-m", "initial")

	bareDir := filepath.Join(parent, name+".git")
	gitCmd(t, parent, "clone", "--bare", workDir, bareDir)
	return bareDir
}

func getHEAD(t *testing.T, bareDir string) string {
	t.Helper()
	return gitOut(t, bareDir, "rev-parse", "HEAD")
}

// writeTestSnapshot creates a snapshot directory with snapshot.json inside.
// Returns the directory path (suitable for passing to Run as snapPath).
func writeTestSnapshot(t *testing.T, rs snapshot.RepoSnap) string {
	t.Helper()
	tmpDir, _ := os.MkdirTemp("", "githand-snap-file-")
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	snap := &snapshot.Snapshot{
		Schema:   1,
		BasePath: "/test",
		Repos:    []snapshot.RepoSnap{rs},
	}

	// write snapshot.json inside the temp directory
	snapDir := filepath.Join(tmpDir, "githand-snapshot.test")
	os.MkdirAll(snapDir, 0o755)
	jsonPath := filepath.Join(snapDir, snapshot.SnapshotJSONName)

	data, _ := json.MarshalIndent(snap, "", "  ")
	os.WriteFile(jsonPath, data, 0o644)

	return snapDir
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

func configureGitIdentity(t *testing.T, dir string) {
	t.Helper()
	gitCmd(t, dir, "config", "user.email", "test@test.com")
	gitCmd(t, dir, "config", "user.name", "Test")
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s in %s: %v", strings.Join(args, " "), dir, err)
	}
	return strings.TrimSpace(string(out))
}

// TestRestoreExistingRepo verifies that restoring into an existing repo
// updates it instead of failing.
func TestRestoreExistingRepo(t *testing.T) {
	// Create initial repo and snapshot
	bareDir := initBareRepo(t, "existing-repo")
	commit := getHEAD(t, bareDir)
	snapPath := writeTestSnapshot(t, snapshot.RepoSnap{
		Name:          "existing-repo",
		RelPath:       "existing-repo",
		CurrentBranch: "main",
		HeadCommit:    commit,
		Remotes:       []snapshot.RemoteSnap{{Name: "origin", URL: bareDir}},
	})

	targetDir, _ := os.MkdirTemp("", "githand-restore-test-")
	defer os.RemoveAll(targetDir)

	// First restore - should clone
	err := Run(snapPath, targetDir, "", false)
	if err != nil {
		t.Fatal(err)
	}

	repoDir := filepath.Join(targetDir, "existing-repo")
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil {
		t.Fatalf("restored repo should exist: %v", err)
	}

	// Make a change in the bare repo
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	gitCmd(t, tmpDir, "clone", bareDir, workDir)
	configureGitIdentity(t, workDir)
	os.WriteFile(filepath.Join(workDir, "new.txt"), []byte("new content\n"), 0o644)
	gitCmd(t, workDir, "add", "new.txt")
	gitCmd(t, workDir, "commit", "-m", "add new file")
	gitCmd(t, workDir, "push")

	// Create new snapshot with updated commit
	newCommit := getHEAD(t, workDir)
	snapPath2 := writeTestSnapshot(t, snapshot.RepoSnap{
		Name:          "existing-repo",
		RelPath:       "existing-repo",
		CurrentBranch: "main",
		HeadCommit:    newCommit,
		Remotes:       []snapshot.RemoteSnap{{Name: "origin", URL: bareDir}},
	})

	// Second restore - should update existing repo
	err = Run(snapPath2, targetDir, "", false)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the new file exists
	if _, err := os.Stat(filepath.Join(repoDir, "new.txt")); err != nil {
		t.Errorf("new file should exist after update: %v", err)
	}
}

func TestRestoreExistingRepoDoesNotDiscardLocalChanges(t *testing.T) {
	bareDir := initBareRepo(t, "dirty-existing-repo")
	commit := getHEAD(t, bareDir)
	snapPath := writeTestSnapshot(t, snapshot.RepoSnap{
		Name:          "dirty-existing-repo",
		RelPath:       "dirty-existing-repo",
		CurrentBranch: "main",
		HeadCommit:    commit,
		Remotes:       []snapshot.RemoteSnap{{Name: "origin", URL: bareDir}},
	})

	targetDir := t.TempDir()
	if err := Run(snapPath, targetDir, "", false); err != nil {
		t.Fatal(err)
	}

	repoDir := filepath.Join(targetDir, "dirty-existing-repo")
	localContent := []byte("local edit\n")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), localContent, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run(snapPath, targetDir, "", false); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(repoDir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(localContent) {
		t.Fatalf("restore should not discard local changes, got %q", got)
	}
}

func TestRestoreExistingRepoUsesSnapshotRemoteAndCommit(t *testing.T) {
	correctBare := initBareRepo(t, "correct-remote")
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	gitCmd(t, tmpDir, "clone", correctBare, workDir)
	configureGitIdentity(t, workDir)
	if err := os.WriteFile(filepath.Join(workDir, "correct.txt"), []byte("correct\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, workDir, "add", "correct.txt")
	gitCmd(t, workDir, "commit", "-m", "add correct file")
	gitCmd(t, workDir, "push")
	correctCommit := getHEAD(t, workDir)

	wrongBare := initBareRepo(t, "wrong-remote")
	targetDir := t.TempDir()
	repoDir := filepath.Join(targetDir, "existing-repo")
	gitCmd(t, targetDir, "clone", wrongBare, repoDir)

	snapPath := writeTestSnapshot(t, snapshot.RepoSnap{
		Name:          "existing-repo",
		RelPath:       "existing-repo",
		CurrentBranch: "main",
		HeadCommit:    correctCommit,
		Remotes:       []snapshot.RemoteSnap{{Name: "origin", URL: correctBare}},
		Branches:      []snapshot.BranchSnap{{Name: "main", Upstream: "origin/main"}},
	})

	if err := Run(snapPath, targetDir, "", false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(repoDir, "correct.txt")); err != nil {
		t.Fatalf("existing repo should be updated from snapshot remote and commit: %v", err)
	}
	head := gitOut(t, repoDir, "rev-parse", "HEAD")
	if head != correctCommit {
		t.Fatalf("HEAD should match snapshot commit %s, got %s", correctCommit, head)
	}
}

func TestRestoreExistingRepoReturnsErrorWhenSnapshotCommitMissing(t *testing.T) {
	bareDir := initBareRepo(t, "missing-commit")
	targetDir := t.TempDir()
	repoDir := filepath.Join(targetDir, "missing-commit")
	gitCmd(t, targetDir, "clone", bareDir, repoDir)

	err := restoreRepo(snapshot.RepoSnap{
		Name:          "missing-commit",
		RelPath:       "missing-commit",
		CurrentBranch: "main",
		HeadCommit:    strings.Repeat("0", 40),
		Remotes:       []snapshot.RemoteSnap{{Name: "origin", URL: bareDir}},
	}, repoDir, t.TempDir())
	if err == nil {
		t.Fatal("expected missing snapshot commit to fail")
	}
}
