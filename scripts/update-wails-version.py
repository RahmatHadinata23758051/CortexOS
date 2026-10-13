#!/usr/bin/env python3
"""
Update wails.json productVersion from VERSION file.
Run this before wails build to inject the application version.
"""
import json
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
VERSION_FILE = REPO_ROOT / "VERSION"
WAILS_FILE = REPO_ROOT / "wails.json"


def main() -> int:
    if not VERSION_FILE.exists():
        print(f"ERROR: VERSION file not found at {VERSION_FILE}", file=sys.stderr)
        return 1

    version = VERSION_FILE.read_text().strip()
    if not version:
        print("ERROR: VERSION file is empty", file=sys.stderr)
        return 1

    # Strip leading 'v' if present for productVersion field
    if version.startswith("v"):
        product_version = version[1:]
    else:
        product_version = version

    if not WAILS_FILE.exists():
        print(f"ERROR: wails.json not found at {WAILS_FILE}", file=sys.stderr)
        return 1

    with WAILS_FILE.open("r", encoding="utf-8") as f:
        wails_data = json.load(f)

    # Ensure info object exists
    if "info" not in wails_data:
        wails_data["info"] = {}

    current = wails_data["info"].get("productVersion")
    if current == product_version:
        print(f"wails.json productVersion already {product_version}")
        return 0

    wails_data["info"]["productVersion"] = product_version

    with WAILS_FILE.open("w", encoding="utf-8") as f:
        json.dump(wails_data, f, indent=2)
        f.write("\n")

    print(f"Updated wails.json productVersion to {product_version} (from VERSION={version})")
    return 0


if __name__ == "__main__":
    sys.exit(main())