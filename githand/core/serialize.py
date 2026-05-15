"""JSON serialization / deserialization for workspace snapshots."""

from __future__ import annotations

import json
from pathlib import Path

from githand.core.models import WorkspaceSnapshot


def save_snapshot(
    snapshot: WorkspaceSnapshot,
    json_path: Path,
) -> tuple[Path, Path | None]:
    """Write snapshot to JSON file. If there's a data dir with untracked tar files,
    return (json_path, data_dir_path). Otherwise (json_path, None).

    The data dir sits next to the JSON file: <stem>-data/
    """
    json_path = json_path.resolve()
    json_path.parent.mkdir(parents=True, exist_ok=True)

    ## check if there's a data dir to bundle
    data_dir = json_path.parent / (json_path.stem + "-data")
    has_data = data_dir.exists() and any(data_dir.iterdir())

    json_path.write_text(snapshot.to_json(), encoding="utf-8")

    return json_path, data_dir if has_data else None


def load_snapshot(json_path: Path) -> WorkspaceSnapshot:
    """Load snapshot from JSON file."""
    text = json_path.read_text(encoding="utf-8")
    return WorkspaceSnapshot.from_json(text)


def pack_snapshot(snapshot_json: Path, data_dir: Path | None, output: Path) -> Path:
    """Pack JSON + data dir into a single tar.gz for easy transfer."""
    import tarfile

    output = output.resolve()
    with tarfile.open(str(output), "w:gz") as tar:
        tar.add(str(snapshot_json), arcname=snapshot_json.name)
        if data_dir and data_dir.exists():
            tar.add(str(data_dir), arcname=data_dir.name)

    return output


def unpack_snapshot(archive: Path, target_dir: Path) -> Path:
    """Unpack a tar.gz archive, return the JSON file path."""
    import tarfile

    target_dir.mkdir(parents=True, exist_ok=True)
    with tarfile.open(str(archive), "r:gz") as tar:
        tar.extractall(str(target_dir), filter="data")

    ## find the json file
    for f in target_dir.iterdir():
        if f.suffix == ".json":
            return f

    raise FileNotFoundError(f"No JSON file found in archive {archive}")
