"""Directory scanning — discover git repos under a path."""

from __future__ import annotations

from pathlib import Path

from githand.core.models import RepoRecord
from githand.core.utils import is_git_repo


def scan_directory(
    base_path: Path,
    recursive: bool = False,
    auto_group: bool = False,
) -> list[RepoRecord]:
    """Scan a directory for git repositories.

    Args:
        base_path: root directory to scan
        recursive: scan subdirectories recursively
        auto_group: use subdirectory name as group tag

    Returns:
        list of RepoRecord for discovered repos
    """
    records: list[RepoRecord] = []
    seen: set[str] = set()

    _walk(base_path, base_path, records, seen, recursive=recursive, auto_group=auto_group)
    return records


def _walk(
    current: Path,
    base: Path,
    records: list[RepoRecord],
    seen: set[str],
    *,
    recursive: bool,
    auto_group: bool,
) -> None:
    """Recursively walk directory tree for git repos."""
    if not current.is_dir():
        return

    try:
        entries = sorted(current.iterdir())
    except PermissionError:
        return

    for entry in entries:
        if not entry.is_dir():
            continue

        ## skip hidden dirs (but not .git itself — we check parent)
        if entry.name.startswith("."):
            continue

        if is_git_repo(entry):
            resolved = str(entry.resolve())
            if resolved not in seen:
                seen.add(resolved)
                group = None
                if auto_group and entry.parent != base:
                    group = entry.parent.name
                records.append(RepoRecord(
                    name=entry.name,
                    path=resolved,
                    group=group,
                ))
            ## don't recurse into a git repo
            continue

        if recursive:
            _walk(entry, base, records, seen, recursive=recursive, auto_group=auto_group)
