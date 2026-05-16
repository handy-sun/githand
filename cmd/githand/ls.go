package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List registered repo names",
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
