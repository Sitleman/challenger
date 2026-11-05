import json
import os
import signal
import subprocess
import sys
import tempfile
from pathlib import Path

from common import validate, write


def main():
    environment = sys.argv[1]
    if environment not in ("stage", "production"):
        raise ValueError("invalid forced environment")
    command = os.environ.get("SSH_ORIGINAL_COMMAND", "")
    allowed = {f"deploy {environment}": "deploy", f"notify {environment}": "notify"}
    if command not in allowed:
        raise ValueError("SSH key is restricted to its deployment environment")
    signal.signal(signal.SIGHUP, signal.SIG_IGN)
    payload = sys.stdin.buffer.read(1024 * 1024 + 1)
    if len(payload) > 1024 * 1024:
        raise ValueError("manifest too large")
    manifest = validate(json.loads(payload))
    if not manifest["image"].startswith("ghcr.io/sitleman/challenger@sha256:"):
        raise ValueError("unexpected image repository")
    with tempfile.TemporaryDirectory(prefix="challenger-manifest-") as directory:
        path = Path(directory) / "release.json"
        write(path, manifest)
        subprocess.run([sys.executable, str(Path(__file__).with_name("server.py")),
                        allowed[command], environment, str(path)], check=True)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError) as error:
        raise SystemExit(str(error)) from None
