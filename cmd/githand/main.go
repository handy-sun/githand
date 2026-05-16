// Package main is the entry point for the githand CLI.
package main

import (
	"fmt"
	"os"

	"github.com/handy-sun/githand/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgDir   string
	cfg      config.Config
	registry config.Registry

	// Build-time variables injected via -ldflags -X
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "githand",
	Short: "Git workspace sync and migration CLI",
	Long:  "Scan directories for git repos, display multi-repo status, snapshot state, and restore on another machine.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgDir, "config-dir", "", "config directory (default: $XDG_CONFIG_HOME/githand)")
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(snapshotCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(groupCmd)
	rootCmd.Version = version
	rootCmd.SetVersionTemplate(fmt.Sprintf("githand %s (commit: %s, built: %s)\n", version, commit, date))
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// loadConfigDir resolves the config directory, falling back to the default.
func loadConfigDir() (string, error) {
	if cfgDir != "" {
		return cfgDir, nil
	}
	return config.DefaultConfigDir()
}

// mustLoadConfig loads config and registry, exiting on error.
func mustLoadConfig() (string, config.Config, config.Registry) {
	dir, err := loadConfigDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg, err := config.LoadConfig(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	reg, err := config.LoadRegistry(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return dir, cfg, reg
}
