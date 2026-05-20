package main

import (
	"fmt"

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
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: i18n.T("status.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, cfg, reg := mustLoadConfig()

		// CLI flags override config
		asJSON := statusJSON
		if !cmd.Flags().Changed("json") {
			asJSON = cfg.Status.JSON
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
}
