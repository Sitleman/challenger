import argparse
import fcntl
import json
import os
import signal
import shutil
import subprocess
import tarfile
import time
import urllib.error
import urllib.request
from pathlib import Path

from common import changelog, message, read, validate, write


ROOT = Path("/var/lib/challenger")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=["deploy", "notify", "rollback"])
    parser.add_argument("environment", choices=["stage", "production"])
    parser.add_argument("manifest")
    args = parser.parse_args()
    os.umask(0o077)
    signal.signal(signal.SIGHUP, signal.SIG_IGN)
    root = ROOT / args.environment
    root.mkdir(parents=True, exist_ok=True)
    with (root / "deploy.lock").open("w") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        config = read(Path("/etc/challenger") / (args.environment + ".json"))
        if args.action == "rollback":
            rollback(root, config, args.manifest)
            return
        target = validate(read(args.manifest))
        if args.action == "deploy":
            deploy(root, config, target, args.environment)
        else:
            notify(root, config, target)


def run(*args, **kwargs):
    return subprocess.run(args, check=True, text=True, **kwargs)


def compose(config, manifest, *args):
    env = dict(os.environ, IMAGE_REF=manifest["image"], APP_VERSION=manifest["version"],
               BOT_ENV_FILE=config["env_file"], DATA_VOLUME=config["volume"],
               HEALTHCHECK_DISABLED="true" if manifest["version"] == "v0" else "false")
    return run("docker", "compose", "-p", config["project"], "-f", str(Path(__file__).parents[2] / "deploy/compose.yml"),
               *args, env=env, capture_output=True).stdout.strip()


def state(root):
    path = root / "current.json"
    if not path.exists():
        raise ValueError("environment must be bootstrapped: current.json is missing")
    return read(path)


def deploy(root, config, target, environment):
    previous = state(root)
    if (root / "pending.json").exists():
        raise ValueError("unfinished deployment: inspect pending.json and recover before retrying")
    if previous["version"] == target["version"]:
        if previous["image"] != target["image"]:
            raise ValueError("version already deployed with another image")
        healthy(config, previous)
        print("already deployed")
        return
    if int(previous["version"][1:]) >= int(target["version"][1:]):
        raise ValueError("stale deployment; use explicit rollback")
    if environment == "production" and target.get("base") != previous["version"]:
        raise ValueError("production changed since preparation; prepare and approve again")
    notes = target.get("notes") if environment == "production" else changelog(previous, target)
    message(target["version"], notes)
    target = dict(target, notes=notes)
    validate_config(config)
    compose(config, target, "pull", "bot")
    backup = root / "backups" / (time.strftime("%Y%m%dT%H%M%SZ", time.gmtime()) + "-" + target["version"])
    backup.mkdir(parents=True)
    write(backup / "previous.json", previous)
    write(backup / "target.json", target)
    was_running = bool(compose(config, previous, "ps", "-q", "bot"))
    write(root / "pending.json", {"backup": str(backup), "target": target, "phase": "stopping"})
    try:
        compose(config, previous, "stop", "bot")
        volume = volume_path(config)
        with tarfile.open(backup / "data.tgz", "w:gz") as archive:
            archive.add(volume, arcname=".")
        with tarfile.open(backup / "data.tgz", "r:gz") as archive:
            archive.getmembers()
    except Exception:
        if was_running:
            compose(config, previous, "up", "-d", "--no-build", "--pull", "never", "bot")
            healthy(config, previous)
        (root / "pending.json").unlink()
        raise
    write(root / "pending.json", {"backup": str(backup), "target": target, "phase": "starting"})
    try:
        compose(config, target, "up", "-d", "--no-build", "--pull", "never", "bot")
        healthy(config, target)
    except Exception:
        compose(config, target, "stop", "bot")
        raise RuntimeError(f"new bot stopped; restore explicitly from {backup}") from None
    write(root / "current.json", target)
    (root / "pending.json").unlink()
    print("deployed " + target["version"])


def validate_config(config):
    values = read_env(config["env_file"])
    chats = [x.strip() for x in values["ALLOWED_CHAT_IDS"].split(",") if x.strip()]
    if chats != [str(config["chat_id"])]:
        raise ValueError("ALLOWED_CHAT_IDS must contain only this environment's release chat")
    if not values.get("BOT_TOKEN"):
        raise ValueError("BOT_TOKEN is missing")


def read_env(path):
    values = {}
    for line in Path(path).read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        key, value = line.split("=", 1)
        values[key.strip()] = value.strip().strip("\"'")
    return values


def volume_path(config):
    return Path(run("docker", "volume", "inspect", config["volume"], "--format", "{{.Mountpoint}}",
                    capture_output=True).stdout.strip())


def healthy(config, target):
    deadline = time.monotonic() + 120
    while time.monotonic() < deadline:
        container = compose(config, target, "ps", "-q", "bot")
        if container:
            info = json.loads(run("docker", "inspect", container, capture_output=True).stdout)[0]
            if info["RestartCount"]:
                raise RuntimeError("bot restarted during startup")
            if (target["version"] == "v0" and info["State"].get("Running")) or info["State"].get("Health", {}).get("Status") == "healthy":
                return
        time.sleep(3)
    raise RuntimeError("bot did not become ready within 120 seconds")


def notify(root, config, requested):
    current = state(root)
    if current["version"] != requested["version"] or current["image"] != requested["image"]:
        raise ValueError("notification target is no longer deployed")
    healthy(config, current)
    validate_config(config)
    text = message(current["version"], current["notes"])
    path = root / "notifications"
    path.mkdir(exist_ok=True)
    receipt = path / f'{current["version"]}-{config["chat_id"]}.json'
    if receipt.exists():
        record = read(receipt)
        if record["status"] == "sent":
            print("already announced")
            return
        if record["status"] == "sending":
            raise ValueError("ambiguous previous send: reconcile Telegram message before retrying")
    write(receipt, {"status": "sending", "text": text})
    body = {"chat_id": config["chat_id"], "text": text,
            "link_preview_options": {"is_disabled": True}}
    if config.get("thread_id"):
        body["message_thread_id"] = config["thread_id"]
    token = read_env(config["env_file"])["BOT_TOKEN"]
    request = urllib.request.Request("https://api.telegram.org/bot" + token + "/sendMessage",
                                     data=json.dumps(body).encode(), headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            result = json.load(response)
    except urllib.error.HTTPError as error:
        try:
            with error:
                result = json.loads(error.read())
        except (ValueError, OSError):
            raise RuntimeError("ambiguous Telegram response; inspect notification receipt") from None
        if result.get("ok") is not False:
            raise RuntimeError("ambiguous Telegram response; inspect notification receipt") from None
    except Exception:
        raise RuntimeError("ambiguous Telegram send; inspect notification receipt") from None
    if not result.get("ok"):
        write(receipt, {"status": "rejected", "error_code": result.get("error_code")})
        raise RuntimeError("Telegram rejected the announcement; deployment remains successful")
    write(receipt, {"status": "sent", "message_id": result["result"]["message_id"], "text": text})
    print("announced " + current["version"])


def rollback(root, config, backup_name):
    if Path(backup_name).name != backup_name:
        raise ValueError("expected a backup directory name")
    backup = root / "backups" / backup_name
    previous = read(backup / "previous.json")
    target = read(backup / "target.json")
    run("docker", "image", "inspect", previous["image"], capture_output=True)
    with tarfile.open(backup / "data.tgz", "r:gz") as archive:
        archive.getmembers()
    write(root / "pending.json", {"backup": str(backup), "target": previous, "phase": "rollback"})
    compose(config, target, "stop", "bot")
    volume = volume_path(config)
    aside = root / ("before-restore-" + str(time.time_ns()))
    aside.mkdir()
    for child in volume.iterdir():
        shutil.move(str(child), str(aside / child.name))
    with tarfile.open(backup / "data.tgz", "r:gz") as archive:
        archive.extractall(volume, filter="data")
    for child in [volume, *volume.rglob("*")]:
        os.chown(child, 65532, 65532, follow_symlinks=False)
    compose(config, previous, "up", "-d", "--no-build", "--pull", "never", "bot")
    healthy(config, previous)
    write(root / "current.json", previous)
    (root / "pending.json").unlink(missing_ok=True)
    print("restored " + previous["version"] + "; synchronize GitHub production marker before next release")


if __name__ == "__main__":
    try:
        main()
    except subprocess.CalledProcessError:
        raise SystemExit("Docker command failed; inspect the server (raw output withheld)") from None
    except (ValueError, RuntimeError, KeyError) as error:
        raise SystemExit(str(error)) from None
