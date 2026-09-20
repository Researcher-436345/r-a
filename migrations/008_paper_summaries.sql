CREATE TABLE paper_summaries (
    paper_id uuid NOT NULL REFERENCES papers(id) ON DELETE CASCADE,
    lang varchar(8) NOT NULL,
    version_id uuid REFERENCES paper_versions(id) ON DELETE SET NULL,
    model varchar(128) NOT NULL DEFAULT '',
    content text NOT NULL DEFAULT '',
    status varchar(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'ready', 'failed')),
    error_message text NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (paper_id, lang)
);

CREATE INDEX ix_paper_summaries_status ON paper_summaries (status, updated_at);
