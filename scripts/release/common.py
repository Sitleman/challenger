import json
import re
from pathlib import Path


VERSION = re.compile(r"v[1-9][0-9]*\Z")
SHA = re.compile(r"[0-9a-f]{40}\Z")
IMAGE = re.compile(r"ghcr\.io/[a-z0-9_./-]+@sha256:[0-9a-f]{64}\Z")
TYPES = {"feature": "Новое", "improvement": "Улучшения", "fix": "Исправления", "internal": ""}


def validate(manifest):
    if not VERSION.fullmatch(manifest["version"]):
        raise ValueError("invalid version")
    if not SHA.fullmatch(manifest["commit"]):
        raise ValueError("invalid commit")
    if not IMAGE.fullmatch(manifest["image"]):
        raise ValueError("image must be pinned to GHCR digest")
    validate_entries(manifest["entries"])
    return manifest


def validate_entries(entries):
    for name, entry in entries.items():
        if not re.fullmatch(r"[a-zA-Z0-9_-]+\.json", name):
            raise ValueError("invalid change filename")
        if set(entry) != {"type", "text"} or entry["type"] not in TYPES:
            raise ValueError(f"invalid change: {name}")
        if not isinstance(entry["text"], str) or not entry["text"].strip() or len(entry["text"]) > 600:
            raise ValueError(f"invalid change text: {name}")


def changelog(previous, target):
    sections = []
    for kind, heading in TYPES.items():
        if kind == "internal":
            continue
        texts = dict.fromkeys(entry["text"].strip() for key, entry in sorted(target["entries"].items())
                              if key not in previous.get("entries", {}) and entry["type"] == kind)
        if texts:
            sections.append(heading + "\n" + "\n".join("• " + text for text in texts))
    return "\n\n".join(sections) or "Технические изменения, команды работают как прежде."


def message(version, notes):
    if not isinstance(notes, str):
        raise ValueError("changelog must be text")
    text = f"🚀 Бот обновлён до {version}\n\n{notes.strip()}"
    if not notes.strip() or len(text.encode("utf-16-le")) // 2 > 4096:
        raise ValueError("changelog must fit one Telegram message (4096 UTF-16 units)")
    return text


def read(path):
    return json.loads(Path(path).read_text())


def write(path, value):
    path = Path(path)
    temporary = path.with_suffix(path.suffix + ".next")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
    temporary.replace(path)
