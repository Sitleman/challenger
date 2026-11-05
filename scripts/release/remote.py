import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

from common import read, validate


def main():
    action, environment = sys.argv[1:]
    if action not in ("deploy", "notify") or environment not in ("stage", "production"):
        raise ValueError("invalid action/environment")
    host = os.environ["DEPLOY_HOST"]
    user = os.environ["DEPLOY_USER"]
    if not re.fullmatch(r"[a-zA-Z0-9.-]+", host) or not re.fullmatch(r"[a-z_][a-z0-9_-]*", user):
        raise ValueError("invalid SSH host/user")
    if not os.environ.get("DEPLOY_SSH_KEY") or not os.environ.get("DEPLOY_KNOWN_HOSTS"):
        raise ValueError("SSH credentials and pinned host keys are required")
    validate(read("release.json"))
    with tempfile.TemporaryDirectory() as directory:
        key = Path(directory) / "key"
        key.write_text(os.environ["DEPLOY_SSH_KEY"] + "\n")
        key.chmod(0o600)
        known = Path(directory) / "known_hosts"
        known.write_text(os.environ["DEPLOY_KNOWN_HOSTS"] + "\n")
        options = ["-i", str(key), "-o", "IdentitiesOnly=yes", "-o", "BatchMode=yes", "-o",
                   "StrictHostKeyChecking=yes", "-o", f"UserKnownHostsFile={known}", "-o", "ConnectTimeout=15"]
        subprocess.run(["ssh", *options, f"{user}@{host}", action + " " + environment],
                       input=Path("release.json").read_text(), text=True, check=True)


if __name__ == "__main__":
    main()
