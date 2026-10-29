-- Метаданные действий: содержимое запросов и ответов в аналитику не попадает.
BEGIN;

CREATE TABLE analytics_settings (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    started_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO analytics_settings(singleton) VALUES (true);

CREATE TABLE analytics_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action_id uuid NOT NULL,
    session_id uuid,
    project_id uuid,
    paper_id uuid,
    result_id uuid,
    source_action_id uuid,
    event_type varchar(32) NOT NULL CHECK (event_type IN (
        'action_started', 'action_finished', 'result_viewed', 'project_opened',
        'paper_opened', 'paper_saved', 'note_created', 'result_copied', 'feedback'
    )),
    producer varchar(8) NOT NULL CHECK (producer IN ('server', 'client')),
    feature varchar(24) NOT NULL DEFAULT '' CHECK (feature IN ('', 'summary', 'translation', 'chat', 'explanation', 'search', 'deep_research')),
    outcome varchar(16) NOT NULL DEFAULT '' CHECK (outcome IN ('', 'success', 'failed', 'cancelled', 'waiting')),
    cache_hit boolean NOT NULL DEFAULT false,
    locale varchar(8) NOT NULL DEFAULT 'unknown' CHECK (locale IN ('ru', 'en', 'unknown')),
    device varchar(8) NOT NULL DEFAULT 'unknown' CHECK (device IN ('mobile', 'desktop', 'unknown')),
    source varchar(16) NOT NULL DEFAULT 'other' CHECK (source IN ('feed', 'search', 'library', 'reader', 'other')),
    feedback smallint CHECK (feedback IN (-1, 1)),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    received_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(user_id, action_id, event_type)
);
CREATE INDEX analytics_events_activity ON analytics_events(user_id, occurred_at);
CREATE INDEX analytics_events_period ON analytics_events(occurred_at, event_type, feature);
CREATE INDEX analytics_events_project ON analytics_events(project_id, occurred_at) WHERE project_id IS NOT NULL;
CREATE INDEX analytics_events_result ON analytics_events(user_id, result_id) WHERE result_id IS NOT NULL;

-- История проектов сохраняется после удаления папки; удаление пользователя удаляет её.
CREATE TABLE analytics_projects (
    project_id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL
);
INSERT INTO analytics_projects(project_id,user_id,created_at)
SELECT id,user_id,created_at FROM library_folders WHERE parent_id IS NULL AND system_key IS NULL;

COMMIT;
