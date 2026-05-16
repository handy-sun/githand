// Package discover walks a directory tree to find git repositories.
package discover

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/handy-sun/githand/internal/config"
	"github.com/handy-sun/githand/internal/git"
)

// Discover walks dir and returns all git repositories found.
// If recursive is true, it descends into subdirectories.
// If autoGroup is true, the immediate subdirectory name becomes the group.
func Discover(dir string, recursive, autoGroup bool) ([]config.Repo, error) {
	var repos []config.Repo
	seen := make(map[string]bool)

	err := walkDir(dir, dir, recursive, autoGroup, seen, &repos)
	if err != nil {
		return nil, err
	}
	return repos, nil
}

func walkDir(root, dir string, recursive, autoGroup bool, seen map[string]bool, repos *[]config.Repo) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // skip unreadable dirs
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())

		// skip hidden dirs (except when dir itself starts with .)
		if strings.HasPrefix(entry.Name(), ".") {
			// but check if it's a .git inside a non-git dir — skip
			continue
		}

		// check if this directory is a git repo
		if git.IsRepo(fullPath) {
			topLevel, err := git.TopLevel(fullPath)
			if err != nil {
				continue
			}

			if seen[topLevel] {
				continue
			}
			seen[topLevel] = true

			name := filepath.Base(topLevel)
			group := ""
			if autoGroup {
				group = computeGroup(root, topLevel)
			}

			*repos = append(*repos, config.Repo{
				Name:  name,
				Path:  topLevel,
				Group: group,
			})
			// don't recurse into a git repo's subdirectories
			continue
		}

		// not a git repo — recurse if allowed
		if recursive {
			_ = walkDir(root, fullPath, recursive, autoGroup, seen, repos)
		}
	}
	return nil
}

// computeGroup returns the subdirectory name under root as the group.
// e.g. root=/Users/qi/work, path=/Users/qi/work/nix/expnix -> "nix"
func computeGroup(root, path string) string {
	// resolve symlinks to handle macOS /var -> /private/var
	root, _ = filepath.EvalSymlinks(root)
	path, _ = filepath.EvalSymlinks(path)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) > 1 {
		return parts[0]
	}
	return ""
}
