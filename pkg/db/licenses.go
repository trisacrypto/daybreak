package db

import (
	"context"
	"crypto/rand"
	"database/sql"

	"github.com/trisacrypto/daybreak/pkg/db/fields"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"go.rtnl.ai/ulid"
)

type License struct {
	ID          ulid.ULID
	Secret      []byte
	Name        string
	Email       string
	Description sql.NullString
	Permissions fields.StringArray
	Syncs       int64
	Downloads   int64
	LastAccess  fields.Timestamp
	Expires     fields.Timestamp
	Created     fields.Timestamp
	Modified    fields.Timestamp
}

func (l *License) Scan(scanner Scanner) error {
	return scanner.Scan(
		&l.ID,
		&l.Secret,
		&l.Name,
		&l.Email,
		&l.Description,
		&l.Permissions,
		&l.Syncs,
		&l.Downloads,
		&l.LastAccess,
		&l.Expires,
		&l.Created,
		&l.Modified,
	)
}

//============================================================================
// Transactions CRUD
//============================================================================

const listLicensesSQL = `SELECT * FROM licenses;`

func (tx *Tx) ListLicenses() (licenses *Iterator[License, *License], err error) {
	var rows *sql.Rows
	if rows, err = tx.Query(listLicensesSQL); err != nil {
		return nil, err
	}
	return Iterate[License](rows), nil
}

const createLicenseSQL = `INSERT INTO licenses (id, secret, name, email, description, permissions, syncs, downloads, last_access, expires, created, modified) VALUES (:id, :secret, :name, :email, :description, :permissions, :syncs, :downloads, :last_access, :expires, :created, :modified);`

func (tx *Tx) CreateLicense(license *License) (err error) {
	if len(license.Secret) != 32 {
		if len(license.Secret) == 0 {
			license.Secret = make([]byte, 32)
			if _, err = rand.Read(license.Secret); err != nil {
				return err
			}
		} else {
			return errors.ErrInvalidLicenseSecret
		}
	}

	if license.ID.IsZero() {
		license.ID = ulid.Make()
	}

	license.Created = fields.TimeNow()
	license.Modified = license.Created

	params := []sql.NamedArg{
		{Name: "id", Value: license.ID},
		{Name: "secret", Value: license.Secret},
		{Name: "name", Value: license.Name},
		{Name: "email", Value: license.Email},
		{Name: "description", Value: license.Description},
		{Name: "permissions", Value: license.Permissions},
		{Name: "syncs", Value: license.Syncs},
		{Name: "downloads", Value: license.Downloads},
		{Name: "last_access", Value: license.LastAccess},
		{Name: "expires", Value: license.Expires},
		{Name: "created", Value: license.Created},
		{Name: "modified", Value: license.Modified},
	}

	if _, err = tx.Exec(createLicenseSQL, params...); err != nil {
		return err
	}

	return nil
}

const retrieveLicenseSQL = `SELECT * FROM licenses WHERE id=:id;`

func (tx *Tx) RetrieveLicense(id ulid.ULID) (license *License, err error) {
	license = &License{}
	if err = license.Scan(tx.QueryRow(retrieveLicenseSQL, sql.Named("id", id))); err != nil {
		return nil, err
	}
	return license, nil
}

const updateLicenseSQL = `UPDATE licenses SET name=:name, email=:email, description=:description, permissions=:permissions, syncs=:syncs, downloads=:downloads, last_access=:last_access, expires=:expires, modified=:modified WHERE id=:id;`

func (tx *Tx) UpdateLicense(license *License) (err error) {
	license.Modified = fields.TimeNow()

	params := []sql.NamedArg{
		{Name: "id", Value: license.ID},
		{Name: "name", Value: license.Name},
		{Name: "email", Value: license.Email},
		{Name: "description", Value: license.Description},
		{Name: "permissions", Value: license.Permissions},
		{Name: "syncs", Value: license.Syncs},
		{Name: "downloads", Value: license.Downloads},
		{Name: "last_access", Value: license.LastAccess},
		{Name: "expires", Value: license.Expires},
		{Name: "modified", Value: license.Modified},
	}

	var result sql.Result
	if result, err = tx.Exec(updateLicenseSQL, params...); err != nil {
		return err
	}

	if rowsAffected, err := result.RowsAffected(); err != nil || rowsAffected == 0 {
		if err != nil {
			return err
		}
		return errors.ErrNotFound
	}

	return nil
}

const deleteLicenseSQL = `DELETE FROM licenses WHERE id=:id;`

func (tx *Tx) DeleteLicense(id ulid.ULID) (err error) {
	var result sql.Result
	if result, err = tx.Exec(deleteLicenseSQL, sql.Named("id", id)); err != nil {
		return err
	}

	if rowsAffected, err := result.RowsAffected(); err != nil || rowsAffected == 0 {
		if err != nil {
			return err
		}
		return errors.ErrNotFound
	}

	return nil
}

//============================================================================
// Database Methods
//============================================================================

func (db *DB) ListLicenses(ctx context.Context) (licenses []*License, err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: true}); err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var iter *Iterator[License, *License]
	if iter, err = tx.ListLicenses(); err != nil {
		return nil, err
	}

	return iter.List()
}

func (db *DB) CreateLicense(ctx context.Context, license *License) (err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return err
	}
	defer tx.Rollback()

	if err = tx.CreateLicense(license); err != nil {
		return err
	}

	return tx.Commit()
}

func (db *DB) RetrieveLicense(ctx context.Context, id ulid.ULID) (license *License, err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: true}); err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if license, err = tx.RetrieveLicense(id); err != nil {
		return nil, err
	}

	return license, nil
}

func (db *DB) UpdateLicense(ctx context.Context, license *License) (err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return err
	}
	defer tx.Rollback()

	if err = tx.UpdateLicense(license); err != nil {
		return err
	}

	return tx.Commit()
}

func (db *DB) DeleteLicense(ctx context.Context, id ulid.ULID) (err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return err
	}
	defer tx.Rollback()

	if err = tx.DeleteLicense(id); err != nil {
		return err
	}

	return tx.Commit()
}
