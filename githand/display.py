"""Terminal output formatting — colors, tables."""

from __future__ import annotations

import sys

from githand.core.models import DirtyState, RepoStatus, SyncStatus


## ANSI colors
class C:
    RESET = "\033[0m"
    BOLD = "\033[1m"
    DIM = "\033[2m"
    RED = "\033[31m"
    GREEN = "\033[32m"
    YELLOW = "\033[33m"
    BLUE = "\033[34m"
    MAGENTA = "\033[35m"
    CYAN = "\033[36m"


def _supports_color() -> bool:
    return hasattr(sys.stdout, "isatty") and sys.stdout.isatty()


def c(text: str, code: str) -> str:
    """Wrap text in ANSI color if terminal supports it."""
    if not _supports_color():
        return text
    return f"{code}{text}{C.RESET}"


def bold(text: str) -> str:
    return c(text, C.BOLD)


def dim(text: str) -> str:
    return c(text, C.DIM)


def red(text: str) -> str:
    return c(text, C.RED)


def green(text: str) -> str:
    return c(text, C.GREEN)


def yellow(text: str) -> str:
    return c(text, C.YELLOW)


def blue(text: str) -> str:
    return c(text, C.BLUE)


def magenta(text: str) -> str:
    return c(text, C.MAGENTA)


def cyan(text: str) -> str:
    return c(text, C.CYAN)


def format_dirty(dirty: DirtyState) -> str:
    """Format dirty state as compact flags."""
    parts: list[str] = []
    if dirty.has_staged:
        parts.append(green("+"))
    if dirty.has_unstaged:
        parts.append(red("!"))
    if dirty.has_untracked:
        parts.append(yellow("?"))
    if dirty.has_stash:
        parts.append(magenta("$"))
    if dirty.is_detached:
        parts.append(blue("D"))
    return "".join(parts) if parts else green("clean")


def format_sync(status: Optional[SyncStatus]) -> str:
    """Format sync status."""
    if status is None:
        return dim("-")
    mapping = {
        SyncStatus.IN_SYNC: green("="),
        SyncStatus.LOCAL_AHEAD: yellow("↑"),
        SyncStatus.REMOTE_AHEAD: red("↓"),
        SyncStatus.DIVERGED: red("↕"),
        SyncStatus.NO_REMOTE: dim("-"),
    }
    return mapping.get(status, dim("?"))


def format_branch_line(
    name: str,
    is_head: bool,
    upstream: Optional[str],
    sync_status: Optional[SyncStatus],
) -> str:
    """Format one branch line for status display."""
    prefix = green("* ") if is_head else "  "
    branch = bold(name) if is_head else name
    if upstream:
        sync = format_sync(sync_status)
        return f"{prefix}{branch} {dim('->')} {upstream} {sync}"
    return f"{prefix}{branch}"


def print_repo_status(name: str, path: str, status: RepoStatus, group: Optional[str] = None) -> None:
    """Print one repo's status in the table."""
    dirty_str = format_dirty(status.dirty)
    branch_str = status.current_branch or dim("(detached)")

    header = bold(name)
    if group:
        header = f"{header} {dim(f'[{group}]')}"

    commit_info = ""
    if status.head_commit:
        commit_info = dim(status.head_commit)
        if status.commit_time:
            commit_info = f"{commit_info} {dim(status.commit_time)}"

    print(f"  {header}")
    print(f"    branch:  {branch_str}  {dirty_str}  {commit_info}")

    if status.remotes:
        for r in status.remotes:
            url = r.url
            print(f"    remote:  {r.name} {dim(url)}")


def print_repo_json(name: str, path: str, status: RepoStatus) -> dict:
    """Return a dict for JSON output mode."""
    return {
        "name": name,
        "path": path,
        "branch": status.current_branch,
        "head": status.head_commit,
        "dirty": status.dirty.to_dict(),
        "remotes": [r.to_dict() for r in status.remotes],
    }
