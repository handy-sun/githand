"""CLI entry point for githand."""

from __future__ import annotations

import argparse
import sys

from githand import __version__


def main() -> None:
    parser = argparse.ArgumentParser(
        prog="githand",
        description="Git workspace sync & migration tool",
    )
    parser.add_argument("--version", action="version", version=f"githand {__version__}")

    sub = parser.add_subparsers(dest="command", help="available commands")

    ## scan
    p_scan = sub.add_parser("scan", help="scan directory for git repos")
    p_scan.add_argument("path", help="directory to scan")
    p_scan.add_argument("-r", "--recursive", action="store_true", help="scan subdirectories recursively")
    p_scan.add_argument("--auto-group", action="store_true", help="auto-create groups by subdirectory name")

    ## status
    p_status = sub.add_parser("status", help="show repo status")
    p_status.add_argument("--filter", choices=["dirty", "ahead", "stash", "detached"], help="filter repos by status")
    p_status.add_argument("--group", help="only repos in this group")
    p_status.add_argument("--user", help="filter by remote URL owner")
    p_status.add_argument("--json", action="store_true", dest="json", help="machine-readable JSON output")

    ## snapshot
    p_snap = sub.add_parser("snapshot", help="snapshot workspace for migration")
    p_snap.add_argument("-o", "--output", help="output JSON file path")
    p_snap.add_argument("--filter", choices=["dirty", "ahead", "stash", "detached"], help="only snapshot repos matching filter")
    p_snap.add_argument("--group", help="only snapshot repos in this group")

    ## restore
    p_restore = sub.add_parser("restore", help="restore workspace from snapshot")
    p_restore.add_argument("snapshot", help="snapshot JSON file")
    p_restore.add_argument("target_dir", help="target directory to restore into")
    p_restore.add_argument("--dry-run", action="store_true", help="preview without making changes")

    ## ls
    p_ls = sub.add_parser("ls", help="list registered repo names")

    ## rm
    p_rm = sub.add_parser("rm", help="remove repo from registry")
    p_rm.add_argument("name", help="repo name to remove")

    ## group
    p_group = sub.add_parser("group", help="manage groups")
    g_sub = p_group.add_subparsers(dest="group_action", help="group actions")
    g_add = g_sub.add_parser("add", help="add repos to a group")
    g_add.add_argument("group", help="group name")
    g_add.add_argument("repos", nargs="+", help="repo names to add")
    g_rm = g_sub.add_parser("rm", help="remove a group")
    g_rm.add_argument("group", help="group name")
    g_list = g_sub.add_parser("ls", help="list groups")

    args = parser.parse_args()

    if not args.command:
        parser.print_help()
        sys.exit(0)

    dispatch = {
        "scan": _cmd_scan,
        "status": _cmd_status,
        "snapshot": _cmd_snapshot,
        "restore": _cmd_restore,
        "ls": _cmd_ls,
        "rm": _cmd_rm,
        "group": _cmd_group,
    }

    handler = dispatch.get(args.command)
    if handler:
        handler(args)
    else:
        parser.print_help()


def _cmd_scan(args) -> None:
    from githand.commands.scan import run_scan
    run_scan(args)


def _cmd_status(args) -> None:
    from githand.commands.status import run_status
    run_status(args)


def _cmd_snapshot(args) -> None:
    from githand.commands.snapshot import run_snapshot
    run_snapshot(args)


def _cmd_restore(args) -> None:
    from githand.commands.restore import run_restore
    run_restore(args)


def _cmd_ls(args) -> None:
    from githand.config import load_repos
    from githand.display import bold, dim
    repos = load_repos()
    if not repos:
        print("No repos registered.")
        return
    for r in repos:
        group_tag = f" {dim(f'[{r.group}]')}" if r.group else ""
        print(f"  {bold(r.name)}{group_tag}")


def _cmd_rm(args) -> None:
    from githand.config import load_repos, remove_repo, save_repos
    from githand.display import bold
    repos = load_repos()
    before = len(repos)
    repos = remove_repo(args.name, repos)
    after = len(repos)
    if after == before:
        print(f"Repo {bold(args.name)} not found in registry.")
        return
    save_repos(repos)
    print(f"Removed {bold(args.name)} from registry.")


def _cmd_group(args) -> None:
    from githand.config import load_groups, save_groups
    from githand.display import bold, dim

    action = getattr(args, "group_action", None)
    if not action:
        print("Usage: githand group {add|rm|ls}")
        return

    groups = load_groups()

    if action == "add":
        name = args.group
        repos = args.repos
        groups.setdefault(name, [])
        for r in repos:
            if r not in groups[name]:
                groups[name].append(r)
        save_groups(groups)
        print(f"Added {len(repos)} repo(s) to group {bold(name)}")

    elif action == "rm":
        name = args.group
        if name in groups:
            del groups[name]
            save_groups(groups)
            print(f"Removed group {bold(name)}")
        else:
            print(f"Group {bold(name)} not found")

    elif action == "ls":
        if not groups:
            print("No groups defined.")
            return
        for name, members in groups.items():
            print(f"  {bold(name)}: {', '.join(members)}")


if __name__ == "__main__":
    main()
