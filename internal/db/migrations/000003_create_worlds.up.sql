CREATE TABLE worlds (
    name        TEXT PRIMARY KEY,
    width       INT NOT NULL DEFAULT 101,
    height      INT NOT NULL DEFAULT 51,
    -- Stores the full world payload: cells, drops, config, and any future keys.
    -- Extensible: add new top-level keys without schema changes.
    data        JSONB NOT NULL DEFAULT '{"cells":[],"drops":[],"config":{}}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
