"""Подготовка приватных настроек мониторинга на существующем сервере проекта."""
from __future__ import annotations

import argparse
import json
import os
import re
import secrets
import subprocess
from pathlib import Path
from urllib.parse import quote


def container_env(name: str) -> dict[str, str]:
    config = json.loads(subprocess.check_output(["docker", "inspect", name], text=True))[0]
    return dict(item.split("=", 1) for item in config["Config"]["Env"] if "=" in item)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--project-name", default="r-a")
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    private = root / ".monitoring-secrets"
    private.mkdir(mode=0o700, exist_ok=True)
    os.chmod(private, 0o700)

    def secret(name: str) -> tuple[str, bool]:
        path = private / name
        created = not path.exists()
        if created:
            path.write_text(secrets.token_urlsafe(36) + "\n")
        value = path.read_text().strip()
        if not re.fullmatch(r"[A-Za-z0-9_-]{32,128}", value):
            raise SystemExit("Неожиданный формат файла секрета: " + name)
        # Docker монтирует отдельный файл; каталог 0700 закрывает его для других пользователей хоста.
        os.chmod(path, 0o644)
        return value, created

    secret("grafana_admin_password")
    password, created = secret("postgres_password")
    pg_name = args.project_name + "-postgres-1"
    pg = container_env(pg_name)
    user, database = pg.get("POSTGRES_USER", "postgres"), pg.get("POSTGRES_DB", "postgres")
    command = ["docker", "exec", "-i", pg_name, "psql", "-v", "ON_ERROR_STOP=1", "-U", user, "-d", database]
    exists = subprocess.check_output(command + ["-Atc", "SELECT 1 FROM pg_roles WHERE rolname='ra_monitor'"], text=True).strip() == "1"
    if exists and created:
        (private / "postgres_password").unlink()
        raise SystemExit("Роль ra_monitor уже существует без сохранённого секрета; пароль не изменён")
    sql = ""
    if not exists:
        sql += "CREATE ROLE ra_monitor LOGIN PASSWORD '" + password + "';\n"
    sql += "GRANT pg_monitor TO ra_monitor;\n"
    # Пароль передаётся по stdin, а не аргументом процесса и не выводится в лог.
    subprocess.run(command, input=sql, text=True, check=True, stdout=subprocess.DEVNULL)

    pooler = container_env(args.project_name + "-pgbouncer-1")
    pool_user, pool_password = pooler["DB_USER"], pooler["DB_PASSWORD"]
    values = {
        "MONITORING_DATABASE_URL": "postgresql://ra_monitor:" + quote(password, safe="") + "@postgres:5432/" + quote(database, safe="") + "?sslmode=disable",
        "MONITORING_PGBOUNCER_URL": "postgresql://" + quote(pool_user, safe="") + ":" + quote(pool_password, safe="") + "@pgbouncer:5432/pgbouncer?sslmode=disable",
        "PGBOUNCER_STATS_USERS": pool_user,
        "MONITORING_SECRETS_DIR": str(private),
        "GRAFANA_ADMIN_USER": "admin",
        "GRAFANA_BIND_IP": "127.0.0.1",
        "GRAFANA_PORT": "3000",
    }
    environment = root / ".env.monitoring"
    # Существующие операторские настройки сохраняются; секреты берём из приватных файлов.
    if environment.exists():
        for line in environment.read_text().splitlines():
            if line and not line.startswith("#") and "=" in line:
                key, value = line.split("=", 1)
                if key not in {"MONITORING_DATABASE_URL", "MONITORING_PGBOUNCER_URL", "MONITORING_SECRETS_DIR"}:
                    values[key] = value
    environment.write_text("\n".join(key + "=" + value for key, value in values.items()) + "\n")
    os.chmod(environment, 0o600)
    print("Настройки подготовлены:", environment)
    print("Пароль Grafana:", private / "grafana_admin_password")
    print("PostgreSQL: роль ra_monitor имеет pg_monitor; прикладные таблицы ей не выдаются")


if __name__ == "__main__":
    main()
