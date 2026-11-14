CREATE TABLE search_runs (
    id uuid PRIMARY KEY REFERENCES search_chat_messages(id) ON DELETE CASCADE,
    chat_id uuid NOT NULL REFERENCES search_chats(id) ON DELETE CASCADE,
    provider_response_id text,
    status varchar(16) NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'completed', 'failed')),
    assistant_message_id uuid REFERENCES search_chat_messages(id) ON DELETE SET NULL,
    error_message text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ix_search_runs_active_chat ON search_runs (chat_id) WHERE status = 'running';
CREATE INDEX ix_search_runs_chat_created ON search_runs (chat_id, created_at DESC);
