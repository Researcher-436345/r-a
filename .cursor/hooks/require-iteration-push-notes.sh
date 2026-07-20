#!/usr/bin/env bash
# Gate real pushes: require ITERATION_PUSH.md with today's Last refreshed date.
set -euo pipefail

input=$(cat || true)
command=$(printf '%s' "$input" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("command") or "")' 2>/dev/null || true)

# Allow anything that is not a push to a remote
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
  "$root/ITERATION_PUSH.md" \
  "$root/r-a/ITERATION_PUSH.md"
do
  if [[ -f "$cand" ]]; then
    notes="$cand"
    break
  fi
done

if [[ -z "$notes" ]]; then
  sub=$(printf '%s' "$command" | python3 -c 'import re,sys; m=re.search(r"cd\s+([^\s;&|]+)", sys.stdin.read()); print(m.group(1) if m else "")' 2>/dev/null || true)
  if [[ -n "$sub" && "$sub" != /* ]]; then
    sub="$root/$sub"
  fi
  if [[ -n "$sub" && -f "$sub/ITERATION_PUSH.md" ]]; then
    notes="$sub/ITERATION_PUSH.md"
  fi
fi

if [[ -z "$notes" ]]; then
  python3 - <<'PY'
import json
print(json.dumps({
  "permission": "deny",
  "user_message": "Нужен ITERATION_PUSH.md перед отправкой в remote. Скилл: iteration-push-notes.",
  "agent_message": "Blocked: ITERATION_PUSH.md missing. Apply skill iteration-push-notes, write the file with today's Last refreshed, commit, retry."
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
  "user_message": "ITERATION_PUSH.md устарел — обновите Last refreshed на сегодня.",
  "agent_message": "Blocked: ITERATION_PUSH.md Last refreshed='${refreshed}', need '${today}'. Apply skill iteration-push-notes, commit, retry."
}))
PY
  exit 0
fi

printf '%s\n' '{"permission":"allow"}'
exit 0
