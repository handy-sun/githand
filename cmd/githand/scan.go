package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/handy-sun/githand/internal/config"
	"github.com/handy-sun/githand/internal/discover"
	"github.com/handy-sun/githand/internal/i18n"
	"github.com/spf13/cobra"
)

var (
	scanRecursive bool
	scanAutoGroup bool
)

var scanCmd = &cobra.Command{
	Use:   "scan <path>...",
	Short: i18n.T("scan.short"),
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// resolve and validate every path before touching the registry
		roots := make([]string, 0, len(args))
		for _, arg := range args {
			scanPath, err := filepath.Abs(arg)
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}

			info, err := os.Stat(scanPath)
			if err != nil {
				return fmt.Errorf("access path: %w", err)
			}
			if !info.IsDir() {
				return fmt.Errorf("%s is not a directory", scanPath)
			}
			roots = append(roots, scanPath)
		}

		dir, cfg, reg := mustLoadConfig()

		// CLI flags override config
		recursive := scanRecursive
		if !cmd.Flags().Changed("recursive") {
			recursive = cfg.Scan.Recursive
		}
		autoGroup := scanAutoGroup
		if !cmd.Flags().Changed("auto-group") {
			autoGroup = cfg.Scan.AutoGroup
		}

		if reg.Version == 0 {
			reg.Version = 1
		}
		if reg.Groups == nil {
			reg.Groups = make(map[string][]string)
		}

		existing := make(map[string]bool)
		for _, r := range reg.Repos {
			existing[r.Path] = true
		}

		changed := false
		for _, scanPath := range roots {
			// Discover repos
			found, err := discover.Discover(scanPath, recursive, autoGroup)
			if err != nil {
				return fmt.Errorf("scan: %w", err)
			}

			if len(found) == 0 {
				if len(roots) == 1 {
					fmt.Println(i18n.T("scan.none_found"))
					return nil
				}
				fmt.Println(i18n.Tf("scan.result", scanPath, 0, 0))
				continue
			}

			// Merge into registry
			added := 0
			for _, r := range found {
				if !existing[r.Path] {
					reg.Repos = append(reg.Repos, r)
					existing[r.Path] = true
					added++

					// register in groups map
					if r.Group != "" {
						reg.Groups[r.Group] = append(reg.Groups[r.Group], r.Name)
					}
				}
			}

			// record the scanned directory as a workspace root; the first
			// root ever registered stays the primary base_path
			if !registeredRoot(reg, scanPath) {
				reg.BasePaths = append(reg.BasePaths, scanPath)
			}
			if reg.BasePath == "" {
				reg.BasePath = scanPath
			}
			changed = true

			fmt.Println(i18n.Tf("scan.result", scanPath, len(found), added))
		}

		if !changed {
			return nil
		}
		if err := config.SaveRegistry(dir, reg); err != nil {
			return fmt.Errorf("save registry: %w", err)
		}
		return nil
	},
}

// registeredRoot reports whether root is already a workspace root in the
// registry, comparing symlink-resolved paths (macOS /var vs /private/var).
func registeredRoot(reg config.Registry, root string) bool {
	norm, err := filepath.EvalSymlinks(root)
	if err != nil {
		norm = root
	}
	for _, base := range reg.BasePaths {
		candidate, err := filepath.EvalSymlinks(base)
		if err != nil {
			candidate = base
		}
		if candidate == norm {
			return true
		}
	}
	return false
}

func init() {
	scanCmd.Flags().BoolVarP(&scanRecursive, "recursive", "r", false, i18n.T("scan.flag.recurse"))
	scanCmd.Flags().BoolVar(&scanAutoGroup, "auto-group", true, i18n.T("scan.flag.group"))
}
