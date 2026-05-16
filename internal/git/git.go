// Package git wraps the system git command for repository operations.
//
// All operations go through os/exec to invoke the system git binary.
// This project depends on porcelain behavior (diff, apply, stash, clone, etc.)
// that should not be reimplemented in pure Go.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Run executes git with the given args in the specified directory.
// Returns stdout, or an error that includes stderr.
func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// RunSilent is like Run but discards stdout. Useful for commands where
// we only care about success/failure.
func RunSilent(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return nil
}

// IsRepo returns true if dir is inside a git working tree.
func IsRepo(dir string) bool {
	_, err := Run(dir, "rev-parse", "--git-dir")
	return err == nil
}

// TopLevel returns the repository root for dir.
func TopLevel(dir string) (string, error) {
	return Run(dir, "rev-parse", "--show-toplevel")
}

// CurrentBranch returns the current branch name, or empty string if detached.
func CurrentBranch(dir string) string {
	out, err := Run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	if out == "HEAD" {
		return ""
	}
	return out
}

// HEADCommit returns the current HEAD commit hash.
func HEADCommit(dir string) (string, error) {
	return Run(dir, "rev-parse", "HEAD")
}

// IsDirty returns true if the working tree has unstaged or uncommitted changes.
func IsDirty(dir string) bool {
	out, err := Run(dir, "status", "--porcelain")
	if err != nil {
		return false
	}
	return out != ""
}

// StagedPatch returns the diff of the staging area.
func StagedPatch(dir string) (string, error) {
	return Run(dir, "diff", "--cached")
}

// UnstagedPatch returns the diff of unstaged changes.
func UnstagedPatch(dir string) (string, error) {
	return Run(dir, "diff")
}

// StashList returns the stash ref list (one per line).
func StashList(dir string) (string, error) {
	return Run(dir, "stash", "list")
}

// StashPatch returns the full diff of a stash entry (e.g. "stash@{0}").
func StashPatch(dir, ref string) (string, error) {
	return Run(dir, "stash", "show", "-p", ref)
}

// UntrackedFiles returns a list of untracked file paths, one per line.
func UntrackedFiles(dir string) (string, error) {
	return Run(dir, "ls-files", "--others", "--exclude-standard")
}

// Remotes returns "name\turl" lines for all remotes.
func Remotes(dir string) (string, error) {
	return Run(dir, "remote", "-v")
}

// Branches returns "refname\tupstream" lines for all branches.
func Branches(dir string) (string, error) {
	return Run(dir, "for-each-ref", "--format=%(refname:short)\t%(upstream:short)", "refs/heads/")
}

// Clone runs git clone with the given URL and target directory.
func Clone(url, target string) error {
	return RunSilent(target, "clone", url, target)
}

// Checkout switches to the given branch or commit.
func Checkout(dir, ref string) error {
	return RunSilent(dir, "checkout", ref)
}

// ApplyCached applies a patch to the staging area.
func ApplyCached(dir, patch string) error {
	cmd := exec.Command("git", "apply", "--cached")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(patch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git apply --cached: %w\n%s", err, stderr.String())
	}
	return nil
}

// Apply applies a patch to the working tree.
func Apply(dir, patch string) error {
	cmd := exec.Command("git", "apply")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(patch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git apply: %w\n%s", err, stderr.String())
	}
	return nil
}

// StashApply recreates a stash from a patch by applying it to the index,
// then stashing. This is approximate — the exact stash commit graph
// cannot be perfectly recreated.
func StashApply(dir, patch string) error {
	// apply to working tree, then stash
	if err := Apply(dir, patch); err != nil {
		return err
	}
	return RunSilent(dir, "stash")
}

// AddRemote adds a named remote.
func AddRemote(dir, name, url string) error {
	return RunSilent(dir, "remote", "add", name, url)
}

// TrackBranch sets up local branch to track a remote branch.
func TrackBranch(dir, branch, upstream string) error {
	return RunSilent(dir, "branch", "--set-upstream-to="+upstream, branch)
}

// ConfigGet returns a git config value.
func ConfigGet(dir, key string) (string, error) {
	return Run(dir, "config", "--get", key)
}

// ConfigSet sets a git config value in the local repo.
func ConfigSet(dir, key, value string) error {
	return RunSilent(dir, "config", "--local", key, value)
}

// FetchAll fetches all remotes.
func FetchAll(dir string) error {
	return RunSilent(dir, "fetch", "--all")
}

// AheadBehind returns (ahead, behind) counts for the current branch
// relative to its upstream. Returns (0, 0) if no upstream is set.
func AheadBehind(dir string) (int, int, error) {
	out, err := Run(dir, "rev-list", "--count", "--left-right", "@{upstream}...HEAD")
	if err != nil {
		return 0, 0, nil // no upstream
	}
	parts := strings.SplitN(out, "\t", 2)
	if len(parts) != 2 {
		return 0, 0, nil
	}
	var behind, ahead int
	fmt.Sscanf(parts[0], "%d", &behind)
	fmt.Sscanf(parts[1], "%d", &ahead)
	return ahead, behind, nil
}

// IsDetached returns true if HEAD is detached.
func IsDetached(dir string) bool {
	out, err := Run(dir, "symbolic-ref", "-q", "HEAD")
	return err != nil || out == ""
}

// StashCount returns the number of stash entries.
func StashCount(dir string) int {
	out, err := StashList(dir)
	if err != nil || out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}
