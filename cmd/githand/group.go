package main

import (
	"fmt"

	"github.com/handy-sun/githand/internal/config"
	"github.com/handy-sun/githand/internal/i18n"
	"github.com/spf13/cobra"
)

var groupCmd = &cobra.Command{
	Use:   "group",
	Short: i18n.T("group.short"),
}

var groupAddCmd = &cobra.Command{
	Use:   "add <group> <repos...>",
	Short: i18n.T("group.add.short"),
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		group := args[0]
		repoNames := args[1:]
		dir, _, reg := mustLoadConfig()

		if reg.Groups == nil {
			reg.Groups = make(map[string][]string)
		}

		existing := make(map[string]bool)
		for _, m := range reg.Groups[group] {
			existing[m] = true
		}

		for _, name := range repoNames {
			if !existing[name] {
				reg.Groups[group] = append(reg.Groups[group], name)
				existing[name] = true
			}
			// also set group field on the repo
			for i := range reg.Repos {
				if reg.Repos[i].Name == name {
					reg.Repos[i].Group = group
				}
			}
		}

		if err := config.SaveRegistry(dir, reg); err != nil {
			return fmt.Errorf("save registry: %w", err)
		}

		fmt.Println(i18n.Tf("group.added", len(repoNames), group))
		return nil
	},
}

var groupRmCmd = &cobra.Command{
	Use:   "rm <group>",
	Short: i18n.T("group.rm.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		group := args[0]
		dir, _, reg := mustLoadConfig()

		if _, ok := reg.Groups[group]; !ok {
			return fmt.Errorf("group %q not found", group)
		}

		// clear group field on member repos
		for _, name := range reg.Groups[group] {
			for i := range reg.Repos {
				if reg.Repos[i].Name == name && reg.Repos[i].Group == group {
					reg.Repos[i].Group = ""
				}
			}
		}

		delete(reg.Groups, group)

		if err := config.SaveRegistry(dir, reg); err != nil {
			return fmt.Errorf("save registry: %w", err)
		}

		fmt.Println(i18n.Tf("group.removed", group))
		return nil
	},
}

var groupLsCmd = &cobra.Command{
	Use:   "ls",
	Short: i18n.T("group.ls.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, _, reg := mustLoadConfig()
		if len(reg.Groups) == 0 {
			fmt.Println(i18n.T("group.none_defined"))
			return nil
		}
		for name, members := range reg.Groups {
			fmt.Printf("%s: %v\n", name, members)
		}
		return nil
	},
}

func init() {
	groupCmd.AddCommand(groupAddCmd)
	groupCmd.AddCommand(groupRmCmd)
	groupCmd.AddCommand(groupLsCmd)
}