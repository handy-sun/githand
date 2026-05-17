package main

import (
	"fmt"

	"github.com/handy-sun/githand/internal/i18n"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: i18n.T("ls.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, _, reg := mustLoadConfig()
		for _, repo := range reg.Repos {
			if repo.Group != "" {
				fmt.Printf("%s (%s)\n", repo.Name, repo.Group)
			} else {
				fmt.Println(repo.Name)
			}
		}
		return nil
	},
}