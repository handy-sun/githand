package main

import (
	"fmt"
	"strings"

	"github.com/handy-sun/githand/internal/display"
	"github.com/handy-sun/githand/internal/flakeupdate"
	"github.com/handy-sun/githand/internal/i18n"
	"github.com/spf13/cobra"
)

var flakeUpdateGroup string

var flakeUpdateCmd = &cobra.Command{
	Use:   "flake-update [repo]",
	Short: i18n.T("flake-update.short"),
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, cfg, reg := mustLoadConfig()

		workers := cfg.Status.Workers
		repoName := ""
		if len(args) > 0 {
			repoName = args[0]
		}
		updated, failed := 0, 0

		results, err := flakeupdate.Run(&reg, flakeUpdateGroup, repoName, workers, func(r flakeupdate.Result) {
			switch r.Status {
			case "updated":
				fmt.Println(display.Green(r.Name))
			case "error":
				fmt.Println(display.Redf("%s (error)", r.Name))
			default:
				fmt.Println(r.Name)
			}

			// Nix output (indented)
			if r.GitOut != "" {
				for _, line := range strings.Split(r.GitOut, "\n") {
					if s := strings.TrimSpace(line); s != "" {
						fmt.Printf("    %s\n", s)
					}
				}
			}

			switch r.Status {
			case "updated":
				fmt.Printf("    %s\n", display.Green(r.Detail))
				updated++
			case "up-to-date":
				fmt.Printf("    %s\n", r.Detail)
			case "skipped":
				fmt.Printf("    skipped: %s\n", r.Detail)
			case "error":
				if r.Detail != "" {
					fmt.Printf("    %s\n", display.Red(r.Detail))
				}
				failed++
			}

			fmt.Println()
		})
		if err != nil {
			return err
		}

		fmt.Println(i18n.Tf("flake-update.summary", len(results), updated, failed))
		return nil
	},
}

func init() {
	flakeUpdateCmd.Flags().StringVar(&flakeUpdateGroup, "group", "", i18n.T("flake-update.flag.group"))
}
