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
	Use:   "scan <path>",
	Short: i18n.T("scan.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scanPath, err := filepath.Abs(args[0])
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

		// Discover repos
		found, err := discover.Discover(scanPath, recursive, autoGroup)
		if err != nil {
			return fmt.Errorf("scan: %w", err)
		}

		if len(found) == 0 {
			fmt.Println(i18n.T("scan.none_found"))
			return nil
		}

		// Merge into registry
		if reg.BasePath == "" {
			reg.BasePath = scanPath
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

		if err := config.SaveRegistry(dir, reg); err != nil {
			return fmt.Errorf("save registry: %w", err)
		}

		fmt.Println(i18n.Tf("scan.result", scanPath, len(found), added))
		return nil
	},
}

func init() {
	scanCmd.Flags().BoolVarP(&scanRecursive, "recursive", "r", false, i18n.T("scan.flag.recurse"))
	scanCmd.Flags().BoolVar(&scanAutoGroup, "auto-group", true, i18n.T("scan.flag.group"))
}