"""Метрики без содержимого запросов, ответов и идентификаторов пользователей."""
from __future__ import annotations

import math
import time
from typing import Any

from fastapi import FastAPI
from fastapi.responses import Response
from prometheus_client import CONTENT_TYPE_LATEST, Counter, Histogram, generate_latest

HTTP_REQUESTS = Counter("ra_python_http_requests_total", "Completed HTTP requests", ["service", "route", "method", "status"])
HTTP_DURATION = Histogram("ra_python_http_request_duration_seconds", "Full HTTP duration including streaming", ["service", "route"], buckets=(.01, .05, .1, .25, .5, 1, 5, 15, 60, 180, 600, 1200))
LLM_REQUESTS = Counter("ra_llm_requests_total", "Provider requests including retries", ["model", "feature", "outcome"])
LLM_DURATION = Histogram("ra_llm_request_duration_seconds", "Provider request duration", ["model", "feature"], buckets=(.1, .5, 1, 2, 5, 10, 30, 60, 120, 300, 600))
LLM_HTTP = Counter("ra_llm_http_responses_total", "Provider HTTP status", ["model", "feature", "status"])
LLM_TOKENS = Counter("ra_llm_tokens_total", "Reported provider token usage", ["model", "feature", "direction"])
LLM_COST = Counter("ra_llm_reported_cost_dollars_total", "Explicitly reported provider cost", ["model", "feature"])
LLM_USAGE = Counter("ra_llm_usage_reports_total", "Availability of provider usage and cost", ["model", "feature", "kind"])


def record_usage(model: str, feature: str, usage: dict[str, Any] | None) -> None:
    usage = usage if isinstance(usage, dict) else {}
    inputs, outputs = usage.get("prompt_tokens"), usage.get("completion_tokens")
    if isinstance(inputs, int) and isinstance(outputs, int) and inputs >= 0 and outputs >= 0:
        LLM_TOKENS.labels(model, feature, "input").inc(inputs)
        LLM_TOKENS.labels(model, feature, "output").inc(outputs)
        details = usage.get("prompt_tokens_details")
        cached = details.get("cached_tokens", 0) if isinstance(details, dict) else 0
        if isinstance(cached, int) and cached > 0:
            LLM_TOKENS.labels(model, feature, "cached_input").inc(cached)
        LLM_USAGE.labels(model, feature, "tokens").inc()
    else:
        LLM_USAGE.labels(model, feature, "missing_tokens").inc()
    cost = usage.get("cost")
    if isinstance(cost, (int, float)) and math.isfinite(cost) and cost >= 0:
        LLM_COST.labels(model, feature).inc(cost)
        LLM_USAGE.labels(model, feature, "cost").inc()
    else:
        LLM_USAGE.labels(model, feature, "missing_cost").inc()


class HTTPMetrics:
    def __init__(self, app: Any, service: str) -> None:
        self.app, self.service = app, service

    async def __call__(self, scope: dict[str, Any], receive: Any, send: Any) -> None:
        if scope["type"] != "http" or scope.get("path") == "/metrics":
            await self.app(scope, receive, send)
            return
        started, status = time.monotonic(), 500

        async def observed_send(message: dict[str, Any]) -> None:
            nonlocal status
            if message["type"] == "http.response.start":
                status = message["status"]
            await send(message)

        try:
            await self.app(scope, receive, observed_send)
        finally:
            route = getattr(scope.get("route"), "path", "unmatched")
            method = scope.get("method", "OTHER")
            if method not in {"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}:
                method = "OTHER"
            HTTP_REQUESTS.labels(self.service, route, method, str(status)).inc()
            HTTP_DURATION.labels(self.service, route).observe(time.monotonic() - started)


def setup_metrics(app: FastAPI, service: str) -> None:
    app.add_middleware(HTTPMetrics, service=service)

    @app.get("/metrics", include_in_schema=False)
    def metrics() -> Response:
        return Response(generate_latest(), headers={"Content-Type": CONTENT_TYPE_LATEST})
