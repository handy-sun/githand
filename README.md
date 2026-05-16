# githand

Git workspace sync & migration CLI — scan, status, snapshot, restore.

Move your entire git workspace to a new machine with one command, **including uncommitted changes**.

## Features

- **Full workspace migration** — snapshot every repo's state (remotes, branches, stashes, uncommitted changes, untracked files) and reproduce it on another machine
- **Dirty state preservation** — staged/unstaged diffs, stash entries, and untracked files (including binary) are all captured and restored
- **Parallel status** — collect status across all repos concurrently (configurable worker count)
- **Smart grouping** — auto-group repos by subdirectory name, or manage groups manually
- **Portable paths** — snapshots use relative paths internally; `--base-path` remaps the root on restore, so `/Users/you/work` becomes `/home/me/projects` seamlessly
- **Filter & query** — find dirty repos, repos ahead of remote, stashed repos, or repos by group/owner
- **JSON output** — machine-readable status output for scripting
- **TOML config** — registry and groups live under `~/.config/githand/` (config.toml + repos.toml)
- **Cobra CLI** — built with spf13/cobra for a polished command-line experience

## Installation

```bash
# clone and build
git clone https://github.com/handy-sun/githand.git
cd githand
go build -o bin/githand ./cmd/githand/

# or install with go install
go install github.com/handy-sun/githand/cmd/githand@latest
```

Requires Go 1.26+ and git. CGO disabled (pure Go build).

## Quick Start

```bash
# 1. Discover all git repos under your workspace
githand scan ~/work --recursive --auto-group

# 2. Check status across all repos
githand status

# 3. Snapshot before migrating to a new machine
githand snapshot -o my-workspace.json

# 4. On the new machine, restore everything
githand restore my-workspace.json ~/work --base-path ~/work
```

## Commands

### scan — discover and register repos

```bash
githand scan <path>                    # scan a directory for git repos
githand scan <path> -r                 # scan subdirectories recursively
githand scan <path> --auto-group       # auto-create groups by subdirectory name
```

On first scan, the directory is recorded as `base_path`. Subsequent scans preserve it. Repos already in the registry are skipped.

### status — show repo status

```bash
githand status                         # show all repos
githand status --filter dirty          # only repos with uncommitted changes
githand status --filter ahead          # only repos ahead of remote
githand status --filter stash          # only repos with stash entries
githand status --filter detached       # only repos with detached HEAD
githand status --group nix             # only repos in group "nix"
githand status --user handy-sun        # filter by remote URL owner
githand status --json                  # machine-readable JSON output
```

**Status symbols:**

| Symbol | Meaning |
|--------|---------|
| `+` | Staged changes |
| `!` | Unstaged changes |
| `?` | Untracked files |
| `$` | Stash entries |
| `D` | Detached HEAD |
| `clean` | None of the above |

**Sync indicators:**

| Symbol | Meaning |
|--------|---------|
| `=` | In sync with remote |
| `↑` | Local ahead |
| `↓` | Remote ahead |
| `↕` | Diverged |
| `-` | No remote configured |

### snapshot — serialize workspace for migration

```bash
githand snapshot                       # snapshot all registered repos
githand snapshot -o output.json        # custom output path
githand snapshot --group nix           # only repos in group "nix"
githand snapshot --filter dirty        # only repos with uncommitted changes
```

Produces a JSON file + sibling data directory:

```
workspace-snapshot-20260515-221241.json       # all metadata + patch text
workspace-snapshot-20260515-221241-data/      # untracked file tarballs
  untracked/
    expnix.tar.gz
    githand.tar.gz
```

**What gets captured per repo:**

- Remote URLs
- All local branches with upstream tracking
- Current branch and HEAD commit
- Staged diff (`git diff --cached`)
- Unstaged diff (`git diff`)
- Stash entries (each as a full patch)
- Untracked files (tar.gz archive, respects `.gitignore`)

### restore — reproduce workspace on a new machine

```bash
githand restore <snapshot.json> <target_dir>
githand restore <snapshot.json> <target_dir> --base-path /new/root
githand restore <snapshot.json> <target_dir> --dry-run
```

Restore replays each repo's snapshot in order:

1. `git clone` from primary remote
2. Add additional remotes
3. `git checkout` the original branch
4. Apply staged patch (`git apply --cached`)
5. Apply unstaged patch (`git apply`)
6. Apply stash patches (`git apply --index` + `git stash` for each)

The `--base-path` flag remaps the snapshot's original root to a new path, preserving the relative directory structure. Without it, `target_dir` is used as the base.

### ls, rm — manage the registry

```bash
githand ls                             # list registered repo names
githand rm <name>                      # remove a repo from the registry
```

### group — organize repos

```bash
githand group add <group> <repo...>    # add repos to a group
githand group rm <group>               # remove a group
githand group ls                       # list all groups
```

## How It Works

### Path Portability

Snapshots store relative paths, not absolute ones. When you scan `~/work`, the base path `/Users/you/work` is recorded. At snapshot time, each repo's path is computed relative to this base (e.g. `nix/expnix`, `agent-switch/cc-switch`).

On restore, `--base-path` sets the new root. The relative structure is preserved:

```
Machine A:  /Users/you/work/nix/expnix
                         ^^^^^^^^^^^^^  relative path
Machine B:  /home/me/projects/nix/expnix  (--base-path /home/me/projects)
```

This means a single snapshot works across macOS, Linux, or any path layout.

### Why Not Store Relative Paths in the Registry?

If `base_path` changes (e.g. you rescan from a different directory), stored relative paths become stale. Computing them at snapshot time from the stored `base_path` + absolute `path` is simpler and always correct.

## Comparison with gita

[gita](https://github.com/nosarthur/gita) is a popular tool for managing multiple git repos. Here's how githand differs:

| | githand | gita |
|---|---|---|
| **Core purpose** | Workspace migration & snapshot | Multi-repo visibility & command dispatch |
| **Dirty state migration** | Full support (patches + untracked tar.gz) | Not supported — `freeze` only captures URL + branch |
| **Uncommitted changes** | Preserved across machines | Lost on freeze/clone |
| **Stash entries** | Serialized and restored | Not captured |
| **Untracked files** | Archived and restored | Not captured |
| **Cross-machine workflow** | `snapshot` → copy JSON + data → `restore` | `freeze` → `clone -f` (clean repos only) |
| **Batch git commands** | Not the focus | Core feature (`gita super`, `gita shell`) |
| **Custom command dispatch** | — | Yes (cmds.json, super, shell) |
| **Status display** | Per-repo detail view | Compact side-by-side `gita ll` |
| **Parallel status** | Goroutine pool | Async execution |
| **Filter by state** | dirty / ahead / stash / detached | Color-coded display |
| **Group by subdirectory** | `--auto-group` on scan | `add -a` on add |
| **Path portability** | Relative paths + `--base-path` | `clone -p` preserves paths |
| **JSON output** | `--json` flag | No |
| **Implementation** | Go (single binary) | Python (pip package) |

**TL;DR:** Use **gita** if you need to run git commands across many repos from one place. Use **githand** if you need to move your entire workspace — with all its in-progress work — to a new machine.

## Development

```bash
cd githand
go build -o bin/githand ./cmd/githand/   # build
go test ./...                              # test
```

Config lives at `~/.config/githand/` — delete `repos.toml` to reset the registry.

## License

MIT
