"""Configuration — XDG paths, registry persistence."""

from __future__ import annotations

import json
import os
from pathlib import Path

from githand.core.models import RepoRecord


def config_dir() -> Path:
    """Get githand config directory ($XDG_CONFIG_HOME/githand)."""
    xdg = os.environ.get("XDG_CONFIG_HOME")
    base = Path(xdg) if xdg else Path.home() / ".config"
    return base / "githand"


def registry_path() -> Path:
    """Path to the repos registry file."""
    return config_dir() / "repos.json"


def groups_path() -> Path:
    """Path to the groups file."""
    return config_dir() / "groups.json"


def load_repos() -> list[RepoRecord]:
    """Load registered repos from disk."""
    p = registry_path()
    if not p.exists():
        return []
    try:
        data = json.loads(p.read_text(encoding="utf-8"))
        return [RepoRecord.from_dict(d) for d in data]
    except (json.JSONDecodeError, KeyError):
        return []


def save_repos(repos: list[RepoRecord]) -> None:
    """Save repo registry to disk."""
    p = registry_path()
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(
        json.dumps([r.to_dict() for r in repos], indent=2, ensure_ascii=False),
        encoding="utf-8",
    )


def add_repos(new: list[RepoRecord], existing: list[RepoRecord]) -> list[RepoRecord]:
    """Merge new repos into existing, skipping duplicates by path."""
    seen = {r.path for r in existing}
    added: list[RepoRecord] = []
    for r in new:
        if r.path not in seen:
            seen.add(r.path)
            added.append(r)
    return existing + added


def remove_repo(name: str, repos: list[RepoRecord]) -> list[RepoRecord]:
    """Remove a repo by name. Returns updated list."""
    return [r for r in repos if r.name != name]


def load_groups() -> dict[str, list[str]]:
    """Load group definitions from disk."""
    p = groups_path()
    if not p.exists():
        return {}
    try:
        return json.loads(p.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, KeyError):
        return {}


def save_groups(groups: dict[str, list[str]]) -> None:
    """Save group definitions to disk."""
    p = groups_path()
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(json.dumps(groups, indent=2, ensure_ascii=False), encoding="utf-8")
