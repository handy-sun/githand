"""Git info collection — status, patches, stash, untracked."""

from __future__ import annotations

import tarfile
from pathlib import Path
from typing import Optional

from githand.core.models import (
    BranchInfo,
    DirtyState,
    PatchData,
    RemoteInfo,
    RepoSnapshot,
    RepoStatus,
    SyncStatus,
)
from githand.core.utils import git, git_output, is_git_repo


def collect_status(path: str | Path) -> RepoStatus:
    """Collect lightweight status for a repo (for display)."""
    p = str(path)
    dirty = _collect_dirty(p)
    remotes = _collect_remotes(p)
    branches = _collect_branches(p)
    current_branch = _get_current_branch(p)
    head_commit = git_output(["rev-parse", "--short", "HEAD"], cwd=p, default="")
    commit_msg = git_output(["show-branch", "--no-name", "HEAD"], cwd=p, default="")
    commit_time = git_output(["log", "-1", "--format=%cd", "--date=relative"], cwd=p, default="")

    return RepoStatus(
        dirty=dirty,
        branches=branches,
        current_branch=current_branch or None,
        remotes=remotes,
        head_commit=head_commit or None,
        commit_msg=commit_msg or None,
        commit_time=commit_time or None,
    )


def collect_snapshot(path: str | Path, name: str, data_dir: Optional[Path] = None) -> RepoSnapshot:
    """Collect full snapshot for a repo (for migration).

    Args:
        path: repo path
        name: display name
        data_dir: directory for storing untracked tar files
    """
    p = str(path)
    status = collect_status(path)
    patches = _collect_patches(p, name, data_dir) if status.dirty.is_dirty else PatchData()

    ## read .git/config
    gitconfig = _read_gitconfig(p)

    return RepoSnapshot(
        name=name,
        path=str(Path(path).resolve()),
        remotes=status.remotes,
        branches=status.branches,
        current_branch=status.current_branch,
        head_commit=status.head_commit,
        gitconfig=gitconfig,
        dirty=status.dirty,
        patches=patches,
    )


def _collect_dirty(path: str) -> DirtyState:
    """Collect dirty state flags."""
    has_staged = False
    has_unstaged = False
    has_untracked = False
    has_stash = False
    is_detached = False

    ## staged
    r = git(["diff", "--quiet", "--cached"], cwd=path, check=False)
    if r.returncode == 1:
        has_staged = True

    ## unstaged
    r = git(["diff", "--quiet"], cwd=path, check=False)
    if r.returncode == 1:
        has_unstaged = True

    ## untracked
    out = git_output(["ls-files", "-zo", "--exclude-standard"], cwd=path)
    if out:
        has_untracked = True

    ## stash
    stash_path = Path(path) / ".git" / "logs" / "refs" / "stash"
    has_stash = stash_path.exists()

    ## detached HEAD
    r = git(["symbolic-ref", "-q", "HEAD"], cwd=path, check=False)
    if r.returncode != 0:
        is_detached = True

    return DirtyState(
        has_staged=has_staged,
        has_unstaged=has_unstaged,
        has_untracked=has_untracked,
        has_stash=has_stash,
        is_detached=is_detached,
    )


def _collect_remotes(path: str) -> list[RemoteInfo]:
    """Collect remote info."""
    out = git_output(["remote"], cwd=path)
    if not out:
        return []

    remotes: list[RemoteInfo] = []
    for name in out.splitlines():
        name = name.strip()
        if not name:
            continue
        url = git_output(["remote", "get-url", name], cwd=path)
        push_url = git_output(["remote", "get-url", "--push", name], cwd=path, default="")
        push_url_val = push_url if push_url and push_url != url else None
        remotes.append(RemoteInfo(name=name, url=url, push_url=push_url_val))
    return remotes


def _collect_branches(path: str) -> list[BranchInfo]:
    """Collect local branch info with sync status."""
    out = git_output(["branch", "--format=%(refname:short)|%(HEAD)|%(upstream:short)"], cwd=path)
    if not out:
        return []

    branches: list[BranchInfo] = []
    for line in out.splitlines():
        line = line.strip()
        if not line:
            continue
        parts = line.split("|")
        name = parts[0]
        is_head = parts[1] == "*"
        upstream = parts[2] if len(parts) > 2 and parts[2] else None
        head_commit = git_output(["rev-parse", "--short", name], cwd=path, default="")

        sync_status = None
        if upstream:
            sync_status = _get_sync_status(path, upstream)

        branches.append(BranchInfo(
            name=name,
            is_head=is_head,
            upstream=upstream,
            sync_status=sync_status,
            head_commit=head_commit or None,
        ))
    return branches


def _get_sync_status(path: str, upstream: str) -> SyncStatus:
    """Determine sync status between local branch and upstream."""
    r = git(["rev-list", "--left-right", "--count", f"{upstream}...HEAD"], cwd=path, check=False)
    if r.returncode != 0:
        return SyncStatus.NO_REMOTE

    try:
        left, right = r.stdout.strip().split()
        left_n, right_n = int(left), int(right)
    except (ValueError, IndexError):
        return SyncStatus.NO_REMOTE

    if left_n == 0 and right_n == 0:
        return SyncStatus.IN_SYNC
    if left_n == 0 and right_n > 0:
        return SyncStatus.LOCAL_AHEAD
    if left_n > 0 and right_n == 0:
        return SyncStatus.REMOTE_AHEAD
    return SyncStatus.DIVERGED


def _get_current_branch(path: str) -> str:
    """Get current branch name."""
    return git_output(["symbolic-ref", "--short", "HEAD"], cwd=path, default="")


def _collect_patches(path: str, name: str, data_dir: Optional[Path] = None) -> PatchData:
    """Collect all uncommitted state as patches."""
    staged_patch = None
    unstaged_patch = None
    stash_patches: list[str] = []
    untracked_tar = None
    untracked_manifest: list[str] = []

    ## staged
    out = git_output(["diff", "--cached"], cwd=path)
    if out:
        staged_patch = out

    ## unstaged
    out = git_output(["diff"], cwd=path)
    if out:
        unstaged_patch = out

    ## stash entries
    stash_list = git_output(["stash", "list"], cwd=path)
    if stash_list:
        for entry in stash_list.splitlines():
            ## entry format: stash@{0}: On main: message
            ref = entry.split(":")[0].strip()
            patch = git_output(["stash", "show", "-p", ref], cwd=path)
            if patch:
                stash_patches.append(patch)

    ## untracked files
    untracked_raw = git_output(["ls-files", "--others", "--exclude-standard"], cwd=path)
    if untracked_raw:
        untracked_manifest = [f for f in untracked_raw.splitlines() if f.strip()]
        if untracked_manifest and data_dir is not None:
            untracked_tar = _pack_untracked(path, name, untracked_manifest, data_dir)

    return PatchData(
        staged_patch=staged_patch,
        unstaged_patch=unstaged_patch,
        stash_patches=stash_patches,
        untracked_tar=untracked_tar,
        untracked_manifest=untracked_manifest,
    )


def _pack_untracked(
    repo_path: str,
    repo_name: str,
    files: list[str],
    data_dir: Path,
) -> str:
    """Pack untracked files into a tar.gz under data_dir.

    Returns the relative path inside the data dir (e.g. "untracked/expnix.tar.gz").
    """
    untracked_dir = data_dir / "untracked"
    untracked_dir.mkdir(parents=True, exist_ok=True)
    tar_name = f"{repo_name}.tar.gz"
    tar_path = untracked_dir / tar_name

    repo = Path(repo_path)
    with tarfile.open(str(tar_path), "w:gz") as tar:
        for f in files:
            full = repo / f
            if full.exists():
                tar.add(str(full), arcname=f)

    return f"untracked/{tar_name}"


def _read_gitconfig(path: str) -> Optional[str]:
    """Read .git/config content."""
    config_path = Path(path) / ".git" / "config"
    if config_path.exists():
        try:
            return config_path.read_text(encoding="utf-8")
        except OSError:
            return None
    return None
