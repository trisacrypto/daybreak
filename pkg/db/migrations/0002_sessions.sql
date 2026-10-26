-- Manages HTTP sessions for UI authentication and other stateful data.
CREATE TABLE IF NOT EXISTS http_sessions (
    id       TEXT PRIMARY KEY,
    data     TEXT NOT NULL,
    created  DATETIME NOT NULL,
    modified DATETIME NOT NULL,
    expires  DATETIME NOT NULL
);