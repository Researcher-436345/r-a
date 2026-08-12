CREATE TABLE library_folders (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id uuid REFERENCES library_folders(id) ON DELETE CASCADE,
    name varchar(255) NOT NULL,
    system_key varchar(32),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_library_folders_name CHECK (length(btrim(name)) > 0),
    CONSTRAINT ck_library_folders_system_key CHECK (
        system_key IS NULL OR system_key IN ('want_to_read', 'reading', 'other')
    )
);

CREATE UNIQUE INDEX uq_library_folders_system
    ON library_folders (user_id, system_key)
    WHERE system_key IS NOT NULL;

CREATE UNIQUE INDEX uq_library_folders_sibling_name
    ON library_folders (user_id, parent_id, lower(name)) NULLS NOT DISTINCT;

CREATE INDEX ix_library_folders_user_parent
    ON library_folders (user_id, parent_id, created_at);

ALTER TABLE user_library_items
    ADD COLUMN folder_id uuid REFERENCES library_folders(id) ON DELETE SET NULL;

CREATE INDEX ix_user_library_items_folder
    ON user_library_items (user_id, folder_id, added_at DESC);
