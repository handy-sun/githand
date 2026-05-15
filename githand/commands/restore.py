"""restore command — reproduce workspace from a snapshot."""

from __future__ import annotations

from pathlib import Path

from githand.core.restore_ops import restore_repo
from githand.core.serialize import load_snapshot
from githand.core.utils import resolve_path
from githand.display import bold, dim, green, yellow


def run_restore(args) -> None:
    """Execute the restore command."""
    snapshot_path = Path(args.snapshot)
    target_dir = resolve_path(args.target_dir)
    dry_run = getattr(args, "dry_run", False)
    base_path_override = getattr(args, "base_path", None)

    if not snapshot_path.exists():
        print(f"ERROR: snapshot file not found: {snapshot_path}")
        return

    workspace = load_snapshot(snapshot_path)

    ## find data dir (sibling to json)
    data_dir = snapshot_path.parent / (snapshot_path.stem + "-data")
    if not data_dir.exists():
        data_dir = None

    ## determine restore base: if --base-path given, use it as the new base;
    ## otherwise target_dir serves as the base
    restore_base = resolve_path(base_path_override) if base_path_override else target_dir

    print(bold(f"Restoring {len(workspace.repos)} repos"))
    if dry_run:
        print(yellow("(dry run)"))
    print(f"Source: {workspace.hostname} @ {workspace.created_at}")
    print(f"Original base: {workspace.base_path}")
    print(f"Restore base:  {restore_base}")
    print()

    restore_base.mkdir(parents=True, exist_ok=True)

    for snap in workspace.repos:
        restore_repo(snap, restore_base, data_dir=data_dir, dry_run=dry_run)

    ## restore groups
    if workspace.groups and not dry_run:
        from githand.config import save_groups
        save_groups(workspace.groups)
        print(f"\nRestored {len(workspace.groups)} group(s)")

    print()
    print(green("Done."))
