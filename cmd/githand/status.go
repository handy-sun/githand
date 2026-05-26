package main

import (
	"fmt"

	"github.com/handy-sun/githand/internal/config"
	"github.com/handy-sun/githand/internal/display"
	"github.com/handy-sun/githand/internal/i18n"
	"github.com/handy-sun/githand/internal/status"
	"github.com/spf13/cobra"
)

var (
	statusFilter string
	statusGroup  string
	statusOwner  string
	statusJSON   bool
	statusSync   bool
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: i18n.T("status.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, cfg, reg := mustLoadConfig()

		// CLI flags override config
		asJSON := statusJSON
		if !cmd.Flags().Changed("json") {
			asJSON = cfg.Status.JSON
		}

		autoSync := statusSync
		if !cmd.Flags().Changed("sync") {
			autoSync = cfg.Status.AutoSync
		}

		// Sync registry if enabled
		if autoSync {
			syncResult, err := status.SyncRegistry(&reg, cfg.Scan.Recursive, cfg.Scan.AutoGroup)
			if err != nil {
				return fmt.Errorf("sync registry: %w", err)
			}

			// Save updated registry if changes were made
			if syncResult.Added > 0 || syncResult.Removed > 0 {
				if err := config.SaveRegistry(dir, reg); err != nil {
					return fmt.Errorf("save registry: %w", err)
				}

				// Report changes
				if syncResult.Added > 0 {
					fmt.Println(i18n.Tf("status.sync_added", syncResult.Added))
				}
				if syncResult.Removed > 0 {
					fmt.Println(i18n.Tf("status.sync_removed", syncResult.Removed))
				}
			}
		}

		workers := cfg.Status.Workers
		results, err := status.Collect(&reg, workers)
		if err != nil {
			return fmt.Errorf("collect status: %w", err)
		}

		// apply static filters first
		if statusGroup != "" {
			results = status.FilterByGroup(results, &reg, statusGroup)
		}
		if statusOwner != "" {
			results = status.FilterByUser(results, statusOwner)
		}

		// apply dynamic filters that need git data
		if statusFilter != "" {
			results = status.FilterByFlag(results, statusFilter)
		}

		return display.Status(results, asJSON)
	},
}

func init() {
	statusCmd.Flags().StringVar(&statusFilter, "filter", "", i18n.T("status.flag.filter"))
	statusCmd.Flags().StringVar(&statusGroup, "group", "", i18n.T("status.flag.group"))
	statusCmd.Flags().StringVar(&statusOwner, "user", "", i18n.T("status.flag.owner"))
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, i18n.T("status.flag.json"))
	statusCmd.Flags().BoolVar(&statusSync, "sync", false, i18n.T("status.flag.sync"))
}
