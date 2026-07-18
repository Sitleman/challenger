#!/usr/bin/env bash
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="$SCRIPT_DIR/set-status.sh"

pass=0
fail=0

run_case() {
  local desc="$1" exp_rc="$2" exp_sub="$3"
  shift 3
  [ "$1" = "--" ] && shift
  local out rc
  out="$(SET_STATUS_DRY_RUN="${DRY:-}" bash "$SCRIPT" "$@" 2>&1)"
  rc=$?
  if [ "$rc" -eq "$exp_rc" ] && printf '%s' "$out" | grep -qF "$exp_sub"; then
    pass=$((pass + 1))
    echo "ok   - $desc"
  else
    fail=$((fail + 1))
    echo "FAIL - $desc"
    echo "       expected rc=$exp_rc substr='$exp_sub'"
    echo "       got      rc=$rc"
    echo "       output   : $out"
  fi
}

DRY="" run_case "no args -> usage"            1 "usage"          --
DRY="" run_case "one arg -> usage"            1 "usage"          -- 5
DRY="" run_case "non-numeric issue -> error"  1 "numeric"        -- abc "В работе"
DRY="" run_case "unknown status -> error"     1 "unknown status" -- 5 "Нечто"
DRY="" run_case "Открыт rejected (GitHub)"    1 "GitHub"         -- 5 "Открыт"
DRY="" run_case "Закрыт rejected (GitHub)"    1 "GitHub"         -- 5 "Закрыт"

DRY=1 run_case "dry-run В работе -> bc53b63c"   0 "bc53b63c" -- 5 "В работе"
DRY=1 run_case "dry-run Проработка -> d4c2812c" 0 "d4c2812c" -- 5 "Проработка"
DRY=1 run_case "dry-run Ревью -> a50a8d93"      0 "a50a8d93" -- 5 "Ревью"
DRY=1 run_case "dry-run Аппрув -> e5ce596f"     0 "e5ce596f" -- 5 "Аппрув / Нужна информация"

echo
echo "passed: $pass, failed: $fail"
[ "$fail" -eq 0 ]
