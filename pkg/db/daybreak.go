package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
	"github.com/trisacrypto/daybreak/pkg/errors"
)

const daybreakID int64 = 1

type Modified struct {
	ID       uuid.UUID
	Modified fields.Timestamp
}

func (m *Modified) Scan(scanner Scanner) error {
	return scanner.Scan(
		&m.ID,
		&m.Modified,
	)
}

//============================================================================
// Transactions CRUD
//============================================================================

const lastModifiedSQL = `SELECT modified FROM daybreak WHERE id=:id;`

func (tx *Tx) LastModified() (modified fields.Timestamp, err error) {
	if err = tx.QueryRow(lastModifiedSQL, sql.Named("id", daybreakID)).Scan(&modified); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return modified, nil
		}
		return modified, err
	}
	return modified, nil
}

func (tx *Tx) LastModifiedHTTP() (_ time.Time, err error) {
	var modified fields.Timestamp
	if modified, err = tx.LastModified(); err != nil {
		return time.Time{}, err
	}
	return modified.Time().Truncate(time.Second).UTC(), nil
}

const updateModifiedSQL = `INSERT INTO daybreak (id, modified) VALUES (:id, :modified) ON CONFLICT(id) DO UPDATE SET modified=:modified;`

func (tx *Tx) UpdateModified() (err error) {
	params := []sql.NamedArg{
		sql.Named("id", daybreakID),
		sql.Named("modified", fields.TimeNow()),
	}

	if _, err = tx.Exec(updateModifiedSQL, params...); err != nil {
		return err
	}
	return nil
}

const (
	allModifiedCompaniesSQL = `SELECT id, modified FROM companies;`
	modifiedCompaniesSQL    = `SELECT id, modified FROM companies WHERE modified > :since;`
)

func (tx *Tx) ListModifiedCompanies(since time.Time) (iter *Iterator[Modified, *Modified], err error) {
	var rows *sql.Rows
	if since.IsZero() {
		rows, err = tx.Query(allModifiedCompaniesSQL)
	} else {
		rows, err = tx.Query(modifiedCompaniesSQL, sql.Named("since", fields.Time(since)))
	}

	if err != nil {
		return nil, err
	}

	iter = Iterate[Modified](rows)
	return iter, nil
}

const (
	allModifiedContactsSQL = `SELECT id, modified FROM contacts;`
	modifiedContactsSQL    = `SELECT id, modified FROM contacts WHERE modified > :since;`
)

func (tx *Tx) ListModifiedContacts(since time.Time) (iter *Iterator[Modified, *Modified], err error) {
	var rows *sql.Rows
	if since.IsZero() {
		rows, err = tx.Query(allModifiedContactsSQL)
	} else {
		rows, err = tx.Query(modifiedContactsSQL, sql.Named("since", fields.Time(since)))
	}

	if err != nil {
		return nil, err
	}

	iter = Iterate[Modified](rows)
	return iter, nil
}

//============================================================================
// Database Functions
//============================================================================

func (db *DB) LastModified(ctx context.Context) (modified fields.Timestamp, err error) {
	var tx *Tx
	if tx, err = db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true}); err != nil {
		return modified, err
	}
	defer tx.Rollback()

	return tx.LastModified()
}

func (db *DB) UpdateModified(ctx context.Context) (err error) {
	var tx *Tx
	if tx, err = db.BeginTx(ctx, &sql.TxOptions{ReadOnly: false, Isolation: sql.LevelSerializable}); err != nil {
		return err
	}
	defer tx.Rollback()

	if err = tx.UpdateModified(); err != nil {
		return err
	}

	return tx.Commit()
}

func (db *DB) LastModifiedHTTP(ctx context.Context) (_ time.Time, err error) {
	var tx *Tx
	if tx, err = db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true}); err != nil {
		return time.Time{}, err
	}
	defer tx.Rollback()

	return tx.LastModifiedHTTP()
}
