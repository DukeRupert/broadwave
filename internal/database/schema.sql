CREATE TABLE IF NOT EXISTS lists (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT,
    from_name   TEXT NOT NULL,
    from_email  TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS subscribers (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    email             TEXT NOT NULL,
    name              TEXT,
    status            TEXT NOT NULL DEFAULT 'pending',
    confirm_token     TEXT,
    unsubscribe_token TEXT NOT NULL,
    confirmed_at      TEXT,
    unsubscribed_at   TEXT,
    created_at        TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscribers_email ON subscribers(email);

CREATE TABLE IF NOT EXISTS list_subscribers (
    list_id       INTEGER NOT NULL REFERENCES lists(id),
    subscriber_id INTEGER NOT NULL REFERENCES subscribers(id),
    subscribed_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (list_id, subscriber_id)
);

CREATE TABLE IF NOT EXISTS messages (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    list_id     INTEGER NOT NULL REFERENCES lists(id),
    subject     TEXT NOT NULL,
    body_text   TEXT NOT NULL,
    body_html   TEXT,
    status      TEXT NOT NULL DEFAULT 'draft',
    sent_at     TEXT,
    sent_count  INTEGER DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS send_log (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id    INTEGER NOT NULL REFERENCES messages(id),
    subscriber_id INTEGER NOT NULL REFERENCES subscribers(id),
    status        TEXT NOT NULL DEFAULT 'queued',
    sent_at       TEXT,
    error         TEXT
);

CREATE TABLE IF NOT EXISTS api_keys (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    list_id    INTEGER NOT NULL REFERENCES lists(id),
    key_prefix TEXT NOT NULL,
    key_hash   TEXT NOT NULL,
    label      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    revoked_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_list_id ON api_keys(list_id);
