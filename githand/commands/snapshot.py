"""snapshot command — serialize workspace state for migration."""

from __future__ import annotations

import platform
from datetime import datetime, timezone
from pathlib import Path

from githand.config import load_groups, load_repos
from githand.core.collect import collect_snapshot
from githand.core.models import WorkspaceSnapshot
from githand.core.serialize import save_snapshot
from githand.core.utils import resolve_path
from githand.display import bold, dim, green, yellow


def run_snapshot(args) -> None:
    """Execute the snapshot command."""
    repos = load_repos()
    if not repos:
        print("No repos registered. Run 'githand scan <path>' first.")
        return

    filter_type = getattr(args, "filter", None)
    group_name = getattr(args, "group", None)
    output_path = getattr(args, "output", None)

    if group_name:
        groups = load_groups()
        group_members = groups.get(group_name, [])
        repos = [r for r in repos if r.name in group_members or r.group == group_name]

    if filter_type:
        from githand.commands.status import _matches_filter, _safe_collect
        filtered = []
        for r in repos:
            status = _safe_collect(r)
            if status and _matches_filter(status, filter_type):
                filtered.append(r)
        repos = filtered

    if not repos:
        print("No repos match the given filters.")
        return

    ## determine output paths
    if output_path:
        json_path = Path(output_path).resolve()
    else:
        timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
        json_path = Path.cwd() / f"workspace-snapshot-{timestamp}.json"

    data_dir = json_path.parent / (json_path.stem + "-data")
    data_dir.mkdir(parents=True, exist_ok=True)

    print(bold(f"Snapshotting {len(repos)} repos..."))
    print()

    snapshots = []
    for r in repos:
        try:
            snap = collect_snapshot(r.path, r.name, data_dir=data_dir)
            snapshots.append(snap)
            dirty_flag = yellow("dirty") if snap.dirty.is_dirty else green("clean")
            print(f"  {bold(r.name)} {dirty_flag}")
        except Exception as e:
            print(f"  {bold(r.name)} ERROR: {e}")

    groups = load_groups()
    workspace = WorkspaceSnapshot(
        version=1,
        created_at=datetime.now(timezone.utc).isoformat(),
        hostname=platform.node(),
        base_path=str(Path.home()),
        repos=snapshots,
        groups=groups,
    )

    json_path, data_dir_res = save_snapshot(workspace, json_path)

    print()
    print(f"Snapshot saved to {bold(str(json_path))}")
    if data_dir_res:
        print(f"Untracked data in {dim(str(data_dir_res))}")
    print(f"Repos: {len(snapshots)}, "
          f"dirty: {sum(1 for s in snapshots if s.dirty.is_dirty)}")
