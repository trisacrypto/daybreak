 -- This migration handles the creation of the licenses table, which allows paid
 -- Envoy nodes to sync with the daybreak dataset. It also adds and modifies tables to
 -- make it easier to sync.

-- A single row table that holds the last modified timestamp for the daybreak dataset.
CREATE TABLE IF NOT EXISTS daybreak (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    modified DATETIME NOT NULL
);

-- Deleted companies are tracked here to allow syncs to identify records for removal.
CREATE TABLE IF NOT EXISTS deleted_companies (
    id          TEXT PRIMARY KEY,
    modified    DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- Creates licenses for users to access the daybreak dataset.
CREATE TABLE IF NOT EXISTS licenses (
    id          TEXT PRIMARY KEY,
    secret      BLOB NOT NULL CHECK (typeof(secret) = 'blob' AND length(secret) = 32),
    name        TEXT NOT NULL,
    email       TEXT NOT NULL,
    description TEXT DEFAULT NULL,
    permissions BLOB DEFAULT '[]',
    syncs       INTEGER NOT NULL DEFAULT 0,
    downloads   INTEGER NOT NULL DEFAULT 0,
    last_access DATETIME DEFAULT NULL,
    expires     DATETIME NOT NULL,
    created     DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    modified    DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
