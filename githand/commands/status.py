"""status command — display repo status with filtering."""

from __future__ import annotations

import json
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

from githand.config import load_groups, load_repos
from githand.core.collect import collect_status
from githand.core.models import RepoRecord, RepoStatus
from githand.core.utils import resolve_path
from githand.display import bold, dim, print_repo_json, print_repo_status


def run_status(args) -> None:
    """Execute the status command."""
    repos = load_repos()
    if not repos:
        print("No repos registered. Run 'githand scan <path>' first.")
        return

    ## apply filters
    filter_type = getattr(args, "filter", None)
    group_name = getattr(args, "group", None)
    user = getattr(args, "user", None)
    json_output = getattr(args, "json", False)

    if group_name:
        groups = load_groups()
        group_members = groups.get(group_name, [])
        repos = [r for r in repos if r.name in group_members or r.group == group_name]

    if user:
        repos = [r for r in repos if _repo_owned_by(r, user)]

    ## collect status in parallel
    results: dict[str, tuple[RepoRecord, RepoStatus | None]] = {}
    with ThreadPoolExecutor(max_workers=8) as pool:
        futures = {
            pool.submit(_safe_collect, r): r for r in repos
        }
        for future in as_completed(futures):
            record = futures[future]
            results[record.name] = (record, future.result())

    ## apply dirty filter after collection
    if filter_type:
        filtered = {}
        for name, (record, status) in results.items():
            if status is None:
                continue
            if _matches_filter(status, filter_type):
                filtered[name] = (record, status)
        results = filtered

    ## display
    if json_output:
        output = []
        for name, (record, status) in sorted(results.items()):
            if status is not None:
                output.append(print_repo_json(name, record.path, status))
        print(json.dumps(output, indent=2, ensure_ascii=False))
    else:
        print(bold(f"Workspace status ({len(results)} repos)"))
        print()
        for name, (record, status) in sorted(results.items()):
            if status is None:
                print(f"  {bold(name)} {dim('ERROR: could not collect status')}")
                continue
            print_repo_status(name, record.path, status, group=record.group)
            print()


def _safe_collect(record):
    """Collect status, return None on error."""
    try:
        return collect_status(record.path)
    except Exception as e:
        return None


def _repo_owned_by(record, user: str) -> bool:
    """Check if any remote URL contains the given username/org."""
    try:
        status = collect_status(record.path)
        for remote in status.remotes:
            if user in remote.url:
                return True
    except Exception:
        pass
    return False


def _matches_filter(status, filter_type: str) -> bool:
    """Check if repo status matches a filter."""
    f = filter_type.lower()
    if f == "dirty":
        return status.dirty.is_dirty
    if f == "ahead":
        return any(b.sync_status and b.sync_status.value == "local_ahead" for b in status.branches)
    if f == "stash":
        return status.dirty.has_stash
    if f == "detached":
        return status.dirty.is_detached
    return True