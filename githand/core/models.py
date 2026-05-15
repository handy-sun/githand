"""Data models for githand."""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from enum import Enum
from typing import Optional


class SyncStatus(Enum):
    """Branch sync status with remote."""

    NO_REMOTE = "no_remote"
    IN_SYNC = "in_sync"
    LOCAL_AHEAD = "local_ahead"
    REMOTE_AHEAD = "remote_ahead"
    DIVERGED = "diverged"


@dataclass
class RemoteInfo:
    """One remote entry."""

    name: str
    url: str
    fetch_refspec: Optional[str] = None
    push_url: Optional[str] = None

    def to_dict(self) -> dict:
        return {k: v for k, v in {
            "name": self.name,
            "url": self.url,
            "fetch_refspec": self.fetch_refspec,
            "push_url": self.push_url,
        }.items() if v is not None}

    @classmethod
    def from_dict(cls, d: dict) -> RemoteInfo:
        return cls(**{k: v for k, v in d.items() if k in cls.__dataclass_fields__})


@dataclass
class BranchInfo:
    """One local branch."""

    name: str
    is_head: bool = False
    upstream: Optional[str] = None
    sync_status: Optional[SyncStatus] = None
    head_commit: Optional[str] = None

    def to_dict(self) -> dict:
        d: dict = {
            "name": self.name,
            "is_head": self.is_head,
        }
        if self.upstream is not None:
            d["upstream"] = self.upstream
        if self.sync_status is not None:
            d["sync_status"] = self.sync_status.value
        if self.head_commit is not None:
            d["head_commit"] = self.head_commit
        return d

    @classmethod
    def from_dict(cls, d: dict) -> BranchInfo:
        if "sync_status" in d and isinstance(d["sync_status"], str):
            d["sync_status"] = SyncStatus(d["sync_status"])
        return cls(**{k: v for k, v in d.items() if k in cls.__dataclass_fields__})


@dataclass
class DirtyState:
    """Working tree dirty flags."""

    has_staged: bool = False
    has_unstaged: bool = False
    has_untracked: bool = False
    has_stash: bool = False
    is_detached: bool = False

    @property
    def is_dirty(self) -> bool:
        return self.has_staged or self.has_unstaged or self.has_untracked or self.has_stash

    def to_dict(self) -> dict:
        return {
            "has_staged": self.has_staged,
            "has_unstaged": self.has_unstaged,
            "has_untracked": self.has_untracked,
            "has_stash": self.has_stash,
            "is_detached": self.is_detached,
        }

    @classmethod
    def from_dict(cls, d: dict) -> DirtyState:
        return cls(**{k: v for k, v in d.items() if k in cls.__dataclass_fields__})


@dataclass
class RepoStatus:
    """Read-only status snapshot for display."""

    dirty: DirtyState = field(default_factory=DirtyState)
    branches: list[BranchInfo] = field(default_factory=list)
    current_branch: Optional[str] = None
    remotes: list[RemoteInfo] = field(default_factory=list)
    head_commit: Optional[str] = None
    commit_msg: Optional[str] = None
    commit_time: Optional[str] = None


@dataclass
class RepoRecord:
    """Lightweight record for workspace registry."""

    name: str
    path: str
    group: Optional[str] = None

    def to_dict(self) -> dict:
        d = {"name": self.name, "path": self.path}
        if self.group is not None:
            d["group"] = self.group
        return d

    @classmethod
    def from_dict(cls, d: dict) -> RepoRecord:
        return cls(**{k: v for k, v in d.items() if k in cls.__dataclass_fields__})


@dataclass
class PatchData:
    """Serialized uncommitted changes."""

    staged_patch: Optional[str] = None
    unstaged_patch: Optional[str] = None
    stash_patches: list[str] = field(default_factory=list)
    untracked_tar: Optional[str] = None
    untracked_manifest: list[str] = field(default_factory=list)

    def to_dict(self) -> dict:
        d: dict = {}
        if self.staged_patch is not None:
            d["staged_patch"] = self.staged_patch
        if self.unstaged_patch is not None:
            d["unstaged_patch"] = self.unstaged_patch
        if self.stash_patches:
            d["stash_patches"] = self.stash_patches
        if self.untracked_tar is not None:
            d["untracked_tar"] = self.untracked_tar
        if self.untracked_manifest:
            d["untracked_manifest"] = self.untracked_manifest
        return d

    @classmethod
    def from_dict(cls, d: dict) -> PatchData:
        return cls(**{k: v for k, v in d.items() if k in cls.__dataclass_fields__})


@dataclass
class RepoSnapshot:
    """Full serializable snapshot of one repo — core of migration."""

    name: str
    path: str
    remotes: list[RemoteInfo]
    branches: list[BranchInfo]
    current_branch: Optional[str]
    head_commit: Optional[str]
    gitconfig: Optional[str] = None
    dirty: DirtyState = field(default_factory=DirtyState)
    patches: PatchData = field(default_factory=PatchData)

    def to_dict(self) -> dict:
        d: dict = {
            "name": self.name,
            "path": self.path,
            "remotes": [r.to_dict() for r in self.remotes],
            "branches": [b.to_dict() for b in self.branches],
            "current_branch": self.current_branch,
            "head_commit": self.head_commit,
            "dirty": self.dirty.to_dict(),
            "patches": self.patches.to_dict(),
        }
        if self.gitconfig is not None:
            d["gitconfig"] = self.gitconfig
        return d

    @classmethod
    def from_dict(cls, d: dict) -> RepoSnapshot:
        d = dict(d)  ## shallow copy
        d["remotes"] = [RemoteInfo.from_dict(r) for r in d.get("remotes", [])]
        d["branches"] = [BranchInfo.from_dict(b) for b in d.get("branches", [])]
        d["dirty"] = DirtyState.from_dict(d.get("dirty", {}))
        d["patches"] = PatchData.from_dict(d.get("patches", {}))
        return cls(**{k: v for k, v in d.items() if k in cls.__dataclass_fields__})


@dataclass
class WorkspaceSnapshot:
    """Top-level container — one file = entire workspace state."""

    version: int = 1
    created_at: str = ""
    hostname: str = ""
    base_path: str = ""
    repos: list[RepoSnapshot] = field(default_factory=list)
    groups: dict[str, list[str]] = field(default_factory=dict)

    def to_dict(self) -> dict:
        return {
            "version": self.version,
            "created_at": self.created_at,
            "hostname": self.hostname,
            "base_path": self.base_path,
            "repos": [r.to_dict() for r in self.repos],
            "groups": self.groups,
        }

    @classmethod
    def from_dict(cls, d: dict) -> WorkspaceSnapshot:
        d = dict(d)
        d["repos"] = [RepoSnapshot.from_dict(r) for r in d.get("repos", [])]
        return cls(**{k: v for k, v in d.items() if k in cls.__dataclass_fields__})

    def to_json(self, indent: int = 2) -> str:
        return json.dumps(self.to_dict(), indent=indent, ensure_ascii=False)

    @classmethod
    def from_json(cls, text: str) -> WorkspaceSnapshot:
        return cls.from_dict(json.loads(text))
