-- Initial schema for Daybreak sunrise email database
-- NOTE: all primary keys are UUIDs to match the GDS data model. In this schema we're
-- using the UUID string value rather than a 16-byte blob to make database queries
-- easier and because the sqlite3 storage backend isn't performance sensitive.


CREATE TABLE IF NOT EXISTS companies (
    id                  TEXT PRIMARY KEY,
    lei                 TEXT DEFAULT NULL,
    domain              TEXT NOT NULL UNIQUE,
    name                TEXT NOT NULL,
    website             TEXT NOT NULL,
    country             TEXT NOT NULL,
    business_category   TEXT DEFAULT NULL,
    vasp_categories     BLOB DEFAULT '[]',
    ivms101             BLOB DEFAULT NULL,
    verified_on         DATETIME DEFAULT NULL,
    created             DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    modified            DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE IF NOT EXISTS contacts (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    email       TEXT NOT NULL UNIQUE,
    role        TEXT DEFAULT NULL,
    company_id  TEXT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    ivms101     BLOB DEFAULT NULL,
    created     DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    modified    DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

ALTER TABLE companies ADD COLUMN primary_contact_id TEXT DEFAULT NULL REFERENCES contacts(id) ON DELETE SET NULL;