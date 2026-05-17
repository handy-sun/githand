// Package restore handles restoring repos from a snapshot.
package restore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/handy-sun/githand/internal/git"
	"github.com/handy-sun/githand/internal/i18n"
	"github.com/handy-sun/githand/internal/snapshot"
)

// Run restores repos from a snapshot JSON file into targetDir.
func Run(snapPath, targetDir, basePath string, dryRun bool) error {
	data, err := os.ReadFile(snapPath)
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

	fmt.Println(i18n.Tf("restore.progress", len(snap.Repos), snapPath, targetDir))

	for _, rs := range snap.Repos {
		repoDir := filepath.Join(targetDir, rs.RelPath)

		if dryRun {
fmt.Println(i18n.Tf("restore.dry_run", rs.Name, repoDir))
			continue
		}

		if err := restoreRepo(rs, repoDir); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: restore %s failed: %v\n", rs.Name, err)
			continue
		}
		fmt.Println(i18n.Tf("restore.restored", rs.Name))

		_ = effectiveBase // used for path remapping
	}

	return nil
}

func restoreRepo(rs snapshot.RepoSnap, targetDir string) error {
	// ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	// clone from primary remote
	if len(rs.Remotes) == 0 {
		return fmt.Errorf("no remotes for %s", rs.Name)
	}

	primary := rs.Remotes[0]
	if err := git.Clone(primary.URL, targetDir); err != nil {
		return fmt.Errorf("clone: %w", err)
	}

	// add additional remotes
	for _, r := range rs.Remotes[1:] {
		_ = git.AddRemote(targetDir, r.Name, r.URL)
	}

	// checkout branch or detached commit
	if rs.Detached {
		_ = git.Checkout(targetDir, rs.HeadCommit)
	} else if rs.CurrentBranch != "" {
		_ = git.Checkout(targetDir, rs.CurrentBranch)
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

	// untracked archives extraction is handled by the data directory
	// (to be implemented with tar.gz extraction)

	return nil
}
