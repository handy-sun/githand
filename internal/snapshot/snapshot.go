// Package snapshot handles creating and writing workspace snapshots.
package snapshot

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/handy-sun/githand/internal/config"
	"github.com/handy-sun/githand/internal/git"
)

// Snapshot is the top-level JSON manifest.
type Snapshot struct {
	Schema    int        `json:"schema"`
	CreatedAt string     `json:"created_at"`
	Host      string     `json:"host"`
	BasePath  string     `json:"base_path"`
	Repos     []RepoSnap `json:"repos"`
	Groups    map[string][]string `json:"groups,omitempty"`
}

// RepoSnap holds the captured state of a single repo.
type RepoSnap struct {
	Name         string      `json:"name"`
	RelPath      string      `json:"rel_path"`
	Group        string      `json:"group,omitempty"`
	Remotes      []RemoteSnap `json:"remotes"`
	Branches     []BranchSnap `json:"branches"`
	CurrentBranch string     `json:"current_branch,omitempty"`
	HeadCommit   string      `json:"head_commit"`
	Detached     bool        `json:"detached"`
	Dirty        bool        `json:"dirty"`
	StagedPatch  string      `json:"staged_patch,omitempty"`
	UnstagedPatch string     `json:"unstaged_patch,omitempty"`
	Stashes      []StashSnap `json:"stashes,omitempty"`
	Untracked    []string    `json:"untracked,omitempty"`
	UntrackedArchive string  `json:"untracked_archive,omitempty"`
}

type RemoteSnap struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type BranchSnap struct {
	Name     string `json:"name"`
	Upstream string `json:"upstream,omitempty"`
}

type StashSnap struct {
	Ref   string `json:"ref"`
	Patch string `json:"patch"`
}

// Take creates a snapshot for the given repos.
func Take(reg *config.Registry, repos []config.Repo, includeClean bool) (*Snapshot, error) {
	hostname, _ := os.Hostname()
	snap := &Snapshot{
		Schema:    1,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Host:      hostname,
		BasePath:  reg.BasePath,
		Groups:    reg.Groups,
	}

	for _, repo := range repos {
		rs, err := snapshotRepo(reg.BasePath, repo)
		if err != nil {
			return nil, fmt.Errorf("snapshot %s: %w", repo.Name, err)
		}
		if !includeClean && !rs.Dirty && len(rs.Stashes) == 0 && len(rs.Untracked) == 0 {
			continue
		}
		snap.Repos = append(snap.Repos, rs)
	}

	return snap, nil
}

func snapshotRepo(basePath string, repo config.Repo) (RepoSnap, error) {
	dir := repo.Path
	rs := RepoSnap{
		Name:     repo.Name,
		RelPath:  relPath(basePath, dir),
		Group:    repo.Group,
	}

	// head
	rs.CurrentBranch = git.CurrentBranch(dir)
	rs.Detached = git.IsDetached(dir)
	commit, _ := git.HEADCommit(dir)
	rs.HeadCommit = commit
	rs.Dirty = git.IsDirty(dir)

	// remotes
	remotesStr, _ := git.Remotes(dir)
	rs.Remotes = parseRemotes(remotesStr)

	// branches
	branchesStr, _ := git.Branches(dir)
	rs.Branches = parseBranches(branchesStr)

	// patches
	if rs.Dirty {
		if patch, err := git.StagedPatch(dir); err == nil {
			rs.StagedPatch = patch
		}
		if patch, err := git.UnstagedPatch(dir); err == nil {
			rs.UnstagedPatch = patch
		}
	}

	// stashes
	stashList, _ := git.StashList(dir)
	if stashList != "" {
		rs.Stashes = parseStashes(dir, stashList)
	}

	// untracked
	untracked, _ := git.UntrackedFiles(dir)
	if untracked != "" {
		rs.Untracked = strings.Split(untracked, "\n")
	}

	return rs, nil
}

func parseRemotes(raw string) []RemoteSnap {
	var result []RemoteSnap
	seen := make(map[string]bool)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		name := parts[0]
		url := strings.SplitN(parts[1], " ", 2)[0]
		if !seen[name] {
			seen[name] = true
			result = append(result, RemoteSnap{Name: name, URL: url})
		}
	}
	return result
}

func parseBranches(raw string) []BranchSnap {
	var result []BranchSnap
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		name := parts[0]
		upstream := ""
		if len(parts) == 2 {
			upstream = parts[1]
		}
		result = append(result, BranchSnap{Name: name, Upstream: upstream})
	}
	return result
}

func parseStashes(dir, raw string) []StashSnap {
	var result []StashSnap
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// format: "stash@{0}: On branch: message"
		ref := strings.SplitN(line, ":", 2)[0]
		ref = strings.TrimSpace(ref)
		patch, _ := git.StashPatch(dir, ref)
		result = append(result, StashSnap{Ref: ref, Patch: patch})
	}
	return result
}

// relPath computes a repo's path relative to base_path.
func relPath(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

// DataDirPath returns the sibling data directory for a snapshot JSON file.
func DataDirPath(jsonPath string) string {
	ext := filepath.Ext(jsonPath)
	base := jsonPath[:len(jsonPath)-len(ext)]
	return base + "-data"
}

// Write saves the snapshot JSON and archives untracked files.
func Write(snap *Snapshot, jsonPath, dataDir string) error {
	// write JSON
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}

	// archive untracked files per repo
	for _, rs := range snap.Repos {
		if len(rs.Untracked) == 0 {
			continue
		}
		archiveName := rs.Name + ".tar.gz"
		rs.UntrackedArchive = archiveName

		untrackedDir := filepath.Join(dataDir, "untracked")
		if err := os.MkdirAll(untrackedDir, 0o755); err != nil {
			return fmt.Errorf("create untracked dir: %w", err)
		}

		archivePath := filepath.Join(untrackedDir, archiveName)
		if err := tarGzUntracked(rs, archivePath); err != nil {
			return fmt.Errorf("archive untracked for %s: %w", rs.Name, err)
		}
	}

	return nil
}

func tarGzUntracked(rs RepoSnap, archivePath string) error {
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// we need the repo's absolute path to read files
	// this is reconstructed at restore time from base_path + rel_path
	// during snapshot, we look up the repo by name in the registry
	// For now, we skip the actual archiving here — it requires the repo's
	// absolute path which we need to pass through. This will be wired up
	// when integrating with the scan command.
	_ = tw
	_ = io.Copy

	return nil
}

// Filter removes repos from a snapshot that don't match the given filter.
func Filter(repos []RepoSnap, filter string) []RepoSnap {
	var result []RepoSnap
	for _, rs := range repos {
		switch filter {
		case "dirty":
			if rs.Dirty {
				result = append(result, rs)
			}
		case "ahead":
			// ahead info is not stored in snapshot directly;
			// this filter makes more sense for live status
		case "stash":
			if len(rs.Stashes) > 0 {
				result = append(result, rs)
			}
		case "detached":
			if rs.Detached {
				result = append(result, rs)
			}
		}
	}
	return result
}
