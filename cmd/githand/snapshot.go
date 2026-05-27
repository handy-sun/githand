package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/handy-sun/githand/internal/i18n"
	"github.com/handy-sun/githand/internal/snapshot"
	"github.com/spf13/cobra"
)

var (
	snapshotOutput  string
	snapshotGroup   string
	snapshotFilter  string
	snapshotArchive bool
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: i18n.T("snapshot.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, cfg, reg := mustLoadConfig()

		// determine output directory
		parentDir := cfg.Snapshot.OutputDir
		if snapshotOutput != "" {
			parentDir = snapshotOutput
		}
		snapDir := snapshot.DefaultSnapshotDir(expandHome(parentDir))

		// select repos
		repos := reg.Repos
		if snapshotGroup != "" {
			repos = reg.ReposInGroup(snapshotGroup)
		}

		// snapshot
		snap, err := snapshot.Take(&reg, repos, cfg.Snapshot.IncludeClean)
		if err != nil {
			return fmt.Errorf("snapshot: %w", err)
		}

		// apply filter after collection
		if snapshotFilter != "" {
			snap.Repos = snapshot.Filter(snap.Repos, snapshotFilter)
		}

		// write snapshot output
		writtenPath, err := snapshot.WriteOutput(snap, snapDir, reg.BasePath, snapshotArchive)
		if err != nil {
			return fmt.Errorf("write snapshot: %w", err)
		}

		fmt.Println(i18n.Tf("snapshot.written", writtenPath, len(snap.Repos)))
		return nil
	},
}

func init() {
	snapshotCmd.Flags().StringVarP(&snapshotOutput, "output", "o", "", i18n.T("snapshot.flag.output"))
	snapshotCmd.Flags().StringVar(&snapshotGroup, "group", "", i18n.T("snapshot.flag.group"))
	snapshotCmd.Flags().StringVar(&snapshotFilter, "filter", "", i18n.T("snapshot.flag.filter"))
	snapshotCmd.Flags().BoolVar(&snapshotArchive, "archive", false, i18n.T("snapshot.flag.archive"))
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
