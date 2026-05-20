package i18n

// catalogs maps locale codes to their key→string maps.
// Add new locales by adding an entry here and a corresponding file.
var catalogs = map[string]map[string]string{
	"en": en,
	"zh": zh,
}

var en = map[string]string{
	// root
	"root.short":           "Git workspace sync and migration CLI",
	"root.long":            "Scan directories for git repos, display multi-repo status, snapshot state, and restore on another machine.",
	"root.flag.config-dir": "config directory (default: $XDG_CONFIG_HOME/githand)",
	"root.flag.lang":       "output language (en, zh)",

	// scan
	"scan.short":        "Scan a directory for git repos and register them",
	"scan.flag.recurse": "scan subdirectories recursively",
	"scan.flag.group":   "auto-create groups by subdirectory name",
	"scan.none_found":   "No git repositories found.",
	"scan.result":       "Scanned %s: %d repos found, %d new added.",

	// status
	"status.short":       "Show status of all registered repos",
	"status.flag.filter": "filter: dirty, ahead, stash, detached",
	"status.flag.group":  "filter by group name",
	"status.flag.owner":  "filter by remote URL owner",
	"status.flag.json":   "machine-readable JSON output",

	// snapshot
	"snapshot.short":       "Snapshot all registered repos",
	"snapshot.flag.output": "parent directory for timestamped snapshot folders",
	"snapshot.flag.group":  "snapshot only repos in this group",
	"snapshot.flag.filter": "snapshot only matching repos (dirty, ahead, stash, detached)",
	"snapshot.written":     "Snapshot written to %s (%d repos)",

	// restore
	"restore.short":          "Restore repos from a snapshot",
	"restore.flag.base-path": "remap base path for restored repos",
	"restore.flag.dry-run":   "show what would be done without making changes",

	// ls
	"ls.short": "List registered repo names",

	// rm
	"rm.short":   "Remove a repo from the registry",
	"rm.removed": "Removed %q from registry.",

	// group
	"group.short":        "Manage repo groups",
	"group.add.short":    "Add repos to a group",
	"group.rm.short":     "Remove a group",
	"group.ls.short":     "List all groups",
	"group.flag.repos":   "repo names to add",
	"group.flag.name":    "group name",
	"group.added":        "Added %d repo(s) to group %q.",
	"group.removed":      "Removed group %q.",
	"group.none_defined": "No groups defined.",

	// display
	"display.no_repos": "No repositories registered.",
	"display.header":   "REPO\tBRANCH\tSTATUS\tAHEAD\tBEHIND\tSTASH",
	"display.clean":    "clean",
	"display.dirty":    "dirty",

	// restore (internal)
	"restore.progress": "Restoring %d repos from %s into %s",
	"restore.dry_run":  "[dry-run] would restore %s -> %s",
	"restore.restored": "restored %s",
}
