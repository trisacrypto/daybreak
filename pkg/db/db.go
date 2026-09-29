package db

import (
	"context"
	"database/sql"
	"os"
	"time"

	"github.com/trisacrypto/daybreak/pkg/errors"
	"go.rtnl.ai/x/dsn"

	modernc "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	defaultDBTimeout = 600 * time.Second
)

// Opens a new database connection using the provided DSN.
func Open(uri *dsn.DSN) (_ *DB, err error) {
	// Ensure that only sqlite3 connections can be opened.
	if uri.Provider != dsn.SQLite && uri.Provider != dsn.SQLite3 {
		return nil, errors.Join(errors.ErrInvalidDSN, errors.Fmt("unknown database scheme: %s", uri.Provider))
	}

	// Only pure-Go sqlite3 drivers are supported.
	if uri.Driver != "" && uri.Driver != dsn.ModernC {
		return nil, errors.Join(errors.ErrInvalidDSN, errors.Fmt("unknown driver: %s", uri.Driver))
	}

	// Require a path in order to open the database connection (no in-memory databases).
	if uri.Path == "" {
		return nil, errors.ErrInvalidDSN
	}

	// Check if the database file exists, if it doesn't exist, it will be created and
	// all migrations will be applied to the database. Otherwise only the migrations
	// that have not been applied will be run.
	empty := false
	if _, err = os.Stat(uri.Path); os.IsNotExist(err) {
		empty = true
	}

	// Ensure the timezone is set to UTC.
	if _, ok := uri.Get("_timezone"); !ok {
		uri.Set("_timezone", "UTC")
	}

	// Open the database connection.
	db := &DB{uri: uri, readonly: uri.ReadOnly()}
	if db.DB, err = sql.Open(dsn.SQLite, uri.FileURI()); err != nil {
		return nil, err
	}

	// Ping the database to establish the connection
	if err = db.DB.Ping(); err != nil {
		return nil, err
	}

	// Ensure that foreign key support is turned on by executing PRAGMA query.
	if _, err = db.DB.Exec("PRAGMA foreign_keys = on;"); err != nil {
		return nil, errors.Join(errors.ErrSQLiteForeignKeys, err)
	}

	// Ensure the schema is initialized.
	if err = db.InitializeSchema(empty); err != nil {
		return nil, err
	}

	// Set the database to readonly mode after initializing the schema.
	if uri.ReadOnly() {
		if _, err = db.DB.Exec("PRAGMA query_only = on;"); err != nil {
			return nil, errors.Join(errors.ErrSQLiteQueryOnly, err)
		}
	}

	return db, nil
}

// Implements the Store interface using sqlite3 as the storage backend.
// NOTE: we're using a pure-Go sqlite3 driver without CGO.
type DB struct {
	*sql.DB
	uri      *dsn.DSN
	readonly bool
}

func (db *DB) Begin(ctx context.Context, opts *sql.TxOptions) (_ *Tx, err error) {
	return db.BeginTx(ctx, opts)
}

func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (_ *Tx, err error) {
	// Ensure the options respect the readonly mode of the store.
	if opts == nil {
		opts = &sql.TxOptions{ReadOnly: db.readonly}
	} else if db.readonly && !opts.ReadOnly {
		return nil, errors.ErrReadOnly
	}

	var tx *sql.Tx
	if tx, err = db.DB.BeginTx(ctx, opts); err != nil {
		return nil, err
	}

	return &Tx{Tx: tx, opts: opts}, nil
}

func (db *DB) Exec(query string, args ...sql.NamedArg) (result sql.Result, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultDBTimeout)
	defer cancel()

	return db.ExecContext(ctx, query, args...)
}

func (db *DB) ExecContext(ctx context.Context, query string, nargs ...sql.NamedArg) (result sql.Result, err error) {
	if result, err = db.DB.ExecContext(ctx, query, args(nargs)...); err != nil {
		return nil, dbe(err)
	}
	return result, nil
}

func (db *DB) Query(query string, args ...sql.NamedArg) (rows *sql.Rows, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultDBTimeout)
	defer cancel()

	return db.QueryContext(ctx, query, args...)
}

func (db *DB) QueryContext(ctx context.Context, query string, nargs ...sql.NamedArg) (rows *sql.Rows, err error) {
	if rows, err = db.DB.QueryContext(ctx, query, args(nargs)...); err != nil {
		return nil, dbe(err)
	}
	return rows, nil
}

func (db *DB) QueryRow(query string, args ...sql.NamedArg) *Row {
	ctx, cancel := context.WithTimeout(context.Background(), defaultDBTimeout)
	defer cancel()

	return db.QueryRowContext(ctx, query, args...)
}

func (db *DB) QueryRowContext(ctx context.Context, query string, nargs ...sql.NamedArg) *Row {
	row := db.DB.QueryRowContext(ctx, query, args(nargs)...)
	return &Row{Row: row}
}

// ===========================================================================
// Database Helpers
// ===========================================================================

// Converts a sqlite3 error into a standardized error.
func dbe(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return errors.Join(errors.ErrNotFound, err)
	}

	if sqliteErr, ok := err.(*modernc.Error); ok {
		switch sqliteErr.Code() {
		case sqlite3.SQLITE_READONLY:
			return errors.Join(errors.ErrReadOnly, err)
		case sqlite3.SQLITE_CONSTRAINT_UNIQUE:
			return errors.Join(errors.ErrAlreadyExists, err)
		}
	}

	return errors.Join(errors.ErrDatabase, err)
}
