// Package snapshot handles creating and writing workspace snapshots.
package snapshot

import (
	"archive/tar"
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
	Schema    int                 `json:"schema"`
	CreatedAt string              `json:"created_at"`
	Host      string              `json:"host"`
	BasePath  string              `json:"base_path"`
	Repos     []RepoSnap          `json:"repos"`
	Groups    map[string][]string `json:"groups,omitempty"`
}

// RepoSnap holds the captured state of a single repo.
type RepoSnap struct {
	Name          string       `json:"name"`
	RelPath       string       `json:"rel_path"`
	Group         string       `json:"group,omitempty"`
	Remotes       []RemoteSnap `json:"remotes"`
	Branches      []BranchSnap `json:"branches"`
	CurrentBranch string       `json:"current_branch,omitempty"`
	HeadCommit    string       `json:"head_commit"`
	Detached      bool         `json:"detached"`
	Dirty         bool         `json:"dirty"`
	HooksPath     string       `json:"hooks_path,omitempty"`
	StagedPatch   string       `json:"staged_patch,omitempty"`
	UnstagedPatch string       `json:"unstaged_patch,omitempty"`
	Stashes       []StashSnap  `json:"stashes,omitempty"`
	Untracked     []string     `json:"untracked,omitempty"`
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

// SnapshotJSONName is the fixed filename for the snapshot manifest inside a snapshot folder.
const SnapshotJSONName = "snapshot.json"

// DefaultSnapshotDir generates a snapshot folder path under parentDir.
// Folder name format: githand-snapshot.MMDD-HHmmss
func DefaultSnapshotDir(parentDir string) string {
	ts := time.Now().Format("0102-150405")
	name := fmt.Sprintf("githand-snapshot.%s", ts)
	return filepath.Join(parentDir, name)
}

// ResolveSnapshotPath accepts either a directory (containing snapshot.json)
// or a direct .json file path. Returns the absolute path to the JSON file.
func ResolveSnapshotPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		jsonPath := filepath.Join(abs, SnapshotJSONName)
		if _, err := os.Stat(jsonPath); err != nil {
			return "", fmt.Errorf("snapshot.json not found in %s: %w", abs, err)
		}
		return jsonPath, nil
	}
	// it's a file — use directly (backward compat with old-style standalone json)
	return abs, nil
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
		Name:    repo.Name,
		RelPath: relPath(basePath, dir),
		Group:   repo.Group,
	}

	// head
	rs.CurrentBranch = git.CurrentBranch(dir)
	rs.Detached = git.IsDetached(dir)
	commit, _ := git.HEADCommit(dir)
	rs.HeadCommit = commit
	rs.Dirty = git.IsDirty(dir)

	// core.hooksPath (effective value — may come from local/global/system config)
	if hp, err := git.ConfigGet(dir, "core.hooksPath"); err == nil {
		rs.HooksPath = hp
	}

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

// Write saves the snapshot into the given directory.
// It creates snapshot.json and copies untracked files into untracked/<repo>/.
func Write(snap *Snapshot, dir string, basePath string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}

	// write JSON
	jsonPath := filepath.Join(dir, SnapshotJSONName)
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}

	// copy untracked files per repo
	for _, rs := range snap.Repos {
		if len(rs.Untracked) == 0 {
			continue
		}
		repoAbsPath := filepath.Join(basePath, filepath.FromSlash(rs.RelPath))
		untrackedDir := filepath.Join(dir, "untracked", rs.Name)
		if err := os.MkdirAll(untrackedDir, 0o755); err != nil {
			return fmt.Errorf("create untracked dir: %w", err)
		}
		for _, file := range rs.Untracked {
			src := filepath.Join(repoAbsPath, file)
			dst := filepath.Join(untrackedDir, file)
			if err := copyFile(src, dst); err != nil {
				// log warning but don't fail the whole snapshot
				fmt.Fprintf(os.Stderr, "  warning: copy untracked %s: %v\n", file, err)
			}
		}
	}

	return nil
}

// WriteOutput saves a snapshot using the compact output layout.
// JSON-only snapshots are written directly to outputBase+".json".
// Snapshots with untracked files keep the directory layout so file payloads
// can live next to snapshot.json. If archiveDir is true, that directory is
// also written as outputBase+".tar".
func WriteOutput(snap *Snapshot, outputBase string, basePath string, archiveDir bool) (string, error) {
	if HasUntracked(snap) {
		if err := Write(snap, outputBase, basePath); err != nil {
			return "", err
		}
		if archiveDir {
			if err := writeTar(outputBase, outputBase+".tar"); err != nil {
				return "", fmt.Errorf("archive snapshot: %w", err)
			}
		}
		return outputBase, nil
	}

	jsonPath := outputBase
	if filepath.Ext(jsonPath) != ".json" {
		jsonPath += ".json"
	}
	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
		return "", fmt.Errorf("create snapshot parent dir: %w", err)
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal snapshot: %w", err)
	}
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write snapshot: %w", err)
	}
	return jsonPath, nil
}

// HasUntracked reports whether the snapshot needs payload files next to JSON.
func HasUntracked(snap *Snapshot) bool {
	for _, rs := range snap.Repos {
		if len(rs.Untracked) > 0 {
			return true
		}
	}
	return false
}

func writeTar(srcDir, tarPath string) error {
	out, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer out.Close()

	tw := tar.NewWriter(out)
	defer tw.Close()

	parent := filepath.Dir(srcDir)
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(parent, path)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
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
	// preserve permissions
	info, err := in.Stat()
	if err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
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
