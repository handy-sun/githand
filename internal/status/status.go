// Package status collects and filters git repository status information.
package status

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/handy-sun/githand/internal/config"
	"github.com/handy-sun/githand/internal/discover"
	"github.com/handy-sun/githand/internal/git"
	"golang.org/x/sync/errgroup"
)

// RepoStatus holds the collected status of a single repository.
type RepoStatus struct {
	Repo       config.Repo
	Branch     string
	Commit     string
	Dirty      bool
	Ahead      int
	Behind     int
	StashCount int
	Detached   bool
	Remotes    []RemoteInfo
}

// RemoteInfo holds a remote name and URL.
type RemoteInfo struct {
	Name string
	URL  string
}

// PrimarySource returns the host of the origin remote, or the first remote
// when origin is not configured.
func PrimarySource(remotes []RemoteInfo) string {
	primary, ok := primaryRemote(remotes)
	if !ok {
		return "-"
	}

	if parsed, err := url.Parse(primary.URL); err == nil {
		if host := parsed.Hostname(); host != "" {
			return host
		}
	}
	if strings.Contains(primary.URL, "://") {
		return "-"
	}

	if len(primary.URL) >= 3 && primary.URL[1] == ':' &&
		(primary.URL[2] == '/' || primary.URL[2] == '\\') {
		return "-"
	}

	// Git also accepts SCP-like URLs such as git@github.com:owner/repo.git.
	colon := strings.IndexByte(primary.URL, ':')
	slash := strings.IndexAny(primary.URL, `/\`)
	if colon > 0 && (slash == -1 || colon < slash) {
		host := primary.URL[:colon]
		if at := strings.LastIndexByte(host, '@'); at >= 0 {
			host = host[at+1:]
		}
		if host != "" {
			return host
		}
	}

	return "-"
}

// PrimaryProtocol returns the transport protocol of the primary remote:
// https, http, ssh, git, or file. Returns "-" when unknown or absent.
func PrimaryProtocol(remotes []RemoteInfo) string {
	primary, ok := primaryRemote(remotes)
	if !ok {
		return "-"
	}

	u := primary.URL
	// Windows drive-letter paths parse as a single-letter URL scheme; check
	// them before treating any scheme as an explicit transport.
	if len(u) >= 3 && u[1] == ':' && (u[2] == '/' || u[2] == '\\') {
		return "file"
	}
	if parsed, err := url.Parse(u); err == nil && parsed.Scheme != "" {
		switch strings.ToLower(parsed.Scheme) {
		case "http", "https", "ssh", "git", "file":
			return strings.ToLower(parsed.Scheme)
		default:
			return "-" // explicit transport we do not model
		}
	}
	if strings.Contains(u, "://") {
		return "-" // url.Parse failed on an explicit URL
	}

	// SCP-like URLs such as git@github.com:owner/repo.git use SSH.
	colon := strings.IndexByte(u, ':')
	slash := strings.IndexAny(u, `/\`)
	if colon > 0 && (slash == -1 || colon < slash) {
		return "ssh"
	}

	if strings.HasPrefix(u, "/") || strings.HasPrefix(u, "./") ||
		strings.HasPrefix(u, "../") || strings.HasPrefix(u, "~") {
		return "file"
	}
	return "-"
}

// primaryRemote picks the origin remote, falling back to the first one.
func primaryRemote(remotes []RemoteInfo) (RemoteInfo, bool) {
	if len(remotes) == 0 {
		return RemoteInfo{}, false
	}
	primary := remotes[0]
	for _, remote := range remotes {
		if remote.Name == "origin" {
			primary = remote
			break
		}
	}
	return primary, true
}

// Collect gathers status for all repos in the registry concurrently.
func Collect(reg *config.Registry, workers int) ([]RepoStatus, error) {
	results := make([]RepoStatus, len(reg.Repos))
	eg := &errgroup.Group{}
	eg.SetLimit(workers)

	for i, repo := range reg.Repos {
		i, repo := i, repo
		eg.Go(func() error {
			s, err := collectOne(repo)
			if err != nil {
				// best-effort: report error in status, don't abort all
				s = RepoStatus{Repo: repo}
				_ = err
			}
			results[i] = s
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

func collectOne(repo config.Repo) (RepoStatus, error) {
	dir := repo.Path
	s := RepoStatus{Repo: repo}

	s.Branch = git.CurrentBranch(dir)
	s.Detached = git.IsDetached(dir)

	commit, err := git.HEADCommit(dir)
	if err == nil {
		s.Commit = commit
	}

	s.Dirty = git.IsDirty(dir)

	ahead, behind, _ := git.AheadBehind(dir)
	s.Ahead = ahead
	s.Behind = behind

	s.StashCount = git.StashCount(dir)

	// parse remotes
	remotesStr, err := git.Remotes(dir)
	if err == nil {
		s.Remotes = parseRemotes(remotesStr)
	}

	return s, nil
}

func parseRemotes(raw string) []RemoteInfo {
	var result []RemoteInfo
	seen := make(map[string]bool)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// format: "name\turl (fetch)" or "name\turl (push)"
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		name := parts[0]
		urlPart := strings.SplitN(parts[1], " ", 2)[0]
		if !seen[name] {
			seen[name] = true
			result = append(result, RemoteInfo{Name: name, URL: urlPart})
		}
	}
	return result
}

// FilterByFlag filters statuses by a named filter.
// Supported: dirty, ahead, stash, detached.
func FilterByFlag(statuses []RepoStatus, filter string) []RepoStatus {
	var result []RepoStatus
	for _, s := range statuses {
		switch filter {
		case "dirty":
			if s.Dirty {
				result = append(result, s)
			}
		case "ahead":
			if s.Ahead > 0 {
				result = append(result, s)
			}
		case "stash":
			if s.StashCount > 0 {
				result = append(result, s)
			}
		case "detached":
			if s.Detached {
				result = append(result, s)
			}
		}
	}
	return result
}

// FilterByGroup keeps only repos in the given group.
func FilterByGroup(statuses []RepoStatus, reg *config.Registry, group string) []RepoStatus {
	inGroup := make(map[string]bool)
	for _, r := range reg.ReposInGroup(group) {
		inGroup[r.Name] = true
	}
	var result []RepoStatus
	for _, s := range statuses {
		if inGroup[s.Repo.Name] {
			result = append(result, s)
		}
	}
	return result
}

// FilterByUser keeps only repos whose remote URL contains the given username.
func FilterByUser(statuses []RepoStatus, user string) []RepoStatus {
	var result []RepoStatus
	for _, s := range statuses {
		for _, r := range s.Remotes {
			if strings.Contains(r.URL, user) {
				result = append(result, s)
				break
			}
		}
	}
	return result
}

// SyncResult holds the result of syncing the registry with the filesystem.
type SyncResult struct {
	Added   int
	Removed int
}

// SyncRegistry synchronizes the registry with the filesystem:
// - Removes repos whose paths no longer exist
// - Discovers and adds new repos under every registered base path
// Returns the number of repos added and removed.
func SyncRegistry(reg *config.Registry, recursive, autoGroup bool) (SyncResult, error) {
	result := SyncResult{}

	// Step 1: Remove repos that no longer exist
	var validRepos []config.Repo
	for _, repo := range reg.Repos {
		if _, err := os.Stat(repo.Path); err == nil {
			// Path exists, keep it
			validRepos = append(validRepos, repo)
		} else if os.IsNotExist(err) {
			// Path doesn't exist, mark for removal
			result.Removed++
		}
	}
	reg.Repos = validRepos

	// Step 2: Discover new repos under every registered workspace root
	roots := reg.BasePaths
	if len(roots) == 0 && reg.BasePath != "" {
		// registry built directly rather than through LoadRegistry
		roots = []string{reg.BasePath}
	}
	if len(roots) == 0 {
		// No base path configured, skip discovery
		return result, nil
	}

	// Build a map of existing repos by normalized path
	existing := make(map[string]bool)
	for _, r := range reg.Repos {
		// Normalize path to handle symlinks (e.g., /var -> /private/var on macOS)
		normalized, err := filepath.EvalSymlinks(r.Path)
		if err != nil {
			// If we can't resolve symlinks, use the original path
			normalized = r.Path
		}
		existing[normalized] = true
	}

	for _, root := range roots {
		found, err := discover.Discover(root, recursive, autoGroup)
		if err != nil {
			return result, err
		}

		// Add new repos
		for _, r := range found {
			// Normalize the discovered path as well
			normalized, err := filepath.EvalSymlinks(r.Path)
			if err != nil {
				normalized = r.Path
			}

			if !existing[normalized] {
				reg.Repos = append(reg.Repos, r)
				existing[normalized] = true
				result.Added++

				// Register in groups map
				if r.Group != "" {
					if reg.Groups == nil {
						reg.Groups = make(map[string][]string)
					}
					reg.Groups[r.Group] = append(reg.Groups[r.Group], r.Name)
				}
			}
		}
	}

	return result, nil
}
