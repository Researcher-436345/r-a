#!/usr/bin/env bash
# Before `git push`: require ITERATION_PUSH.md refreshed today (Last refreshed: YYYY-MM-DD).
set -euo pipefail

input=$(cat || true)
command=$(printf '%s' "$input" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("command") or "")' 2>/dev/null || true)

# Only gate real pushes (allow dry-run / help)
case "$command" in
  *git\ push*)
    ;;
  *)
    printf '%s\n' '{"permission":"allow"}'
    exit 0
    ;;
esac

if printf '%s' "$command" | grep -Eq 'push[[:space:]]+(-n|--dry-run)\b|push[[:space:]]+--help\b'; then
  printf '%s\n' '{"permission":"allow"}'
  exit 0
fi

# Find repo root with ITERATION_PUSH.md or docker-compose.yml
root=$(pwd)
notes=""
for cand in "$root/ITERATION_PUSH.md" "$root/../ITERATION_PUSH.md"; do
  if [[ -f "$cand" ]]; then
    notes="$cand"
    break
  fi
done

today=$(date +%Y-%m-%d)

if [[ -z "$notes" ]]; then
  python3 - <<'PY'
import json
print(json.dumps({
  "permission": "deny",
  "user_message": "Перед push нужен файл ITERATION_PUSH.md. Агент обновит его по скиллу iteration-push-notes.",
  "agent_message": "Push blocked: ITERATION_PUSH.md missing. Apply skill iteration-push-notes, write ITERATION_PUSH.md with today's Last refreshed date, commit it, then retry git push."
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
  "user_message": "ITERATION_PUSH.md устарел (нужна сегодняшняя дата Last refreshed). Обновите заметки перед push.",
  "agent_message": "Push blocked: ITERATION_PUSH.md Last refreshed is '${refreshed}' (need ${today}). Apply skill iteration-push-notes, refresh ITERATION_PUSH.md, commit, then retry git push."
}))
PY
  exit 0
fi

printf '%s\n' '{"permission":"allow"}'
exit 0
