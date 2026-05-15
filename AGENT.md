# githand

Git workspace sync & migration tool. Scans a directory for git repos, displays status, and snapshots/restores full repo state (including uncommitted changes) for machine migration.

## Commands

```
githand scan <path>                    # scan directory, register repos
githand scan <path> --recursive        # scan recursively
githand scan <path> --auto-group       # auto-create groups by subdirectory

githand status                         # show all repos status
githand status --filter dirty          # only repos with uncommitted changes
githand status --filter ahead          # only repos ahead of remote
githand status --filter stash          # only repos with stash entries
githand status --group nix             # only repos in group "nix"
githand status --user handy-sun        # filter by remote URL owner
githand status --json                  # machine-readable output

githand snapshot [-o output.json]      # snapshot all registered repos
githand snapshot --group nix           # snapshot only group
githand snapshot --filter dirty        # snapshot only dirty repos

githand restore <snapshot.json> <target_dir>   # restore to new machine
githand restore --dry-run              # preview what would happen

githand ls                             # list repo names
githand rm <name>                      # remove repo from registry
githand group add <group> <repos>      # manage groups
githand group rm <group>
```

## Architecture

```
src/githand/
  __init__.py          # version
  __main__.py          # CLI entry, argparse dispatch
  commands/
    __init__.py
    scan.py            # scan: discover repos under a path
    status.py          # status: display repo status
    snapshot.py        # snapshot: serialize repo state to file
    restore.py         # restore: deserialize + reproduce on new machine
  core/
    __init__.py
    models.py          # dataclasses: RepoRecord, RepoSnapshot, WorkspaceSnapshot
    discover.py        # directory scanning logic
    collect.py         # git info collection (status, patches, stash, untracked)
    serialize.py       # JSON serialization / deserialization
    restore_ops.py     # restore operations (clone, checkout, apply patches, etc.)
  config.py            # XDG path resolution, config loading
  display.py           # terminal output formatting (colors, table)
  utils.py             # subprocess helpers, path utilities
```

## Data Model

### RepoRecord (lightweight, for registry)

- name: str — display name
- path: str — absolute path
- group: Optional[str] — optional group tag

### RepoStatus (read-only view for status display)

- dirty: DirtyState — has_staged, has_unstaged, has_untracked, has_stash, is_detached
- branches: list[BranchInfo] — name, is_head, upstream, sync_status, head_commit
- current_branch: Optional[str]
- remotes: list[RemoteInfo] — name, url, fetch_refspec, push_url
- head_commit: Optional[str]
- commit_msg: Optional[str]
- commit_time: Optional[str]

### PatchData (serialized uncommitted changes)

- staged_patch: Optional[str] — git diff --cached
- unstaged_patch: Optional[str] — git diff
- stash_patches: list[str] — each stash entry
- untracked_tar: Optional[str] — relative path in tar.gz for untracked files
- untracked_manifest: list[str] — file list

### RepoSnapshot (full serializable state — core of migration)

- name, path, remotes, branches, current_branch, head_commit
- gitconfig: Optional[str] — .git/config content
- dirty: DirtyState
- patches: PatchData

### WorkspaceSnapshot (top-level container — one file = entire workspace)

- version: int — schema version
- created_at: str — ISO timestamp
- hostname: str — source machine
- base_path: str — original workspace root
- repos: list[RepoSnapshot]
- groups: dict[str, list[str]]

## Serialization Format

- `workspace-snapshot.json` — all metadata + patch text
- `workspace-snapshot-data.tar.gz` — binary untracked files

`untracked_tar` field in PatchData stores relative path inside the tar.gz, not base64.

## Core Flows

### scan
path -> walk directories -> is_git() check -> dedup -> RepoRecord -> save registry

### status
load registry -> ThreadPoolExecutor -> per repo: git commands -> RepoStatus -> display

### snapshot
load registry -> per repo:
1. collect RepoStatus (remotes, branches, dirty flags)
2. if dirty: collect PatchData
   - git diff --cached -> staged_patch
   - git diff -> unstaged_patch
   - git stash list -> for each: git stash show -p stash@{N} -> stash_patches
   - git ls-files --others --exclude-standard -> untracked_manifest
   - tar untracked files -> write to tar.gz -> record path in untracked_tar
3. read .git/config -> gitconfig
-> RepoSnapshot
aggregate -> WorkspaceSnapshot -> write JSON + tar.gz

### restore
read JSON + tar.gz -> per RepoSnapshot:
1. git clone <primary_remote_url> <target_dir/repo_name>
2. git checkout <current_branch> (or git checkout <head_commit> if detached)
3. if other branches exist: git branch --track <branch> <upstream>
4. restore .git/config (merge, don't overwrite)
5. if staged_patch: git apply --cached
6. if unstaged_patch: git apply
7. if stash_patches: for each: git apply + git stash
8. if untracked_tar: extract tar.gz into worktree
-> verify: git status should match original DirtyState

## Key Design Decisions

- Python + uv, not Rust or Shell
- JSON for structured data (not CSV like gita)
- dataclasses for type safety
- Parallel status collection via ThreadPoolExecutor
- Patch text for diffs, tar.gz for untracked binary files
- Schema versioning in WorkspaceSnapshot for future compatibility
