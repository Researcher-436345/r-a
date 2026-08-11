ALTER TABLE annotations
    ADD COLUMN IF NOT EXISTS source_chat_message_id uuid NULL;

CREATE INDEX IF NOT EXISTS ix_annotations_source_chat_message
    ON annotations (source_chat_message_id)
    WHERE source_chat_message_id IS NOT NULL;
