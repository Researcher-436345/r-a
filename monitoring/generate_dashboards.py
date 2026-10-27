"""Воспроизводимая сборка дашбордов Grafana: только Prometheus, без пользовательских ID."""
from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent / "grafana" / "dashboards"
DS = {"type": "prometheus", "uid": "prometheus"}


def dashboard(uid: str, title: str, note: str) -> dict:
    return {"uid": uid, "title": title, "tags": ["Odyssey", "monitoring"], "timezone": "utc",
            "schemaVersion": 39, "version": 1, "editable": False, "refresh": "30s",
            "time": {"from": "now-24h", "to": "now"}, "panels": [
                {"id": 1, "type": "text", "title": "Как читать показатели", "gridPos": {"x": 0, "y": 0, "w": 24, "h": 4},
                 "options": {"mode": "markdown", "content": note}}],
            "links": [{"title": title, "type": "link", "url": "/d/" + key, "keepTime": True}
                      for key, title in [("ra-overview", "Общее состояние"), ("ra-product", "Продукт"),
                                         ("ra-reliability", "Надёжность"), ("ra-llm", "LLM и расходы"), ("ra-infrastructure", "Инфраструктура")]]}


def panel(d: dict, title: str, query: str | list[tuple[str, str]], x: int, y: int, w: int = 12, h: int = 8,
          kind: str = "timeseries", unit: str = "short", legend: str = "{{service}}", description: str = "") -> None:
    targets = [(query, legend)] if isinstance(query, str) else query
    p = {"id": len(d["panels"]) + 1, "title": title, "description": description, "type": kind,
         "datasource": DS, "gridPos": {"x": x, "y": y, "w": w, "h": h},
         "targets": [{"refId": chr(65 + i), "expr": expr, "legendFormat": label, "datasource": DS,
                      "instant": kind in {"stat", "bargauge"}, "range": kind not in {"stat", "bargauge"}}
                     for i, (expr, label) in enumerate(targets)],
         "fieldConfig": {"defaults": {"unit": unit, "noValue": "Нет данных", "decimals": 2,
                                        "color": {"mode": "palette-classic"}}, "overrides": []},
         "options": {"legend": {"displayMode": "list", "placement": "bottom"}, "tooltip": {"mode": "multi"}}}
    if kind in {"stat", "bargauge"}:
        p["options"] = {"reduceOptions": {"calcs": ["lastNotNull"], "values": False}, "textMode": "auto", "colorMode": "value"}
    d["panels"].append(p)


def ratio(metric: str, numerator: str, denominator: str, group: str = "") -> str:
    agg = f"max by ({group})" if group else "max"
    return f'{agg}({metric}{{{numerator}}}) / {agg}({metric}{{{denominator}}})'


def build() -> list[dict]:
    overview = dashboard("ra-overview", "Odyssey · Общее состояние",
        "Продуктовые показатели относятся к авторизованным пользователям и пересчитываются раз в 15 минут. "
        "Окна DAU/WAU/MAU — последние 1/7/30 суток. Retention и BRR считаются по зрелым когортам; "
        "нулевой знаменатель означает **недостаточно данных**. История начинается с подключения мониторинга.")
    stats = [
        ("Пользователи с результатом за 7 дней", 'max(ra_product_value_users{window="7d"})', "short"),
        ("WAU", 'max(ra_product_active_users{window="7d"})', "short"),
        ("MAU", 'max(ra_product_active_users{window="30d"})', "short"),
        ("D7 retention", ratio("ra_product_retention_users", 'period="d7",state="returned"', 'period="d7",state="eligible"'), "percentunit"),
        ("D30 retention", ratio("ra_product_retention_users", 'period="d30",state="returned"', 'period="d30",state="eligible"'), "percentunit"),
        ("BRR7", ratio("ra_product_project_return_projects", 'period="7d",state="returned"', 'period="7d",state="eligible"'), "percentunit")]
    for i, (title, query, unit) in enumerate(stats): panel(overview, title, query, i * 4, 4, 4, 4, "stat", unit, "")
    panel(overview, "Активная аудитория", 'max by (window)(ra_product_active_users)', 0, 8, legend="{{window}}")
    panel(overview, "Retention по окнам", ratio("ra_product_retention_users", 'state="returned"', 'state="eligible"', "period"), 12, 8, unit="percentunit", legend="{{period}}")
    panel(overview, "Доступность проверяемых сценариев", 'probe_success{job="probes"}', 0, 16, legend="{{instance}}")
    panel(overview, "Активные алерты", 'ALERTS{alertstate="firing"}', 12, 16, kind="stat", legend="{{alertname}}")

    product = dashboard("ra-product", "Odyssey · Продукт и воронки",
        "Использование функции — просмотр готового результата; генерация и кэш считаются отдельно. Повторы HTTP дедуплицируются. "
        "Активация — результат за первые 7 суток. D7/D30 — возврат в конкретный день; W1=дни 1–7, W2=8–14, W4=22–28. "
        "Для retention берём последние 30 полностью наблюдавшихся дневных когорт. Системные папки исключены из проектов. "
        "BRR — содержательный возврат на другой день. Конверсии поиска имеют полное 7-дневное окно после завершения. "
        "Открытие и сохранение — независимые конверсии: статью можно сохранить прямо из ответа.")
    panel(product, "Активировано за 7 дней", ratio("ra_product_activation_users", 'state="activated"', 'state="eligible"'), 0, 4, 6, 4, "stat", "percentunit", "")
    panel(product, "Размер когорты активации", 'max(ra_product_activation_users{state="eligible"})', 6, 4, 6, 4, "stat", legend="")
    panel(product, "Время до результата: медиана", 'max(ra_product_time_to_first_value_seconds{quantile="0.5"})', 12, 4, 6, 4, "stat", "s", "")
    panel(product, "Время до результата: p90", 'max(ra_product_time_to_first_value_seconds{quantile="0.9"})', 18, 4, 6, 4, "stat", "s", "")
    panel(product, "Feature adoption / MAU", 'max by (feature)(ra_product_feature_users) / scalar(max(ra_product_active_users{window="30d"}))', 0, 8, kind="bargauge", unit="percentunit", legend="{{feature}}")
    panel(product, "Использований на активного пользователя за 30 дней", 'max by (feature)(ra_product_feature_uses) / scalar(max(ra_product_active_users{window="30d"}))', 12, 8, kind="bargauge", legend="{{feature}}")
    panel(product, "Пользователи функций за 30 дней", 'max by (feature)(ra_product_feature_users)', 0, 16, legend="{{feature}}")
    panel(product, "Использование: медиана и p90, включая нули", 'max by (feature,quantile)(ra_product_feature_usage_quantile)', 12, 16, legend="{{feature}} · {{quantile}}")
    panel(product, "Retention: числители и знаменатели", 'max by (period,state)(ra_product_retention_users)', 0, 24, legend="{{period}} · {{state}}")
    panel(product, "Возвраты к проектам BRR7 / BRR30", ratio("ra_product_project_return_projects", 'state="returned"', 'state="eligible"', "period"), 12, 24, unit="percentunit", legend="{{period}}")
    panel(product, "Проекты: размер зрелых когорт", 'max by (period)(ra_product_project_return_projects{state="eligible"})', 0, 32, legend="{{period}}")
    panel(product, "Активация проекта за 7 дней", ratio("ra_product_project_activation_projects", 'state="activated"', 'state="eligible"'), 12, 32, unit="percentunit", legend="")
    panel(product, "Поиск: завершения, открытия и сохранения", 'max by (feature,stage)(ra_product_search_funnel_actions)', 0, 40, legend="{{feature}} · {{stage}}")
    panel(product, "Итоги действий за 30 дней", 'max by (feature,outcome)(ra_product_action_results)', 12, 40, legend="{{feature}} · {{outcome}}")
    panel(product, "Полезность среди оценивших", 'sum(ra_product_feedback_results{feedback="helpful"}) / sum(ra_product_feedback_results)', 0, 48, unit="percentunit", legend="")
    panel(product, "Обратная связь и дальнейшие действия", [("max by (feedback)(ra_product_feedback_results)", "{{feedback}}"), ("max by (action)(ra_product_result_actions)", "{{action}}")], 12, 48)
    panel(product, "Покрытие оценками", 'sum(ra_product_feedback_results) / sum(ra_product_feature_uses)', 0, 56, 6, 4, "stat", "percentunit", "")
    panel(product, "Действия без завершения >30 минут", 'max(ra_product_stalled_actions)', 6, 56, 6, 4, "stat", legend="")
    panel(product, "Давность пересчёта", 'time()-max(ra_product_aggregation_last_success_timestamp_seconds)', 12, 56, 6, 4, "stat", "s", "")
    panel(product, "Сбор событий начат", '1000*max(ra_product_tracking_started_timestamp_seconds)', 18, 56, 6, 4, "stat", "dateTimeAsIso", "")

    reliable = dashboard("ra-reliability", "Odyssey · Надёжность приложения",
        "Итог SSE определяется по done/error, а не по HTTP 200. Время до первого содержимого исключает сообщения прогресса. "
        "Ошибки генерации, отмены и ожидание готовности обзора разделены. Проверки blackbox здесь выполняются с того же VPS; "
        "для обнаружения потери всего сервера нужна независимая внешняя проверка.")
    panel(reliable, "Доступность сценариев", 'probe_success{job="probes"}', 0, 4, legend="{{instance}}")
    panel(reliable, "Состояние сбора метрик", 'up{job=~"application|python"}', 12, 4, legend="{{service}}")
    panel(reliable, "HTTP запросов/с", 'sum by(service)(rate(ra_http_requests_total{route!="/health"}[$__rate_interval]))', 0, 12)
    panel(reliable, "HTTP 5xx", 'sum by(service)(rate(ra_http_requests_total{status=~"5.."}[$__rate_interval]))', 12, 12)
    panel(reliable, "HTTP задержка p95", 'histogram_quantile(0.95,sum by(le,service)(rate(ra_http_request_duration_seconds_bucket{route!="/health"}[$__rate_interval])))', 0, 20, unit="s")
    panel(reliable, "Завершение пользовательских действий", 'sum by(feature,outcome)(rate(ra_action_results_total[$__rate_interval]))', 12, 20, legend="{{feature}} · {{outcome}}")
    panel(reliable, "Время до первого содержимого p95", 'histogram_quantile(0.95,sum by(le,feature)(rate(ra_action_first_content_seconds_bucket[$__rate_interval])))', 0, 28, unit="s", legend="{{feature}}")
    panel(reliable, "Полное время действия p95", 'histogram_quantile(0.95,sum by(le,feature)(rate(ra_action_duration_seconds_bucket[$__rate_interval])))', 12, 28, unit="s", legend="{{feature}}")
    panel(reliable, "Очередь PDF", 'sum by(state)(ra_queue_tasks)', 0, 36, legend="{{state}}")
    panel(reliable, "Возраст старейшей ожидающей задачи", 'max(ra_queue_oldest_pending_age_seconds)', 12, 36, unit="s", legend="")
    panel(reliable, "Задачи: попытки и ошибки", 'sum by(type,outcome)(rate(ra_job_attempts_total[$__rate_interval]))', 0, 44, legend="{{type}} · {{outcome}}")
    panel(reliable, "Давность обновления русской ленты", 'time()-ra_feed_refresh_last_success_timestamp_seconds', 12, 44, unit="s", legend="{{sort}}")
    panel(reliable, "Попадания в кэш", 'sum by(cache)(rate(ra_cache_requests_total{outcome="hit"}[$__rate_interval])) / sum by(cache)(rate(ra_cache_requests_total[$__rate_interval]))', 0, 52, unit="percentunit", legend="{{cache}}")
    panel(reliable, "Запись аналитики", 'sum by(producer,outcome)(increase(ra_analytics_event_writes_total[$__range]))', 12, 52, legend="{{producer}} · {{outcome}}")

    llm = dashboard("ra-llm", "Odyssey · LLM и расходы",
        "Стоимость — только суммы, явно возвращённые провайдером; это мониторинг наблюдаемых расходов, а не сверка счёта. "
        "Пропущенные usage/cost не превращаются в бесплатные запросы: покрытие показывается отдельно. "
        "Первые короткие интервалы и перезапуски ограничивают точность increase(). Тексты запросов и ответов не экспортируются.")
    panel(llm, "Сообщённые расходы за выбранный период", 'sum(increase(ra_llm_reported_cost_dollars_total[$__range]))', 0, 4, 6, 4, "stat", "currencyUSD", "")
    panel(llm, "Покрытие отчётами стоимости", 'sum(increase(ra_llm_usage_reports_total{kind="cost"}[$__range]))/sum(increase(ra_llm_usage_reports_total{kind=~"cost|missing_cost"}[$__range]))', 6, 4, 6, 4, "stat", "percentunit", "")
    panel(llm, "Вызовы провайдеров", 'sum(increase(ra_llm_requests_total[$__range]))', 12, 4, 6, 4, "stat", legend="")
    panel(llm, "Доля кэшированных обзоров", 'sum(increase(ra_action_results_total{feature="summary",cache="hit",outcome="success"}[$__range])) / sum(increase(ra_action_results_total{feature="summary",outcome="success"}[$__range]))', 18, 4, 6, 4, "stat", "percentunit", "")
    panel(llm, "Расходы по функциям и моделям", 'sum by(model,feature)(increase(ra_llm_reported_cost_dollars_total[$__range]))', 0, 8, unit="currencyUSD", legend="{{feature}} · {{model}}")
    panel(llm, "Токены", 'sum by(model,feature,direction)(increase(ra_llm_tokens_total[$__range]))', 12, 8, legend="{{feature}} · {{direction}} · {{model}}")
    panel(llm, "Итоги обращений к провайдерам", 'sum by(model,outcome)(rate(ra_llm_requests_total[$__rate_interval]))', 0, 16, legend="{{model}} · {{outcome}}")
    panel(llm, "Задержка провайдера p95", 'histogram_quantile(0.95,sum by(le,model,feature)(rate(ra_llm_request_duration_seconds_bucket[$__rate_interval])))', 12, 16, unit="s", legend="{{feature}} · {{model}}")
    panel(llm, "Наличие usage и cost", 'sum by(kind)(increase(ra_llm_usage_reports_total[$__range]))', 0, 24, legend="{{kind}}")
    panel(llm, "HTTP-ошибки LLM-провайдеров", 'sum by(model,status)(increase(ra_llm_http_responses_total{status!~"2.."}[$__range]))', 12, 24, legend="{{model}} · {{status}}")
    panel(llm, "Источник обзора: полный текст или аннотация", 'sum by(source,cache)(increase(ra_summary_source_results_total[$__range]))', 0, 32, legend="{{source}} · {{cache}}")
    panel(llm, "Бюджет за 24 часа (если задан)", 'max(ra_llm_daily_budget_dollars)', 12, 32, kind="stat", unit="currencyUSD", legend="")

    infra = dashboard("ra-infrastructure", "Odyssey · Инфраструктура",
        "Ресурсы единственного VPS, контейнеры и зависимости. Экспортеры доступны только внутри Docker-сети. "
        "Отсутствие ряда означает отсутствие наблюдения, а не нулевую нагрузку. Метрики процесса дополнительно доступны по каждому Go/Python-сервису.")
    panel(infra, "CPU сервера", '1-avg(rate(node_cpu_seconds_total{mode="idle"}[$__rate_interval]))', 0, 4, unit="percentunit", legend="")
    panel(infra, "Доступная RAM", 'node_memory_MemAvailable_bytes', 12, 4, unit="bytes", legend="")
    panel(infra, "Свободное место /", 'node_filesystem_avail_bytes{mountpoint="/",fstype!~"tmpfs|overlay|squashfs"}', 0, 12, unit="bytes", legend="{{device}}")
    panel(infra, "CPU контейнеров", 'sum by(name)(rate(container_cpu_usage_seconds_total{name!=""}[$__rate_interval]))', 12, 12, legend="{{name}}")
    panel(infra, "RAM контейнеров", 'sum by(name)(container_memory_working_set_bytes{name!=""})', 0, 20, unit="bytes", legend="{{name}}")
    panel(infra, "PostgreSQL и Redis", [('pg_up', 'PostgreSQL'), ('redis_up', 'Redis')], 12, 20)
    panel(infra, "Подключения приложения к БД", 'ra_db_pool_connections', 0, 28)
    panel(infra, "Занятые подключения приложения", 'ra_db_pool_acquired_connections', 12, 28)
    panel(infra, "Отменённые ожидания соединения", 'sum by(service)(increase(ra_db_pool_canceled_acquires_total[$__range]))', 0, 36)
    panel(infra, "Память Redis", 'redis_memory_used_bytes', 12, 36, unit="bytes", legend="")
    panel(infra, "Redis: вытеснения и промахи", [('rate(redis_evicted_keys_total[$__rate_interval])', 'Вытеснения'), ('rate(redis_keyspace_misses_total[$__rate_interval])', 'Промахи')], 0, 44)
    panel(infra, "Перезапуски / начало жизни контейнеров", 'container_start_time_seconds{name!=""}*1000', 12, 44, unit="dateTimeAsIso", legend="{{name}}")
    panel(infra, "Все источники метрик", 'up', 0, 52, 24, 8, legend="{{job}} · {{instance}}")
    return [overview, product, reliable, llm, infra]


if __name__ == "__main__":
    ROOT.mkdir(parents=True, exist_ok=True)
    for d in build():
        (ROOT / (d["uid"] + ".json")).write_text(json.dumps(d, ensure_ascii=False, indent=2) + "\n")
    print("Собраны пять дашбордов Grafana")
