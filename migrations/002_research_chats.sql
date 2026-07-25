CREATE TABLE research_chats (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title varchar(500) NOT NULL,
    mode varchar(16) NOT NULL CHECK (mode IN ('web', 'deep')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_research_chats_user_updated
    ON research_chats (user_id, updated_at DESC);

CREATE TABLE research_chat_messages (
    id uuid PRIMARY KEY,
    chat_id uuid NOT NULL REFERENCES research_chats(id) ON DELETE CASCADE,
    role varchar(16) NOT NULL CHECK (role IN ('user', 'assistant')),
    content text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_research_chat_messages_chat_created
    ON research_chat_messages (chat_id, created_at, id);
