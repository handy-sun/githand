"""Restore operations — reproduce repo state from a snapshot."""

from __future__ import annotations

import tarfile
from pathlib import Path

from githand.core.models import RepoSnapshot
from githand.core.utils import git, git_output


def restore_repo(
    snapshot: RepoSnapshot,
    target_dir: Path,
    data_dir: Path | None = None,
    dry_run: bool = False,
) -> None:
    """Restore a single repo from its snapshot.

    Args:
        snapshot: the repo snapshot
        target_dir: parent directory to clone into
        data_dir: directory containing untracked tar files
        dry_run: only print what would happen
    """
    repo_path = target_dir / snapshot.name

    if not snapshot.remotes:
        print(f"  SKIP {snapshot.name}: no remotes configured")
        return

    primary_remote = snapshot.remotes[0]

    ## 1. clone
    if dry_run:
        print(f"  CLONE {primary_remote.url} -> {repo_path}")
    else:
        if repo_path.exists():
            print(f"  SKIP {snapshot.name}: directory already exists")
            return
        r = git(["clone", primary_remote.url, str(repo_path)], check=False)
        if r.returncode != 0:
            print(f"  ERROR cloning {snapshot.name}: {r.stderr.strip()}")
            return

    ## remaining steps skip on dry_run
    if dry_run:
        if snapshot.current_branch:
            print(f"  CHECKOUT {snapshot.current_branch}")
        for branch in snapshot.branches:
            if not branch.is_head and branch.upstream:
                print(f"  TRACK {branch.name} -> {branch.upstream}")
        if snapshot.patches.staged_patch:
            print(f"  APPLY --cached staged patch ({len(snapshot.patches.staged_patch)} chars)")
        if snapshot.patches.unstaged_patch:
            print(f"  APPLY unstaged patch ({len(snapshot.patches.unstaged_patch)} chars)")
        if snapshot.patches.stash_patches:
            print(f"  APPLY {len(snapshot.patches.stash_patches)} stash patch(es)")
        if snapshot.patches.untracked_manifest:
            print(f"  EXTRACT {len(snapshot.patches.untracked_manifest)} untracked file(s)")
        return

    ## 2. add additional remotes
    for remote in snapshot.remotes[1:]:
        git(["remote", "add", remote.name, remote.url], cwd=str(repo_path), check=False)
        if remote.push_url:
            git(["remote", "set-url", "--push", remote.name, remote.push_url], cwd=str(repo_path), check=False)

    ## 3. checkout branch
    if snapshot.current_branch:
        git(["checkout", snapshot.current_branch], cwd=str(repo_path), check=False)

    ## 4. track non-HEAD branches
    for branch in snapshot.branches:
        if not branch.is_head and branch.upstream:
            git(["branch", "--track", branch.name, branch.upstream], cwd=str(repo_path), check=False)

    ## 5. restore .git/config (merge)
    if snapshot.gitconfig:
        _merge_gitconfig(repo_path, snapshot.gitconfig)

    ## 6. apply staged patch
    if snapshot.patches.staged_patch:
        _apply_patch(snapshot.patches.staged_patch, str(repo_path), cached=True)

    ## 7. apply unstaged patch
    if snapshot.patches.unstaged_patch:
        _apply_patch(snapshot.patches.unstaged_patch, str(repo_path), cached=False)

    ## 8. apply stash patches
    for patch in snapshot.patches.stash_patches:
        _apply_patch(patch, str(repo_path), cached=False)
        git(["stash"], cwd=str(repo_path), check=False)

    ## 9. extract untracked files
    if snapshot.patches.untracked_tar and data_dir:
        tar_path = data_dir / snapshot.patches.untracked_tar
        if tar_path.exists():
            with tarfile.open(str(tar_path), "r:gz") as tar:
                tar.extractall(str(repo_path), filter="data")

    print(f"  OK {snapshot.name}")


def _apply_patch(patch: str, cwd: str, cached: bool = False) -> bool:
    """Apply a git patch. Returns True on success."""
    import subprocess

    args = ["git", "apply"]
    if cached:
        args.append("--cached")
    r = subprocess.run(
        args,
        input=patch,
        cwd=cwd,
        capture_output=True,
        text=True,
    )
    if r.returncode != 0:
        print(f"    WARN: patch apply failed: {r.stderr.strip()}")
        return False
    return True


def _merge_gitconfig(repo_path: Path, config_text: str) -> None:
    """Append non-duplicate config sections to .git/config.

    Handles subsections like [remote "origin"] — matches the full
    section header line, not just the bare section name.
    """
    config_path = repo_path / ".git" / "config"
    if not config_path.exists():
        config_path.write_text(config_text, encoding="utf-8")
        return

    existing = config_path.read_text(encoding="utf-8")
    ## collect all section headers from existing config
    existing_headers: set[str] = set()
    for line in existing.splitlines():
        stripped = line.strip()
        if stripped.startswith("["):
            existing_headers.add(stripped)

    ## append sections whose header is not already present
    new_lines: list[str] = []
    skip = False
    for line in config_text.splitlines():
        stripped = line.strip()
        if stripped.startswith("["):
            if stripped in existing_headers:
                skip = True
            else:
                skip = False
                new_lines.append(line)
        elif not skip:
            new_lines.append(line)

    if new_lines:
        config_path.write_text(existing + "\n" + "\n".join(new_lines), encoding="utf-8")
