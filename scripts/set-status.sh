#!/usr/bin/env bash
set -euo pipefail

OWNER="Sitleman"
PROJECT_NUMBER="3"
PROJECT_ID="PVT_kwHOAnMETc4Bdyj6"
FIELD_ID="PVTSSF_lAHOAnMETc4Bdyj6zhYRip8"
ITEM_LIMIT="500"

VALID_STATUSES="Проработка | Аппрув / Нужна информация | В работе | Ревью"

die() {
  echo "error: $*" >&2
  exit 1
}

usage() {
  echo "usage: set-status.sh <issue-number> <status-name>" >&2
  echo "  statuses: $VALID_STATUSES" >&2
  echo "  Открыт/Закрыт are set by the project's built-in workflows" >&2
  exit 1
}

option_id_for() {
  case "$1" in
    "Проработка") echo "d4c2812c" ;;
    "Аппрув / Нужна информация") echo "e5ce596f" ;;
    "В работе") echo "bc53b63c" ;;
    "Ревью") echo "a50a8d93" ;;
    "Открыт" | "Закрыт")
      die "status '$1' is set by GitHub project workflows, not this script" ;;
    *)
      die "unknown status '$1'. valid: $VALID_STATUSES" ;;
  esac
}

find_item_id() {
  local issue="$1" json item
  command -v gh >/dev/null 2>&1 || die "gh CLI not found"
  json="$(gh project item-list "$PROJECT_NUMBER" --owner "$OWNER" --format json --limit "$ITEM_LIMIT")" ||
    die "failed to query Project #$PROJECT_NUMBER — check auth: gh auth refresh -s project"
  item="$(printf '%s' "$json" | ISSUE="$issue" LIMIT="$ITEM_LIMIT" python3 -c '
import json, os, sys
issue = int(os.environ["ISSUE"])
limit = int(os.environ["LIMIT"])
items = json.load(sys.stdin).get("items", [])
if len(items) >= limit:
    sys.stderr.write("warning: item list hit limit %d; issue may be missed\n" % limit)
for it in items:
    c = it.get("content") or {}
    if c.get("type") == "Issue" and c.get("number") == issue:
        print(it["id"])
        break
')"
  [ -n "$item" ] || die "issue #$issue is not on board Project #$PROJECT_NUMBER (add it first)"
  echo "$item"
}

main() {
  [ "$#" -eq 2 ] || usage
  local issue="$1" status="$2" option_id item
  [[ "$issue" =~ ^[0-9]+$ ]] || die "issue number must be numeric, got '$issue'"
  option_id="$(option_id_for "$status")"

  if [ "${SET_STATUS_DRY_RUN:-}" = "1" ]; then
    echo "dry-run: issue=#$issue status='$status' option_id=$option_id"
    echo "dry-run: gh project item-edit --field-id $FIELD_ID --single-select-option-id $option_id --project-id $PROJECT_ID"
    return 0
  fi

  item="$(find_item_id "$issue")"
  gh project item-edit \
    --id "$item" \
    --field-id "$FIELD_ID" \
    --single-select-option-id "$option_id" \
    --project-id "$PROJECT_ID" >/dev/null
  echo "CH-$issue → $status ✓"
}

main "$@"
