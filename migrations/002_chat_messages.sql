CREATE TABLE chat_messages (
    id uuid PRIMARY KEY,
    paper_id uuid NOT NULL REFERENCES papers(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role varchar(16) NOT NULL CHECK (role IN ('user', 'assistant')),
    content text NOT NULL,
    context_text text NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_chat_messages_user_paper_created
    ON chat_messages (user_id, paper_id, created_at, id);
