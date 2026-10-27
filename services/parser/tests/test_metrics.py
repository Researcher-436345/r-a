"""Проверки достоверности метрик и ограниченной кардинальности HTTP-маршрутов."""
import unittest

from fastapi import FastAPI
from fastapi.testclient import TestClient
from prometheus_client import REGISTRY

from app.metrics import setup_metrics, record_usage


class MetricsTest(unittest.TestCase):
    def test_metrics_do_not_record_raw_paths_or_queries(self):
        app = FastAPI()
        setup_metrics(app, "metrics-test")

        @app.get("/documents/{document_id}")
        def document(document_id: str):
            return {"ok": True}

        client = TestClient(app)
        client.get("/documents/private-id?query=secret")
        client.get("/unknown-private-path")
        response = client.get("/metrics")
        self.assertEqual(response.status_code, 200)
        text = response.text
        self.assertIn('route="/documents/{document_id}"', text)
        self.assertIn('route="unmatched"', text)
        self.assertNotIn("private-id", text)
        self.assertNotIn("secret", text)
        self.assertNotIn('route="/metrics"', text)

    def test_missing_cost_is_not_reported_as_free(self):
        model = "test-missing-cost"
        record_usage(model, "search", {"prompt_tokens": 10, "completion_tokens": 4})
        self.assertIsNone(REGISTRY.get_sample_value("ra_llm_reported_cost_dollars_total", {"model": model, "feature": "search"}))
        self.assertEqual(REGISTRY.get_sample_value("ra_llm_usage_reports_total", {"model": model, "feature": "search", "kind": "missing_cost"}), 1)

    def test_malformed_usage_does_not_interrupt_response(self):
        record_usage("test-invalid-usage", "search", {"prompt_tokens": 10, "completion_tokens": 4, "prompt_tokens_details": "invalid", "cost": float("nan")})
        record_usage("test-invalid-usage", "search", "invalid")


if __name__ == "__main__":
    unittest.main()

