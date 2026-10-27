-- Тестовые аккаунты работают как обычные, но не входят в продуктовые показатели.
BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE users ADD COLUMN is_test boolean NOT NULL DEFAULT false;

CREATE VIEW analytics_product_users AS
SELECT id, created_at FROM users WHERE NOT is_test;

CREATE VIEW analytics_product_events AS
SELECT e.* FROM analytics_events e
JOIN analytics_product_users u ON u.id = e.user_id;

CREATE VIEW analytics_product_projects AS
SELECT p.* FROM analytics_projects p
JOIN analytics_product_users u ON u.id = p.user_id;

COMMIT;
