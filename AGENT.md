# githand

Git workspace sync and migration CLI. The tool scans a directory for git repos, displays multi-repo status, snapshots full repo state, and restores that state on another machine, including uncommitted work.

## Migration Direction

The next major implementation should rewrite the project in Go.

Primary goals:

- Ship a portable single-file binary for macOS, Linux, and Windows.
- Keep runtime dependencies minimal. The binary may depend on the system `git` executable, but should not require Python, uv, or a language runtime.
- Improve startup time, concurrency control, and packaging.
- Use the rewrite as a practical Go learning project covering CLI design, subprocesses, filesystem work, archive handling, structured config, and tests.

Do not preserve compatibility with the current Python module layout, internal APIs, or old JSON registry files. The command surface may stay conceptually similar, but the implementation can be redesigned around Go packages and TOML-based local state.

## Command Surface

Keep the main commands recognizable:

```text
githand scan <path>                    # scan directory, register repos
githand scan <path> --recursive        # scan recursively
githand scan <path> --auto-group       # auto-create groups by subdirectory

githand status                         # show all repo statuses
githand status --filter dirty          # only repos with uncommitted changes
githand status --filter ahead          # only repos ahead of remote
githand status --filter stash          # only repos with stash entries
githand status --filter detached       # only repos in detached HEAD
githand status --group nix             # only repos in group "nix"
githand status --user handy-sun        # filter by remote URL owner
githand status --json                  # machine-readable output

githand snapshot [-o output_dir]       # snapshot all registered repos
githand snapshot --group nix           # snapshot only a group
githand snapshot --filter dirty        # snapshot only matching repos

githand restore <snapshot.json> <target_dir>
githand restore <snapshot.json> <target_dir> --base-path <new_root>
githand restore <snapshot.json> <target_dir> --dry-run

githand ls                             # list repo names
githand rm <name>                      # remove repo from registry
githand group add <group> <repos...>   # manage groups
githand group rm <group>
githand group ls
```

The exact flags may evolve during the Go rewrite, but avoid changing semantics without a clear reason.

## Go Implementation Guidance

Prefer a simple package layout:

```text
cmd/githand/           # main package and CLI wiring
internal/config/       # config.toml and repos.toml load/save
internal/git/          # system git command wrapper and git parsing helpers
internal/discover/     # repo discovery
internal/status/       # status collection and filtering
internal/snapshot/     # snapshot model, JSON serialization, untracked archives
internal/restore/      # clone, checkout, patch apply, stash restore, extraction
internal/display/      # terminal formatting and JSON output
```

Recommended libraries:

- CLI: `github.com/spf13/cobra`
- TOML: `github.com/pelletier/go-toml/v2`
- Optional concurrent error handling: `golang.org/x/sync/errgroup`

Keep Git operations backed by the system `git` command through `os/exec`. Do not replace core behavior with `go-git` unless there is a narrow, well-tested reason. This project depends on porcelain behavior such as `git diff`, `git apply`, `git stash`, `git clone`, branch tracking, and config handling.

Keep the implementation pure Go with cgo disabled where possible so cross-compilation remains straightforward.

## File Formats

Use TOML for user-edited local state and configuration. Keep snapshots as JSON because they are machine-generated migration artifacts containing large structured data and multi-line patch text.

No compatibility with old `repos.json` is required.

### Config File

Path:

```text
$XDG_CONFIG_HOME/githand/config.toml
```

If `XDG_CONFIG_HOME` is unset, use the platform-appropriate user config directory.

Proposed format:

```toml
version = 1

[scan]
recursive = true
auto_group = true

[status]
workers = 8
json = false

[snapshot]
output_dir = "~/backups/githand"
include_clean = true

[restore]
dry_run = false
```

Configuration should provide defaults only. Explicit CLI flags override config file values.

### Repo Registry

Path:

```text
$XDG_CONFIG_HOME/githand/repos.toml
```

Proposed format:

```toml
version = 1
base_path = "/Users/qi/work"

[[repos]]
name = "githand"
path = "/Users/qi/work/githand"
group = "tools"

[[repos]]
name = "expnix"
path = "/Users/qi/work/nix/expnix"
group = "nix"

[groups]
tools = ["githand"]
nix = ["expnix"]
```

Design notes:

- `base_path` is the workspace root used to compute portable relative paths at snapshot time.
- `repos[*].path` remains absolute in the registry to avoid ambiguity during local operations.
- `repos[*].group` is a convenience tag from scan-time auto-grouping.
- `[groups]` stores named manual groups. A repo can match a group by either explicit group membership or its `group` field.
- `version` is required so future schema changes can fail clearly or migrate deliberately.

### Snapshot Format

Keep snapshot metadata as JSON:

```text
githand-snapshot.MMDD-HHmmss/
  snapshot.json
  untracked/
    repo-name/
```

The JSON should remain the authoritative manifest:

- schema version
- creation timestamp
- source hostname
- source base path
- repos
- groups
- remotes
- branches
- current branch or detached HEAD commit
- git config data when needed
- dirty flags
- staged patch text
- unstaged patch text
- stash patch text
- untracked file paths

Do not store binary untracked files as base64 in JSON. Keep them as files under the snapshot directory.

If a single-file transfer format is needed later, add an archive command that packages:

```text
snapshot.json
data/
  untracked/...
```

inside a `.tar.gz`. The internal manifest should still be JSON.

## Core Flows

### scan

Resolve path, walk directories, identify git repos, deduplicate by absolute path, assign optional auto-group, write `repos.toml`.

### status

Load `repos.toml` and `config.toml`, apply static filters, collect repo statuses concurrently, then apply dirty/ahead/stash/detached filters that require git status data.

Use a bounded worker count from config or CLI. Default to 8 workers.

### snapshot

Load registry and config, select repos, then for each repo:

1. collect remotes, branches, current branch, HEAD commit, dirty flags
2. collect staged patch with `git diff --cached`
3. collect unstaged patch with `git diff`
4. collect stash patches from `git stash list` and `git stash show -p`
5. collect untracked files with `git ls-files --others --exclude-standard`
6. archive untracked files into the sibling data directory
7. compute repo path relative to `base_path`
8. write the workspace snapshot JSON manifest

### restore

Read snapshot JSON, locate sibling data directory, then for each repo:

1. compute target repo path from restore base plus snapshot relative path
2. clone from the primary remote
3. add additional remotes
4. checkout the original branch, or checkout the recorded commit for detached HEAD
5. create or track non-current local branches where possible
6. merge extra git config non-destructively
7. apply staged patch with `git apply --cached`
8. apply unstaged patch with `git apply`
9. recreate stash entries from stash patches
10. extract untracked file archives

After restore, status should match the original dirty state as closely as Git permits.

## Testing Priorities

The Go rewrite should add tests before expanding behavior:

- registry TOML parse/write round trips
- config defaulting and CLI override behavior
- scan on nested temp directories
- status collection on temp git repos
- snapshot/restore end-to-end with staged, unstaged, stash, and untracked files
- binary untracked file preservation
- detached HEAD restore
- path remapping with `--base-path`

Use temporary directories and real `git` commands in integration tests. The important behavior is compatibility with Git, not mocked command strings.

## Design Decisions

- Go rewrite is accepted for portability, packaging, startup speed, and learning value.
- Local config and repo registry should use TOML because humans may read and edit them.
- Snapshot manifests should stay JSON because they are machine-generated and contain large structured data plus multi-line patches.
- The project does not need to preserve old Python internals or old JSON registry compatibility.
- System `git` remains the source of truth for repository behavior.
- Patch text plus copied untracked files remain the migration strategy for dirty state.
- Snapshot schema versioning remains required.
