// Package testutil provides helpers for integration tests that use real git
// commands and temporary directories.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TempDir creates a temporary directory and registers cleanup.
func TempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "githand-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// InitGitRepo creates a git repo with an initial commit in dir.
// The repo will have one file "README.md" with content "hello".
func InitGitRepo(t *testing.T, dir string) {
	t.Helper()
	mustRun(t, dir, "git", "init")
	mustRun(t, dir, "git", "config", "user.email", "test@test.com")
	mustRun(t, dir, "git", "config", "user.name", "Test")
	writeFile(t, filepath.Join(dir, "README.md"), "hello")
	mustRun(t, dir, "git", "add", "README.md")
	mustRun(t, dir, "git", "commit", "-m", "initial")
}

// MakeDirty adds an unstaged change to the repo.
func MakeDirty(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "dirty.txt"), "unstaged change")
}

// MakeStaged adds a staged change to the repo.
func MakeStaged(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "staged.txt"), "staged change")
	mustRun(t, dir, "git", "add", "staged.txt")
}

// MakeStash creates a stash entry with the given file.
func MakeStash(t *testing.T, dir string, filename, content string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, filename), content)
	mustRun(t, dir, "git", "add", filename)
	mustRun(t, dir, "git", "stash")
}

// MakeUntracked creates an untracked file.
func MakeUntracked(t *testing.T, dir string, filename, content string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, filename), content)
}

// AddRemote adds a remote to the repo.
func AddRemote(t *testing.T, dir, name, url string) {
	t.Helper()
	mustRun(t, dir, "git", "remote", "add", name, url)
}

// DetachHEAD switches to a detached HEAD state.
func DetachHEAD(t *testing.T, dir string) {
	t.Helper()
	mustRun(t, dir, "git", "checkout", "HEAD")
}

// CreateBranch creates and switches to a new branch.
func CreateBranch(t *testing.T, dir, branch string) {
	t.Helper()
	mustRun(t, dir, "git", "checkout", "-b", branch)
}

// writeFile writes content to path, creating parent dirs if needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run %v in %s: %v\n%s", args, dir, err, out)
	}
}
