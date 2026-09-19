# githand

Git workspace sync and migration CLI. The tool scans directories for git repos, displays multi-repo status, pulls latest changes, snapshots full repo state, and restores that state on another machine, including uncommitted work.

Implemented in Go as a single binary that shells out to the system `git` for all repository operations.

## Command Surface

```text
githand scan <path>...                 # scan one or more roots, register repos
githand scan <path> --recursive        # scan recursively
githand scan <path> --auto-group       # auto-create groups by subdirectory

githand status                         # show all repo statuses
githand status --sync                  # auto-sync registry (detect added/removed repos)
githand status --filter dirty          # only repos with uncommitted changes
githand status --filter ahead          # only repos ahead of remote
githand status --filter stash          # only repos with stash entries
githand status --filter detached       # only repos in detached HEAD
githand status --group nix             # only repos in group "nix"
githand status --user handy-sun        # filter by remote URL owner
githand status --json                  # machine-readable output

githand sync                           # pull latest for all registered repos
githand sync --group nix               # only repos in group "nix"
githand sync --remote upstream         # pull from a non-default remote

githand flake-update                   # update Nix flake inputs for repos with flake.nix
githand flake-update my-repo           # update a single registered repo
githand flake-update --group nix       # only repos in group "nix"

githand snapshot [-o output_dir]       # snapshot all registered repos
githand snapshot --group nix           # snapshot only a group
githand snapshot --filter dirty        # snapshot only matching repos
githand snapshot --archive             # also pack the snapshot directory as .tar

githand restore <snapshot.json|dir> <target_dir>
githand restore <snapshot.json|dir> <target_dir> --dry-run

githand ls                             # list repo names
githand rm <name>                      # remove repo from registry
githand group add <group> <repos...>   # manage groups
githand group rm <group>
githand group ls
```

The `restore` command accepts either the snapshot directory (containing `snapshot.json`) or the JSON file directly. Snapshot output may be a single `.json` file when no payload files are needed, or a directory layout when untracked files or incremental Git bundles must travel alongside the manifest.

## Implementation Layout

```text
cmd/githand/           # main package and CLI wiring
internal/config/       # githand.toml and repos.toml load/save
internal/git/          # system git command wrapper and git parsing helpers
internal/discover/     # repo discovery
internal/status/       # status collection and filtering
internal/sync/         # parallel pull worker pool
internal/flakeupdate/  # Nix flake update for repos with flake.nix
internal/snapshot/     # snapshot model, JSON serialization, payload archives
internal/restore/      # clone, checkout, patch apply, stash restore, extraction
internal/display/      # terminal formatting (colors, table layout) and JSON output
internal/i18n/         # English and Chinese translations, locale selection
```

Dependencies (kept minimal):

- CLI: `github.com/spf13/cobra`
- TOML: `github.com/pelletier/go-toml/v2`
- Bounded concurrency: `golang.org/x/sync/errgroup`

Git operations are backed by the system `git` command through `os/exec`. Do not replace core behavior with `go-git` unless there is a narrow, well-tested reason. This project depends on porcelain behavior such as `git diff`, `git apply`, `git stash`, `git clone`, `git pull`, branch tracking, and config handling.

The build is pure Go with cgo disabled so cross-compilation remains straightforward.

## File Formats

User-edited local state and configuration use TOML. Snapshots use JSON because they are machine-generated migration artifacts containing large structured data and multi-line patch text.

### Config File

Path:

```text
~/.config/githand/githand.toml
```

If `GITHAND_HOME` is set, use that directory instead of `~/.config/githand`.

Format:

```toml
version = 1

[scan]
recursive = false
auto_group = true

[status]
workers = 8
json = false
auto_sync = true

[snapshot]
output_dir = "~/.cache/githand"
include_clean = true

[restore]
dry_run = false
```

Configuration provides defaults only. Explicit CLI flags override config file values.

### Repo Registry

Path:

```text
~/.config/githand/repos.toml
```

Format:

```toml
version = 1
base_path = "/Users/qi/work"
base_paths = ["/Users/qi/work", "/srv/code"]

[[repos]]
name = "githand"
path = "/Users/qi/work/githand"
group = "tools"

[[repos]]
name = "expnix"
path = "/srv/code/nix/expnix"
group = "nix"

[groups]
tools = ["githand"]
nix = ["expnix"]
```

Design notes:

- `base_paths` lists every registered workspace root; each scan adds the directories it was pointed at (exact-deduplicated after symlink resolution). Registries with a single root write only `base_path`, keeping the legacy layout byte-identical.
- `base_path` is the primary workspace root and always equals `base_paths[0]` after loading. It is set by the first scan and never moved by later scans.
- `repos[*].path` remains absolute in the registry to avoid ambiguity during local operations.
- `repos[*].group` is a convenience tag from scan-time auto-grouping.
- `[groups]` stores named manual groups. A repo can match a group by either explicit group membership or its `group` field.
- `version` is required so future schema changes can fail clearly or migrate deliberately.

### Snapshot Format

Snapshot metadata lives as JSON. Two layouts depending on payload:

```text
# JSON-only (no payload files)
githand-snapshot.MMDD-HHmmss.json

# Directory layout (when untracked files or unpushed commits are captured)
githand-snapshot.MMDD-HHmmss/
  snapshot.json
  untracked/
    repo-name/
  bundles/
    encoded-repo-name.bundle
```

With `--archive`, the directory layout is also packed as `githand-snapshot.MMDD-HHmmss.tar` next to the folder.

The JSON manifest is authoritative and includes:

- schema version
- creation timestamp
- source hostname
- source base paths (`base_path` plus `base_paths` when more than one root is registered)
- repos, each anchored at a workspace root: `rel_path` is relative to the repo's anchor root, and repos anchored away from the primary root record that root in a per-repo `base_path` field
- groups
- remotes
- branches
- current branch or detached HEAD commit
- whether an incremental Git bundle is included for unpushed HEAD commits
- `core.hooksPath` config (effective value)
- dirty flags
- staged patch text
- unstaged patch text
- stash patch text
- untracked file paths

A repo that lives under no registered root (only possible in hand-edited registries) records its absolute path instead of a relative one.

Binary untracked files and incremental Git bundles are kept as files under the snapshot directory, not inlined as base64 in JSON.

## Core Flows

### scan

Accepts one or more root directories, validates all of them up front, then for each root: walk directories, identify git repos, deduplicate by absolute path, assign optional auto-group. Repos merge into the existing registry without overwriting it. Every root that yielded repos is registered under `base_paths` (exact-deduplicated after symlink resolution); the first root ever registered becomes the primary `base_path` and is never moved by later scans. A root with no repos is reported but not registered.

### status

Load `repos.toml` and `githand.toml`, apply static filters, collect repo statuses concurrently, then apply dirty/ahead/stash/detached filters that require git status data. With `--sync` (or `status.auto_sync = true` in config), the registry is reconciled against disk before status collection: removed repos are pruned, new repos under every registered base path are added.

Use a bounded worker count from config or CLI. Default to 8 workers.

### sync

Load registry and config, then for each repo in parallel (bounded worker count):

1. skip if not a git repo
2. if detached HEAD, `git fetch` only and report `fetched`
3. otherwise record HEAD, run `git pull --autostash [--rebase|--no-rebase] <remote> <branch>`
4. classify the result as `updated`, `up-to-date`, or `error` (parsing both old `Already up to date.` and git 2.54+ `ok` outputs)
5. stream the repo's git output inline, coloring the repo name green on update and red on error

`pull.rebase` is honored per-repo. `--remote` defaults to `origin`.

### flake-update

Load registry and config, select all repos, one named repo, or a group, then process each selected repo in parallel (bounded worker count):

1. skip if not a git repo
2. skip if no `flake.nix` in repo root
3. skip if the repo is in detached HEAD
4. record HEAD hash, run `nix flake update --commit-lock-file`
5. classify the result as `updated`, `up-to-date`, or `error`
6. stream nix output inline, coloring the repo name green on update and red on error

Only repos with a `flake.nix` are processed. Dirty worktrees are still processed; detached HEAD repos are skipped to avoid creating branchless update commits. An optional repo argument limits the update to one registered repo. The `--group` flag limits which repos are checked. The `nix` binary must be available on `PATH`.

### snapshot

Load registry and config, select repos, then for each repo:

1. collect remotes, branches, current branch, HEAD commit, dirty flags
2. collect `core.hooksPath` (effective value via `git config --get`)
3. collect staged patch with `git diff --cached`
4. collect unstaged patch with `git diff`
5. collect stash patches from `git stash list` and `git stash show -p`
6. detect whether HEAD is reachable from any remote-tracking ref
7. collect untracked files with `git ls-files --others --exclude-standard`
8. write an incremental Git bundle when HEAD contains unpushed commits
9. copy untracked files into the sibling `untracked/<repo>/` directory
10. anchor the repo at the longest registered base path that contains it (symlink-resolved comparison) and compute its relative path against that root; record the anchor in a per-repo `base_path` field when it differs from the primary root, or the absolute path when no root contains the repo
11. write the workspace snapshot JSON manifest

When no payload files are captured, the snapshot is a single `.json` file. Otherwise the directory layout is used so untracked files and Git bundles travel next to `snapshot.json`. `--archive` additionally writes a `.tar` of the directory.

### restore

Read snapshot JSON (and locate sibling payload directories if present). Compute every target path up front: repos anchored at the primary base path land directly under `target_dir`, repos anchored at another registered root nest under `target_dir/<base name of that root>/`, and a repo recording an absolute path (no matching root) restores under `target_dir` with the absolute path's components appended. If two repos resolve to the same target path, the whole restore fails before touching the disk. Then for each repo:

1. clone from the primary remote, or update an existing repo (refuse if local working tree is dirty)
2. add or reconcile additional remotes
3. import the incremental Git bundle when the recorded HEAD was not pushed
4. checkout the original branch, or checkout the recorded commit for detached HEAD
5. set up upstream tracking for the current branch
6. restore `core.hooksPath` config (written to local repo config)
7. apply staged patch with `git apply --cached`
8. apply unstaged patch with `git apply`
9. recreate stash entries from stash patches (`git apply --index` with `--3way` fallback, then `git stash`)
10. copy untracked files from the snapshot directory

After restore, status should match the original dirty state as closely as Git permits.

## Testing Priorities

Tests use temporary directories and real `git` commands. The important behavior is compatibility with Git, not mocked command strings.

Coverage already includes:

- registry TOML parse/write round trips, including `base_paths` migration from the legacy single-root layout
- config defaulting and CLI override behavior
- scan on nested temp directories, multi-root scans, and base path deduplication
- status collection on temp git repos
- sync: clean up-to-date, pull new commit, dirty worktree (autostash), detached HEAD (fetch-only), non-git dir, group filter, `pull.rebase` honoring, default remote
- snapshot/restore end-to-end with staged, unstaged, stash, and untracked files
- binary untracked file preservation
- detached HEAD restore
- `core.hooksPath` capture and restore (fresh clone and existing-repo update)
- multi-root anchoring: per-repo `base_path` recording, longest-prefix anchor selection, secondary-root restore layout, and target path conflict preflight
- snapshot directory layout, single-JSON output, and `.tar` archive contents

When adding behavior, add a test alongside it.

## Design Decisions

- Single Go binary for portability, fast startup, and cross-compilation. Shells out to system `git` for repository behavior.
- Local config and repo registry use TOML because humans may read and edit them.
- Snapshot manifests use JSON because they are machine-generated and contain large structured data plus multi-line patches.
- System `git` remains the source of truth for repository behavior.
- Incremental Git bundles preserve unpushed commits reachable from the captured HEAD; patch text plus copied untracked files preserve dirty state.
- Snapshot schema is versioned via the `schema` field so future changes can fail clearly or migrate deliberately. New optional fields (e.g. `hooks_path`) are added with `omitempty` and remain backward-compatible.
- Multiple workspace roots are supported through the `base_paths` registry list. Repos keep absolute paths locally; root-relative structure is materialized only at snapshot time by anchoring each repo at the longest matching root. Secondary roots restore nested under their base name so separate workspaces cannot silently merge or collide, and the restore preflight rejects ambiguous layouts before touching the disk.
- `restore` writes captured config values like `core.hooksPath` to local repo config so they travel with the restore without mutating global/system config.
