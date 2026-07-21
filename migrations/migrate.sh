#!/bin/sh
set -eu
DB_URL="${DATABASE_URL:-postgres://researcher:researcher@postgres:5432/researcher}"
DB_URL=$(echo "$DB_URL" | sed 's#postgresql+psycopg://#postgres://#; s#postgresql://#postgres://#')

echo "Waiting for database..."
i=0
until psql "$DB_URL" -c 'SELECT 1' >/dev/null 2>&1; do
  i=$((i + 1))
  if [ "$i" -gt 60 ]; then
    echo "database not ready"
    exit 1
  fi
  sleep 1
done

psql "$DB_URL" -v ON_ERROR_STOP=1 -c \
  "CREATE TABLE IF NOT EXISTS schema_migrations (
     id text PRIMARY KEY,
     applied_at timestamptz NOT NULL DEFAULT now()
   );"

# Legacy DB already created by Alembic — mark baseline applied, do not re-run CREATE
if [ "$(psql "$DB_URL" -tAc "SELECT to_regclass('public.users')")" = "users" ]; then
  for f in /migrations/*.sql; do
    [ -f "$f" ] || continue
    name=$(basename "$f")
    psql "$DB_URL" -v ON_ERROR_STOP=1 -c \
      "INSERT INTO schema_migrations(id) VALUES ('$name') ON CONFLICT DO NOTHING;"
  done
  echo "existing schema detected — migrations marked applied"
  exit 0
fi

for f in /migrations/*.sql; do
  [ -f "$f" ] || continue
  name=$(basename "$f")
  exists=$(psql "$DB_URL" -tAc "SELECT 1 FROM schema_migrations WHERE id='$name'")
  if [ "$exists" = "1" ]; then
    echo "skip $name"
    continue
  fi
  echo "apply $name"
  psql "$DB_URL" -v ON_ERROR_STOP=1 -f "$f"
  psql "$DB_URL" -v ON_ERROR_STOP=1 -c \
    "INSERT INTO schema_migrations(id) VALUES ('$name') ON CONFLICT DO NOTHING;"
done
echo "migrations done"
