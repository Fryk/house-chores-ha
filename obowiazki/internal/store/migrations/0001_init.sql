CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Tymczasowy dziennik M0: wysłane powiadomienia i kliknięcia akcji.
CREATE TABLE dev_event_log (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    at      TEXT NOT NULL,
    kind    TEXT NOT NULL,      -- notify | clear | action | error
    service TEXT NOT NULL DEFAULT '',
    tag     TEXT NOT NULL DEFAULT '',
    action  TEXT NOT NULL DEFAULT '',
    detail  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX dev_event_log_tag ON dev_event_log(tag);
