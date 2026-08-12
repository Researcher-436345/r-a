CREATE TABLE search_chats (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title varchar(500) NOT NULL,
    mode varchar(16) NOT NULL CHECK (mode IN ('web', 'deep')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_search_chats_user_updated
    ON search_chats (user_id, updated_at DESC);

CREATE TABLE search_chat_messages (
    id uuid PRIMARY KEY,
    chat_id uuid NOT NULL REFERENCES search_chats(id) ON DELETE CASCADE,
    role varchar(16) NOT NULL CHECK (role IN ('user', 'assistant')),
    content text NOT NULL,
    sources jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_search_chat_messages_chat_created
    ON search_chat_messages (chat_id, created_at, id);
