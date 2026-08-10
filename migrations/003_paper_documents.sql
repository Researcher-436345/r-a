CREATE TABLE paper_documents (
    paper_id uuid NOT NULL REFERENCES papers(id) ON DELETE CASCADE,
    version_id uuid NOT NULL REFERENCES paper_versions(id) ON DELETE CASCADE,
    engine varchar(32) NOT NULL,
    ocr_used boolean NOT NULL DEFAULT false,
    page_count int NOT NULL DEFAULT 0,
    markdown text NOT NULL DEFAULT '',
    plain_text text NOT NULL DEFAULT '',
    status varchar(32) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'ready', 'failed')),
    error_message text NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (paper_id)
);

CREATE INDEX ix_paper_documents_version ON paper_documents (version_id);
CREATE INDEX ix_paper_documents_status ON paper_documents (status);

CREATE TABLE paper_chunks (
    id uuid PRIMARY KEY,
    paper_id uuid NOT NULL REFERENCES papers(id) ON DELETE CASCADE,
    version_id uuid NOT NULL REFERENCES paper_versions(id) ON DELETE CASCADE,
    chunk_index int NOT NULL,
    page_start int NOT NULL,
    page_end int NOT NULL,
    section text NULL,
    text text NOT NULL,
    token_estimate int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (paper_id, chunk_index)
);

CREATE INDEX ix_paper_chunks_paper ON paper_chunks (paper_id, chunk_index);

CREATE TABLE chat_thread_summaries (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    paper_id uuid NOT NULL REFERENCES papers(id) ON DELETE CASCADE,
    summary text NOT NULL,
    covered_message_id uuid NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, paper_id)
);
