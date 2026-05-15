"""scan command — discover and register git repos."""

from __future__ import annotations

from pathlib import Path

from githand.config import add_repos, load_repos, save_repos
from githand.core.discover import scan_directory
from githand.core.utils import resolve_path
from githand.display import bold, dim, green


def run_scan(args) -> None:
    """Execute the scan command."""
    base = resolve_path(args.path)
    if not base.is_dir():
        print(f"ERROR: {base} is not a directory")
        return

    recursive = getattr(args, "recursive", False)
    auto_group = getattr(args, "auto_group", False)

    discovered = scan_directory(base, recursive=recursive, auto_group=auto_group)
    if not discovered:
        print(f"No git repos found under {base}")
        return

    existing_base, existing = load_repos()
    merged = add_repos(discovered, existing)
    new_count = len(merged) - len(existing)
    existing_in_scan = len(discovered) - new_count

    ## set base_path only on first scan; preserve existing
    final_base = existing_base or str(base)
    save_repos(merged, base_path=final_base)

    print(f"Scanned {bold(str(base))}")
    print(f"Found {bold(str(len(discovered)))} repos, {green(str(new_count))} new, {dim(str(existing_in_scan))} already registered")
    print()

    for r in merged:
        group_tag = f" {dim(f'[{r.group}]')}" if r.group else ""
        print(f"  {bold(r.name)}{group_tag}  {dim(r.path)}")