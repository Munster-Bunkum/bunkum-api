CREATE TABLE IF NOT EXISTS private_messages (
    id           BIGSERIAL    PRIMARY KEY,
    sender_id    BIGINT       NOT NULL REFERENCES users(id),
    recipient_id BIGINT       NOT NULL REFERENCES users(id),
    body         TEXT         NOT NULL,
    read_at      TIMESTAMPTZ,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_private_messages_recipient ON private_messages(recipient_id);
CREATE INDEX idx_private_messages_sender    ON private_messages(sender_id);
