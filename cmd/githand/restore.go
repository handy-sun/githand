package main

import (
	"github.com/handy-sun/githand/internal/i18n"
	"github.com/handy-sun/githand/internal/restore"
	"github.com/spf13/cobra"
)

var (
	restoreDryRun bool
)

var restoreCmd = &cobra.Command{
	Use:   "restore <snapshot> <target_dir>",
	Short: i18n.T("restore.short"),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		snapPath := args[0]
		targetDir := args[1]

		_, cfg, _ := mustLoadConfig()

		dryRun := restoreDryRun
		if !cmd.Flags().Changed("dry-run") {
			dryRun = cfg.Restore.DryRun
		}

		return restore.Run(snapPath, targetDir, dryRun)
	},
}

func init() {
	restoreCmd.Flags().BoolVar(&restoreDryRun, "dry-run", false, i18n.T("restore.flag.dry-run"))
}
