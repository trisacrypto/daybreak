package db

import (
	"context"
	"database/sql"

	"github.com/trisacrypto/daybreak/pkg/db/fields"
	"github.com/trisacrypto/daybreak/pkg/errors"
)

const daybreakID int64 = 1

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
