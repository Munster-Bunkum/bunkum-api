CREATE TABLE IF NOT EXISTS users (
    id              BIGSERIAL    PRIMARY KEY,
    username        VARCHAR      NOT NULL UNIQUE,
    email           VARCHAR      NOT NULL UNIQUE,
    password_digest VARCHAR      NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
