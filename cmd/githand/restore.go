package main

import (
	"github.com/handy-sun/githand/internal/restore"
	"github.com/spf13/cobra"
)

var (
	restoreBasePath string
	restoreDryRun   bool
)

var restoreCmd = &cobra.Command{
	Use:   "restore <snapshot.json> <target_dir>",
	Short: "Restore repos from a snapshot",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		snapPath := args[0]
		targetDir := args[1]

		_, cfg, _ := mustLoadConfig()

		dryRun := restoreDryRun
		if !cmd.Flags().Changed("dry-run") {
			dryRun = cfg.Restore.DryRun
		}

		return restore.Run(snapPath, targetDir, restoreBasePath, dryRun)
	},
}

func init() {
	restoreCmd.Flags().StringVar(&restoreBasePath, "base-path", "", "remap base path for restored repos")
	restoreCmd.Flags().BoolVar(&restoreDryRun, "dry-run", false, "show what would be done without making changes")
}
