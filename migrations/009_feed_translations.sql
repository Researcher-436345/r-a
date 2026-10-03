-- Общий перевод описания не зависит от пользователя и не требует импорта статьи в библиотеку.
CREATE TABLE feed_abstract_translations (
    arxiv_id text NOT NULL,
    source_hash text NOT NULL,
    target_lang varchar(8) NOT NULL,
    source_text text NOT NULL,
    translated_text text NOT NULL CHECK (length(trim(translated_text)) > 0),
    model text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (arxiv_id, source_hash, target_lang)
);

-- Публикуем только полностью подготовленную ленту, сохраняя предыдущую при сбоях API.
CREATE TABLE feed_snapshots (
    category varchar(64) NOT NULL,
    sort varchar(16) NOT NULL,
    lang varchar(8) NOT NULL,
    items jsonb NOT NULL,
    prepared_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (category, sort, lang)
);
