// Package sync pulls latest changes from remotes for all registered repos.
package sync

import (
	"strings"
	"sync"

	"github.com/handy-sun/githand/internal/config"
	"github.com/handy-sun/githand/internal/git"
	"golang.org/x/sync/errgroup"
)

// Result holds the outcome of syncing a single repo.
type Result struct {
	Name    string
	Path    string
	Status  string // "updated", "up-to-date", "fetched", "skipped", "error"
	Detail  string // human-readable detail (branch, remote, error message)
	OldHash string // HEAD short hash before pull (empty if unchanged)
	NewHash string // HEAD short hash after pull (empty if unchanged)
	GitOut  string // raw git pull output (stdout + stderr)
}

// Run syncs all registered repos from their remotes concurrently.
// If group is non-empty, only repos in that group are synced.
// remote defaults to "origin" when empty.
// onResult is called (under a mutex) as each repo finishes; may be nil.
func Run(reg *config.Registry, group, remote string, workers int, onResult func(Result)) []Result {
	repos := reg.Repos
	if group != "" {
		repos = reg.ReposInGroup(group)
	}
	if remote == "" {
		remote = "origin"
	}

	results := make([]Result, len(repos))
	var mu sync.Mutex
	eg := &errgroup.Group{}
	eg.SetLimit(workers)

	for i, repo := range repos {
		i, repo := i, repo
		eg.Go(func() error {
			r := syncOne(repo, remote)
			results[i] = r
			if onResult != nil {
				mu.Lock()
				onResult(r)
				mu.Unlock()
			}
			return nil
		})
	}
	_ = eg.Wait()
	return results
}

func syncOne(repo config.Repo, remote string) Result {
	dir := repo.Path
	r := Result{Name: repo.Name, Path: dir}

	if !git.IsRepo(dir) {
		r.Status = "skipped"
		r.Detail = "not a git repository"
		return r
	}

	if git.IsDetached(dir) {
		if err := git.FetchRemote(dir, remote); err != nil {
			r.Status = "error"
			r.Detail = err.Error()
		} else {
			r.Status = "fetched"
			r.Detail = "detached HEAD, fetched only"
		}
		return r
	}

	branch := git.CurrentBranch(dir)
	if branch == "" {
		r.Status = "skipped"
		r.Detail = "detached HEAD"
		return r
	}

	// Record HEAD before pull
	oldHash := git.ShortHash(dir)

	// Check pull.rebase config
	useRebase := pullRebase(dir)

	// --autostash handles dirty worktrees automatically
	pullArgs := []string{"--autostash", remote, branch}
	if useRebase {
		pullArgs = append([]string{"--rebase"}, pullArgs...)
	} else {
		pullArgs = append([]string{"--no-rebase"}, pullArgs...)
	}

	stdout, stderr, err := git.Pull(dir, pullArgs...)

	// Combine output for display
	out := strings.TrimSpace(stdout + "\n" + stderr)
	r.GitOut = out

	if err != nil {
		// Old git: "Already up to date" comes with exit code 1
		if containsUpToDate(stderr) || containsUpToDate(stdout) {
			r.Status = "up-to-date"
			r.Detail = branch
			return r
		}
		r.Status = "error"
		if stderr != "" {
			r.Detail = strings.TrimSpace(stderr)
		} else {
			r.Detail = err.Error()
		}
		return r
	}

	// Git 2.54+: exit 0 for both up-to-date and updated.
	if containsUpToDate(stdout) || isOkOnly(stdout) {
		r.Status = "up-to-date"
	} else {
		r.Status = "updated"
		newHash := git.ShortHash(dir)
		r.OldHash = oldHash
		r.NewHash = newHash
	}
	r.Detail = branch
	return r
}

// containsUpToDate checks for the "Already up to date" message (English or
// locale variants). Git uses this message across versions.
func containsUpToDate(s string) bool {
	return strings.Contains(s, "Already up to date") ||
		strings.Contains(s, "(up-to-date)")
}

// isOkOnly matches git 2.54+ up-to-date output that is just "ok" with nothing
// else on the line (no stats, no hash).
func isOkOnly(stdout string) bool {
	return strings.TrimSpace(stdout) == "ok"
}

// pullRebase checks the pull.rebase git config for the repo.
func pullRebase(dir string) bool {
	val, err := git.ConfigGet(dir, "pull.rebase")
	if err != nil {
		return false
	}
	v := strings.TrimSpace(strings.ToLower(val))
	return v == "true" || v == "yes" || v == "1"
}
