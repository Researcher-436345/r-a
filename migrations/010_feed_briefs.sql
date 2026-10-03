-- Короткие обзоры хранятся отдельно от полных переводов аннотации.
CREATE TABLE feed_briefs (
    arxiv_id text NOT NULL,
    source_hash text NOT NULL,
    source_text text NOT NULL,
    brief_text text NOT NULL CHECK (length(trim(brief_text)) BETWEEN 20 AND 280),
    model text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (arxiv_id, source_hash)
);

CREATE TABLE feed_brief_snapshots (
    category varchar(64) NOT NULL,
    sort varchar(16) NOT NULL,
    lang varchar(8) NOT NULL,
    items jsonb NOT NULL,
    prepared_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (category, sort, lang)
);
