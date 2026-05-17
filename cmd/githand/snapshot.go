package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/handy-sun/githand/internal/i18n"
	"github.com/handy-sun/githand/internal/snapshot"
	"github.com/spf13/cobra"
)

var (
	snapshotOutput string
	snapshotGroup  string
	snapshotFilter string
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: i18n.T("snapshot.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, cfg, reg := mustLoadConfig()

		// determine output path
		outPath := snapshotOutput
		if outPath == "" {
			ts := time.Now().Format("20060102-150405")
			outPath = filepath.Join(".", fmt.Sprintf("workspace-snapshot-%s.json", ts))
		}

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

		// write JSON
		dataDir := snapshot.DataDirPath(outPath)
		if err := snapshot.Write(snap, outPath, dataDir); err != nil {
			return fmt.Errorf("write snapshot: %w", err)
		}

		fmt.Println(i18n.Tf("snapshot.written", outPath, len(snap.Repos)))
		return nil
	},
}

func init() {
	snapshotCmd.Flags().StringVarP(&snapshotOutput, "output", "o", "", i18n.T("snapshot.flag.output"))
	snapshotCmd.Flags().StringVar(&snapshotGroup, "group", "", i18n.T("snapshot.flag.group"))
	snapshotCmd.Flags().StringVar(&snapshotFilter, "filter", "", i18n.T("snapshot.flag.filter"))
}