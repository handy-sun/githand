// Package restore handles restoring repos from a snapshot.
package restore

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/handy-sun/githand/internal/git"
	"github.com/handy-sun/githand/internal/i18n"
	"github.com/handy-sun/githand/internal/snapshot"
)

// Run restores repos from a snapshot into targetDir.
// snapPath can be either a directory (containing snapshot.json) or a direct .json file.
func Run(snapPath, targetDir, basePath string, dryRun bool) error {
	// resolve snapshot JSON path (dir or file)
	jsonPath, err := snapshot.ResolveSnapshotPath(snapPath)
	if err != nil {
		return fmt.Errorf("resolve snapshot: %w", err)
	}

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}

	var snap snapshot.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("parse snapshot: %w", err)
	}

	// determine base path for path remapping
	effectiveBase := snap.BasePath
	if basePath != "" {
		effectiveBase = basePath
	}

	// snapshot directory (parent of snapshot.json)
	snapDir := filepath.Dir(jsonPath)

	fmt.Println(i18n.Tf("restore.progress", len(snap.Repos), snapPath, targetDir))

	for _, rs := range snap.Repos {
		repoDir := filepath.Join(targetDir, rs.RelPath)

		if dryRun {
			fmt.Println(i18n.Tf("restore.dry_run", rs.Name, repoDir))
			continue
		}

		// check if repo already exists
		exists := false
		if _, err := os.Stat(repoDir); err == nil && git.IsRepo(repoDir) {
			exists = true
		}

		if err := restoreRepo(rs, repoDir, snapDir); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: restore %s failed: %v\n", rs.Name, err)
			continue
		}

		if exists {
			fmt.Println(i18n.Tf("restore.updated", rs.Name))
		} else {
			fmt.Println(i18n.Tf("restore.restored", rs.Name))
		}

		_ = effectiveBase // used for path remapping
	}

	return nil
}

func restoreRepo(rs snapshot.RepoSnap, targetDir, snapDir string) error {
	// ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	// check if target directory already exists
	if _, err := os.Stat(targetDir); err == nil {
		// directory exists - check if it's a git repo
		if !git.IsRepo(targetDir) {
			return fmt.Errorf("target exists but is not a git repo: %s", targetDir)
		}
		// it's a git repo - update it instead of cloning
		return updateRepo(rs, targetDir, snapDir)
	}

	// directory doesn't exist - clone from primary remote
	if len(rs.Remotes) == 0 {
		return fmt.Errorf("no remotes for %s", rs.Name)
	}

	primary := rs.Remotes[0]
	if err := git.Clone(primary.URL, targetDir); err != nil {
		return fmt.Errorf("clone: %w", err)
	}

	if err := ensureSnapshotRemotes(rs, targetDir); err != nil {
		return err
	}
	if len(rs.Remotes) > 1 || rs.Bundle {
		if err := git.FetchAll(targetDir); err != nil {
			return fmt.Errorf("fetch remotes: %w", err)
		}
	}
	if err := importSnapshotBundle(rs, targetDir, snapDir); err != nil {
		return err
	}

	// checkout branch or detached commit
	if err := checkoutSnapshotRef(rs, targetDir); err != nil {
		return err
	}

	restoreWorkingState(rs, targetDir, snapDir)
	return nil
}

func updateRepo(rs snapshot.RepoSnap, targetDir, snapDir string) error {
	if git.IsDirty(targetDir) {
		return fmt.Errorf("target repo has local changes; refusing to update existing repo: %s", targetDir)
	}

	if err := ensureSnapshotRemotes(rs, targetDir); err != nil {
		return err
	}

	if len(rs.Remotes) > 0 {
		if err := git.FetchAll(targetDir); err != nil {
			return fmt.Errorf("fetch: %w", err)
		}
	}
	if err := importSnapshotBundle(rs, targetDir, snapDir); err != nil {
		return err
	}

	if err := checkoutSnapshotRef(rs, targetDir); err != nil {
		return err
	}

	restoreWorkingState(rs, targetDir, snapDir)
	return nil
}

func importSnapshotBundle(rs snapshot.RepoSnap, targetDir, snapDir string) error {
	if !rs.Bundle {
		return nil
	}
	bundlePath := snapshot.BundlePath(snapDir, rs.Name)
	if err := git.ImportBundle(targetDir, bundlePath); err != nil {
		return fmt.Errorf("import bundle %s: %w", rs.Name, err)
	}
	return nil
}

func ensureSnapshotRemotes(rs snapshot.RepoSnap, targetDir string) error {
	for _, remote := range rs.Remotes {
		if remote.Name == "" {
			continue
		}

		currentURL, err := git.ConfigGet(targetDir, "remote."+remote.Name+".url")
		if err != nil {
			if err := git.AddRemote(targetDir, remote.Name, remote.URL); err != nil {
				return fmt.Errorf("add remote %s: %w", remote.Name, err)
			}
			continue
		}

		if currentURL != remote.URL {
			if err := git.SetRemoteURL(targetDir, remote.Name, remote.URL); err != nil {
				return fmt.Errorf("set remote %s url: %w", remote.Name, err)
			}
		}
	}
	return nil
}

func checkoutSnapshotRef(rs snapshot.RepoSnap, targetDir string) error {
	if rs.Detached {
		if rs.HeadCommit == "" {
			return fmt.Errorf("detached repo %s has no head commit", rs.Name)
		}
		if err := git.Checkout(targetDir, rs.HeadCommit); err != nil {
			return fmt.Errorf("checkout %s: %w", rs.HeadCommit, err)
		}
		return nil
	}

	if rs.CurrentBranch != "" {
		if rs.HeadCommit == "" {
			if err := git.Checkout(targetDir, rs.CurrentBranch); err != nil {
				return fmt.Errorf("checkout branch %s: %w", rs.CurrentBranch, err)
			}
			return nil
		}
		if err := git.CheckoutBranchAt(targetDir, rs.CurrentBranch, rs.HeadCommit); err != nil {
			return fmt.Errorf("checkout branch %s at %s: %w", rs.CurrentBranch, rs.HeadCommit, err)
		}
		if upstream := branchUpstream(rs, rs.CurrentBranch); upstream != "" {
			_ = git.TrackBranch(targetDir, rs.CurrentBranch, upstream)
		}
		return nil
	}

	if rs.HeadCommit != "" {
		if err := git.Checkout(targetDir, rs.HeadCommit); err != nil {
			return fmt.Errorf("checkout %s: %w", rs.HeadCommit, err)
		}
	}
	return nil
}

func branchUpstream(rs snapshot.RepoSnap, branch string) string {
	for _, b := range rs.Branches {
		if b.Name == branch {
			return b.Upstream
		}
	}
	return ""
}

func restoreWorkingState(rs snapshot.RepoSnap, targetDir, snapDir string) {
	// restore core.hooksPath config (pinned to local repo so it travels with the restore)
	if rs.HooksPath != "" {
		if err := git.ConfigSet(targetDir, "core.hooksPath", rs.HooksPath); err != nil {
			fmt.Fprintf(os.Stderr, "    warning: set core.hooksPath failed: %v\n", err)
		}
	}

	// apply staged patch
	if rs.StagedPatch != "" {
		if err := git.ApplyCached(targetDir, rs.StagedPatch); err != nil {
			fmt.Fprintf(os.Stderr, "    warning: staged patch apply failed: %v\n", err)
		}
	}

	// apply unstaged patch
	if rs.UnstagedPatch != "" {
		if err := git.Apply(targetDir, rs.UnstagedPatch); err != nil {
			fmt.Fprintf(os.Stderr, "    warning: unstaged patch apply failed: %v\n", err)
		}
	}

	// recreate stashes
	for _, stash := range rs.Stashes {
		if err := git.StashApply(targetDir, stash.Patch); err != nil {
			fmt.Fprintf(os.Stderr, "    warning: stash restore failed for %s: %v\n", stash.Ref, err)
		}
	}

	// copy untracked files from snapshot's untracked/<repo>/ directory
	if len(rs.Untracked) > 0 {
		untrackedDir := filepath.Join(snapDir, "untracked", rs.Name)
		for _, file := range rs.Untracked {
			src := filepath.Join(untrackedDir, file)
			dst := filepath.Join(targetDir, file)
			if err := copyFile(src, dst); err != nil {
				fmt.Fprintf(os.Stderr, "    warning: restore untracked %s: %v\n", file, err)
			}
		}
	}
}

// copyFile copies a single file, creating parent directories as needed.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	info, err := in.Stat()
	if err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}
