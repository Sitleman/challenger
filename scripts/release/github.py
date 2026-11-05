import argparse
import json
import os
import re
import subprocess
import tempfile
from pathlib import Path

from common import VERSION, changelog, message, read, validate, validate_entries, write


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=["validate-changes", "register", "stage-success", "prepare", "freeze", "production-success"])
    parser.add_argument("--version")
    parser.add_argument("--image")
    parser.add_argument("--base")
    args = parser.parse_args()
    if args.action == "validate-changes":
        validate_changes(args.base)
        return
    if args.action == "register":
        register(args.image)
        return
    if args.action == "stage-success":
        manifest = validate(read("release.json"))
        upload(manifest["version"], "stage.json", manifest)
        return
    if args.action == "production-success":
        manifest = validate(read("release.json"))
        upload(manifest["version"], "production.json", manifest)
        mark_production(manifest)
        release = release_for(manifest["version"])
        api(f'releases/{release["id"]}', "PATCH", {"draft": False, "prerelease": False,
                                                 "body": manifest["notes"], "name": manifest["version"]})
        return
    version = args.version
    if not version or not VERSION.fullmatch(version):
        raise ValueError("version must be v1, v2, ...")
    release = release_for(version)
    target = validate(asset(release, "stage.json"))
    if target["version"] != version:
        raise ValueError("stage proof/version mismatch")
    if args.action == "prepare":
        previous = production()
        if int(previous["version"][1:]) >= int(version[1:]):
            raise ValueError("choose a version newer than production")
        git("merge-base", "--is-ancestor", previous["commit"], target["commit"])
        notes = changelog(previous, target)
        message(version, notes)
        candidate = dict(target, base=previous["version"], notes=notes)
        upload(version, "candidate.json", candidate, replace=True)
        api(f'releases/{release["id"]}', "PATCH", {"body": notes})
        summary(f'Черновик {version}\n\n{release["html_url"]}\n\n{notes}\n\nОтредактируйте текст выпуска и запустите Promote production.')
    elif args.action == "freeze":
        require_approval_gate()
        candidate = asset(release, "candidate.json")
        if any(candidate[key] != target[key] for key in ("version", "image", "commit", "entries")):
            raise ValueError("candidate differs from successful stage deployment")
        if candidate["base"] != production()["version"]:
            raise ValueError("production changed; prepare again")
        candidate["notes"] = release["body"]
        text = message(version, candidate["notes"])
        write("release.json", candidate)
        summary(f'К утверждению: {candidate["base"]} → {version}\n\nCommit: {candidate["commit"]}\n\nОбраз: {candidate["image"]}\n\n{text}')


def require_approval_gate():
    environment = api("environments/production")
    reviewers = [rule for rule in environment.get("protection_rules", []) if rule["type"] == "required_reviewers"]
    if len(reviewers) != 1 or reviewers[0].get("prevent_self_review"):
        raise ValueError("configure production required reviewer; allow owner self-approval")
    allowed = reviewers[0].get("reviewers", [])
    if len(allowed) != 1 or allowed[0]["type"] != "User" or allowed[0]["reviewer"]["login"] != "Sitleman":
        raise ValueError("production must require approval specifically from Sitleman")
    if environment.get("can_admins_bypass", True):
        raise ValueError("disable production protection rule bypass")


def gh(*args, **kwargs):
    return subprocess.run(["gh", *args], check=True, text=True, capture_output=True, **kwargs).stdout


def api(path, method="GET", body=None):
    args = ["api", f'repos/{os.environ["GITHUB_REPOSITORY"]}/{path}', "--method", method]
    if body is not None:
        args += ["--input", "-"]
    result = gh(*args, input=json.dumps(body) if body is not None else None)
    return json.loads(result) if result.strip() else None


def pages(path):
    result = []
    page = 1
    while True:
        batch = api(f'{path}{"&" if "?" in path else "?"}per_page=100&page={page}')
        result.extend(batch)
        if len(batch) < 100:
            return result
        page += 1


def git(*args):
    return subprocess.run(["git", *args], check=True, text=True, capture_output=True).stdout.strip()


def validate_changes(base):
    entries = {p.name: read(p) for p in sorted(Path("changes").glob("*.json"))}
    validate_entries(entries)
    if base:
        changes = git("diff", "--name-status", "--no-renames", base, "HEAD", "--", "changes/*.json").splitlines()
        if not changes or any(not line.startswith("A\t") for line in changes):
            raise ValueError("each PR must add a change fragment; existing fragments are immutable")


def register(image):
    commit = os.environ["GITHUB_SHA"]
    entries = {p.name: read(p) for p in sorted(Path("changes").glob("*.json"))}
    refs = api("git/matching-refs/tags/v")
    numeric = [ref for ref in refs if VERSION.fullmatch(ref["ref"].removeprefix("refs/tags/"))]
    existing = None
    latest = None
    for ref in sorted(numeric, key=lambda ref: int(ref["ref"].split("/v")[-1]), reverse=True):
        if ref["object"]["type"] != "tag":
            continue
        tag = api("git/tags/" + ref["object"]["sha"])
        if latest is None:
            latest = tag["object"]["sha"]
        if tag["object"]["sha"] == commit:
            existing = validate(json.loads(tag["message"]))
            break
    if existing:
        manifest = existing
    else:
        if latest:
            git("merge-base", "--is-ancestor", latest, commit)
        number = max((int(ref["ref"].split("/v")[-1]) for ref in numeric), default=0) + 1
        manifest = validate({"version": f"v{number}", "commit": commit, "image": image, "entries": entries})
        tag = api("git/tags", "POST", {"tag": manifest["version"], "message": json.dumps(manifest),
                                      "object": commit, "type": "commit"})
        api("git/refs", "POST", {"ref": "refs/tags/" + manifest["version"], "sha": tag["sha"]})
    releases = pages("releases")
    if not any(r["tag_name"] == manifest["version"] for r in releases):
        api("releases", "POST", {"tag_name": manifest["version"], "name": manifest["version"],
                                 "draft": True, "body": "Версия собрана. Подготовьте prod-релиз после проверки stage."})
    write("release.json", manifest)
    if os.environ.get("GITHUB_OUTPUT"):
        with open(os.environ["GITHUB_OUTPUT"], "a") as out:
            out.write("version=" + manifest["version"] + "\n")


def release_for(version):
    for release in pages("releases"):
        if release["tag_name"] == version:
            return release
    raise ValueError("release not found")


def asset(release, name):
    for item in release["assets"]:
        if item["name"] == name:
            return json.loads(gh("api", item["url"], "-H", "Accept: application/octet-stream"))
    raise ValueError("missing release asset: " + name)


def upload(version, name, value, replace=False):
    release = release_for(version)
    if any(a["name"] == name for a in release["assets"]) and not replace:
        if asset(release, name) != value:
            raise ValueError("refusing to overwrite release evidence")
        return
    with tempfile.TemporaryDirectory() as directory:
        path = Path(directory) / name
        write(path, value)
        args = ["release", "upload", version, str(path), "--repo", os.environ["GITHUB_REPOSITORY"]]
        if replace:
            args.append("--clobber")
        gh(*args)


def production():
    refs = api("git/matching-refs/tags/deployed-production")
    refs = [ref for ref in refs if ref["ref"] == "refs/tags/deployed-production"]
    if refs:
        tag = api("git/tags/" + refs[0]["object"]["sha"])
        return validate(json.loads(tag["message"]))
    base = os.environ.get("RELEASE_BASE_SHA", "")
    if not re.fullmatch(r"[0-9a-f]{40}", base):
        raise ValueError("set RELEASE_BASE_SHA to the bootstrapped production commit")
    return {"version": "v0", "commit": base, "entries": {}}


def mark_production(manifest):
    tag = api("git/tags", "POST", {"tag": "deployed-production", "message": json.dumps(manifest),
                                  "object": manifest["commit"], "type": "commit"})
    refs = api("git/matching-refs/tags/deployed-production")
    if any(ref["ref"] == "refs/tags/deployed-production" for ref in refs):
        api("git/refs/tags/deployed-production", "PATCH", {"sha": tag["sha"], "force": True})
    else:
        api("git/refs", "POST", {"ref": "refs/tags/deployed-production", "sha": tag["sha"]})


def summary(text):
    print(text)
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as out:
            out.write(text + "\n")


if __name__ == "__main__":
    try:
        main()
    except subprocess.CalledProcessError as error:
        raise SystemExit(f"command failed: {error.cmd[0]} (exit {error.returncode})") from None
    except (ValueError, KeyError) as error:
        raise SystemExit(str(error)) from None
