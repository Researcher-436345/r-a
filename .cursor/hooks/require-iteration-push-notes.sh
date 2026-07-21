#!/usr/bin/env bash
# Gate real remote pushes: require docs/ITERATION_PUSH.md (or ITERATION_PUSH.md) with today's date.
set -euo pipefail

input=$(cat || true)
command=$(printf '%s' "$input" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("command") or "")' 2>/dev/null || true)

if ! printf '%s' "$command" | grep -Eq '(^|[;&|[:space:]])git[[:space:]]+push([[:space:]]|$)'; then
  printf '%s\n' '{"permission":"allow"}'
  exit 0
fi

if printf '%s' "$command" | grep -Eq '[[:space:]](-n|--dry-run|--help)([[:space:]]|$)'; then
  printf '%s\n' '{"permission":"allow"}'
  exit 0
fi

root=$(pwd)
today=$(date +%Y-%m-%d)
notes=""
for cand in \
  "$root/docs/ITERATION_PUSH.md" \
  "$root/ITERATION_PUSH.md" \
  "$root/r-a/docs/ITERATION_PUSH.md" \
  "$root/r-a/ITERATION_PUSH.md"
do
  if [[ -f "$cand" ]]; then
    notes="$cand"
    break
  fi
done

if [[ -z "$notes" ]]; then
  python3 - <<'PY'
import json
print(json.dumps({
  "permission": "deny",
  "user_message": "Нужен docs/ITERATION_PUSH.md перед push. Скилл: iteration-push-notes.",
  "agent_message": "Blocked: docs/ITERATION_PUSH.md missing. Apply skill iteration-push-notes, write the file with today's Last refreshed, commit, retry."
}))
PY
  exit 0
fi

refreshed=$(grep -E '^Last refreshed:' "$notes" | head -1 | sed 's/^Last refreshed:[[:space:]]*//' | tr -d '\r' || true)

if [[ "$refreshed" != "$today" ]]; then
  python3 - <<PY
import json
print(json.dumps({
  "permission": "deny",
  "user_message": "docs/ITERATION_PUSH.md устарел — обновите Last refreshed на сегодня.",
  "agent_message": "Blocked: Last refreshed='${refreshed}', need '${today}'. Apply skill iteration-push-notes, commit, retry."
}))
PY
  exit 0
fi

printf '%s\n' '{"permission":"allow"}'
exit 0
