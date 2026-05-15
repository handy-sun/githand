"""Subprocess and path utilities."""

from __future__ import annotations

import os
import subprocess
from pathlib import Path


def git(
    args: list[str],
    cwd: Optional[str | Path] = None,
    check: bool = True,
) -> subprocess.CompletedProcess[str]:
    """Run a git command, return CompletedProcess."""
    return subprocess.run(
        ["git"] + args,
        cwd=cwd,
        capture_output=True,
        text=True,
        check=check,
    )


def git_output(args: list[str], cwd: Optional[str | Path] = None, default: str = "") -> str:
    """Run a git command and return stdout stripped. Returns default on failure."""
    try:
        r = git(args, cwd=cwd, check=True)
        return r.stdout.strip()
    except subprocess.CalledProcessError:
        return default


def is_git_repo(path: Path) -> bool:
    """Check if path is a git repository (has .git dir or is a worktree)."""
    return (path / ".git").exists()


def resolve_path(p: str) -> Path:
    """Expand ~ and make absolute."""
    return Path(os.path.expanduser(p)).resolve()
