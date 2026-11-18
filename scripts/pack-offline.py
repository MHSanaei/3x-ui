#!/usr/bin/env python3
"""Build a 3x-ui replacement ZIP from local files."""

import argparse
import hashlib
import os
from pathlib import Path
import tempfile
import zipfile

FORMAT = "3x-ui-local-package:v1"
GEO_FILES = (
    "geoip.dat", "geosite.dat", "geoip_IR.dat", "geosite_IR.dat",
    "geoip_RU.dat", "geosite_RU.dat",
)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("output", type=Path)
    parser.add_argument("--panel", type=Path)
    parser.add_argument("--xray", type=Path)
    parser.add_argument("--geo", type=Path, action="append", default=[])
    args = parser.parse_args()

    entries = dict(zip(("x-ui", "xray"), (args.panel, args.xray)))
    entries = {name: path for name, path in entries.items() if path is not None}
    for path in args.geo:
        if path.name not in GEO_FILES or path.name in entries:
            parser.error(f"unsupported or duplicate GeoData filename: {path.name}")
        entries[path.name] = path
    if not entries:
        parser.error("supply at least one replacement file")
    for path in entries.values():
        if not path.is_file() or not path.stat().st_size:
            parser.error(f"expected a non-empty regular file: {path}")
        if path.resolve() == args.output.resolve():
            parser.error("output must differ from all inputs")
    if sum(p.stat().st_size for p in entries.values()) + len(FORMAT) + 1 > 512 << 20:
        parser.error("expanded update package exceeds 512 MiB")

    with tempfile.NamedTemporaryFile(dir=args.output.parent, delete=False) as temp:
        tmp_path = Path(temp.name)
    try:
        with zipfile.ZipFile(tmp_path, "w", compression=zipfile.ZIP_DEFLATED) as archive:
            archive.writestr("format", FORMAT + "\n")
            for name, path in entries.items():
                archive.write(path, name)
        if tmp_path.stat().st_size > 256 << 20:
            parser.error("update package exceeds 256 MiB")
        os.replace(tmp_path, args.output)
        with args.output.open("rb") as archive:
            digest = hashlib.file_digest(archive, "sha256").hexdigest()
        print(f"{digest}  {args.output}")
    finally:
        tmp_path.unlink(missing_ok=True)


if __name__ == "__main__":
    main()
