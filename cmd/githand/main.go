// Package main is the entry point for the githand CLI.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/handy-sun/githand/internal/config"
	"github.com/handy-sun/githand/internal/i18n"
	"github.com/spf13/cobra"
)

var (
	cfgDir   string
	langFlag string

	// Build-time variables injected via -ldflags -X
	version = "dev"
	commit  = "unknown"
	date    = "unknown"

	describeLongRE = regexp.MustCompile(`^(.*-[0-9]+)-g[0-9a-fA-F]+$`)
)

var rootCmd = &cobra.Command{
	Use:           "githand",
	Short:         i18n.T("root.short"),
	Long:          i18n.T("root.long"),
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if langFlag != "" {
			i18n.SetLocale(langFlag)
			applyTranslations(cmd.Root())
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgDir, "config-dir", "", i18n.T("root.flag.config-dir"))
	rootCmd.PersistentFlags().StringVar(&langFlag, "lang", "", i18n.T("root.flag.lang"))
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(snapshotCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(groupCmd)
	rootCmd.Version, _ = normalizeBuildInfo(version, commit)
	rootCmd.SetVersionTemplate(versionTemplate(version, commit, date))
	applyTranslations(rootCmd)
}

func versionTemplate(rawVersion, rawCommit, buildDate string) string {
	displayVersion, displayCommit := normalizeBuildInfo(rawVersion, rawCommit)
	return fmt.Sprintf("githand %s (commit: %s, built: %s)\n", displayVersion, displayCommit, buildDate)
}

func normalizeBuildInfo(rawVersion, rawCommit string) (string, string) {
	displayVersion := rawVersion
	displayCommit := strings.TrimSuffix(rawCommit, "-dirty")
	dirty := strings.HasSuffix(rawVersion, "-dirty") || strings.HasSuffix(rawCommit, "-dirty")

	displayVersion = strings.TrimSuffix(displayVersion, "-dirty")
	if matches := describeLongRE.FindStringSubmatch(displayVersion); matches != nil {
		displayVersion = matches[1]
	}

	if dirty {
		displayCommit += "-dirty"
	}
	return displayVersion, displayCommit
}

func main() {
	// Pre-parse --lang so locale is set before cobra generates help text.
	// This handles the case where --help is passed, which skips PersistentPreRun.
	for i, arg := range os.Args {
		if arg == "--lang" && i+1 < len(os.Args) {
			i18n.SetLocale(os.Args[i+1])
			applyTranslations(rootCmd)
			break
		}
		if strings.HasPrefix(arg, "--lang=") {
			i18n.SetLocale(strings.TrimPrefix(arg, "--lang="))
			applyTranslations(rootCmd)
			break
		}
	}
	// Wrap the default help function to ensure auto-generated commands
	// (completion, help) are translated before help text is rendered.
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		applyTranslations(cmd.Root())
		defaultHelp(cmd, args)
	})
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// localizedUsageTemplate returns a Cobra usage template with i18n-ized
// section headers and help/version strings.
func localizedUsageTemplate() string {
	return i18n.T("cobra.usage") + `{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

` + i18n.T("cobra.aliases") + `
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

` + i18n.T("cobra.examples") + `
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

` + i18n.T("cobra.available_cmds") + `{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

` + i18n.T("cobra.additional_cmds") + `{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

` + i18n.T("cobra.flags") + `
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

` + i18n.T("cobra.global_flags") + `
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

` + i18n.T("cobra.additional_help_topics") + `{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

` + i18n.T("cobra.use_help") + `{{end}}
`
}

// applyTranslations re-applies i18n strings to all cobra commands
// after a locale switch via --lang.
func applyTranslations(root *cobra.Command) {
	root.Short = i18n.T("root.short")
	root.Long = i18n.T("root.long")
	translateFlags(root, map[string]string{
		"config-dir": "root.flag.config-dir",
		"lang":       "root.flag.lang",
	})
	// Translate cobra built-in --help and --version flag descriptions.
	if f := root.Flags().Lookup("help"); f != nil {
		f.Usage = i18n.Tf("cobra.help_flag", root.Name())
	}
	if f := root.Flags().Lookup("version"); f != nil {
		f.Usage = i18n.Tf("cobra.version_flag", root.Name())
	}
	// Re-apply the usage template with new locale strings.
	root.SetUsageTemplate(localizedUsageTemplate())
	for _, sub := range root.Commands() {
		switch sub.Name() {
		case "completion":
			sub.Short = i18n.T("cobra.completion")
		case "help":
			sub.Short = i18n.T("cobra.help_about")
		case "scan":
			sub.Short = i18n.T("scan.short")
			translateFlags(sub, map[string]string{
				"recursive":  "scan.flag.recurse",
				"auto-group": "scan.flag.group",
			})
		case "status":
			sub.Short = i18n.T("status.short")
			translateFlags(sub, map[string]string{
				"filter": "status.flag.filter",
				"group":  "status.flag.group",
				"user":   "status.flag.owner",
				"json":   "status.flag.json",
			})
		case "snapshot":
			sub.Short = i18n.T("snapshot.short")
			translateFlags(sub, map[string]string{
				"output": "snapshot.flag.output",
				"group":  "snapshot.flag.group",
				"filter": "snapshot.flag.filter",
			})
		case "restore":
			sub.Short = i18n.T("restore.short")
			translateFlags(sub, map[string]string{
				"base-path": "restore.flag.base-path",
				"dry-run":   "restore.flag.dry-run",
			})
		case "ls":
			sub.Short = i18n.T("ls.short")
		case "rm":
			sub.Short = i18n.T("rm.short")
		case "group":
			sub.Short = i18n.T("group.short")
			for _, gsub := range sub.Commands() {
				switch gsub.Name() {
				case "add":
					gsub.Short = i18n.T("group.add.short")
				case "rm":
					gsub.Short = i18n.T("group.rm.short")
				case "ls":
					gsub.Short = i18n.T("group.ls.short")
				}
				if f := gsub.Flags().Lookup("help"); f != nil {
					f.Usage = i18n.Tf("cobra.help_flag", gsub.Name())
				}
			}
		}
		// Translate --help flag for each sub-command.
		if f := sub.Flags().Lookup("help"); f != nil {
			f.Usage = i18n.Tf("cobra.help_flag", sub.Name())
		}
	}
}

// translateFlags updates the usage text of named flags.
func translateFlags(cmd *cobra.Command, flagKeys map[string]string) {
	for name, key := range flagKeys {
		if f := cmd.Flags().Lookup(name); f != nil {
			f.Usage = i18n.T(key)
		}
		if f := cmd.PersistentFlags().Lookup(name); f != nil {
			f.Usage = i18n.T(key)
		}
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
