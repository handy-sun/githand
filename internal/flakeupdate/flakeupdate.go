// Package flakeupdate updates Nix flake inputs for repos that have a flake.nix.
package flakeupdate

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/handy-sun/githand/internal/config"
	"github.com/handy-sun/githand/internal/git"
	"golang.org/x/sync/errgroup"
)

// Result holds the outcome of updating a single repo's flake.
type Result struct {
	Name   string
	Path   string
	Status string // "updated", "up-to-date", "skipped", "error"
	Detail string // human-readable detail
	GitOut string // raw nix flake update output (stdout + stderr)
}

// Run updates flake inputs for all registered repos that contain a flake.nix.
// If group is non-empty, only repos in that group are processed.
// If repoName is non-empty, only that repo is processed.
// onResult is called (under a mutex) as each repo finishes; may be nil.
func Run(reg *config.Registry, group, repoName string, workers int, onResult func(Result)) ([]Result, error) {
	repos, err := selectRepos(reg, group, repoName)
	if err != nil {
		return nil, err
	}

	results := make([]Result, len(repos))
	var mu sync.Mutex
	eg := &errgroup.Group{}
	eg.SetLimit(workers)

	for i, repo := range repos {
		i, repo := i, repo
		eg.Go(func() error {
			r := updateOne(repo)
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
	return results, nil
}

func selectRepos(reg *config.Registry, group, repoName string) ([]config.Repo, error) {
	repos := reg.Repos
	if group != "" {
		repos = reg.ReposInGroup(group)
	}
	if repoName == "" {
		return repos, nil
	}
	for _, repo := range repos {
		if repo.Name == repoName {
			return []config.Repo{repo}, nil
		}
	}
	if group != "" {
		return nil, fmt.Errorf("repo %q not found in group %q", repoName, group)
	}
	return nil, fmt.Errorf("repo %q not found in registry", repoName)
}

func updateOne(repo config.Repo) Result {
	dir := repo.Path
	r := Result{Name: repo.Name, Path: dir}

	// Must be a git repo
	if !git.IsRepo(dir) {
		r.Status = "skipped"
		r.Detail = "not a git repository"
		return r
	}

	// Check for flake.nix
	flakePath := filepath.Join(dir, "flake.nix")
	if _, err := os.Stat(flakePath); os.IsNotExist(err) {
		r.Status = "skipped"
		r.Detail = "no flake.nix"
		return r
	}

	if git.IsDetached(dir) {
		r.Status = "skipped"
		r.Detail = "detached HEAD"
		return r
	}

	// Record HEAD before update
	oldHash := git.ShortHash(dir)

	// Run nix flake update --commit-lock-file
	stdout, stderr, err := runNixFlakeUpdate(dir)

	// Combine output for display
	out := strings.TrimSpace(stdout + "\n" + stderr)
	r.GitOut = out

	if err != nil {
		r.Status = "error"
		if stderr != "" {
			r.Detail = strings.TrimSpace(stderr)
		} else {
			r.Detail = err.Error()
		}
		return r
	}

	// Check if anything changed (commit was created)
	newHash := git.ShortHash(dir)
	if oldHash == newHash {
		r.Status = "up-to-date"
		r.Detail = "flake inputs already up to date"
	} else {
		r.Status = "updated"
		r.Detail = fmt.Sprintf("%s..%s", oldHash, newHash)
	}
	return r
}

// runNixFlakeUpdate runs "nix flake update --commit-lock-file" in dir.
// Returns stdout, stderr, and any error.
func runNixFlakeUpdate(dir string) (string, string, error) {
	cmd := exec.Command("nix", "flake", "update", "--commit-lock-file")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("nix flake update: %w", err)
	}
	return strings.TrimRight(stdout.String(), "\n"), strings.TrimRight(stderr.String(), "\n"), nil
}
