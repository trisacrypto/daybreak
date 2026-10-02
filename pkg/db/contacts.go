package db

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
)

type Contact struct {
	ID        uuid.UUID
	Name      string
	Email     string
	Role      sql.NullString
	CompanyID uuid.UUID
	IVMS101   fields.NullJSONB
	Created   fields.Timestamp
	Modified  fields.Timestamp
}

func (c *Contact) Scan(scanner Scanner) error {
	return scanner.Scan(
		&c.ID,
		&c.Name,
		&c.Email,
		&c.Role,
		&c.CompanyID,
		&c.IVMS101,
		&c.Created,
		&c.Modified,
	)
}

//============================================================================
// Transaction CRUD
//============================================================================

const listContactsSQL = `SELECT * FROM contacts WHERE company_id=:companyID;`

func (tx *Tx) ListContacts(companyID uuid.UUID) (contacts *Iterator[*Contact], err error) {
	var rows *sql.Rows
	if rows, err = tx.Query(listContactsSQL, sql.Named("companyID", companyID)); err != nil {
		return nil, err
	}
	return Iterate[*Contact](rows), nil
}

const createContactSQL = `INSERT INTO contacts
	(id, name, email, role, company_id, ivms101, created, modified) VALUES
	(:id, :name, :email, :role, :companyID, :ivms101, :created, :modified);
`

func (tx *Tx) CreateContact(contact *Contact) (err error) {
	contact.ID = uuid.New()
	contact.Created = fields.TimeNow()
	contact.Modified = contact.Created

	params := []sql.NamedArg{
		{Name: "id", Value: contact.ID},
		{Name: "name", Value: contact.Name},
		{Name: "email", Value: contact.Email},
		{Name: "role", Value: contact.Role},
		{Name: "companyID", Value: contact.CompanyID},
		{Name: "ivms101", Value: contact.IVMS101},
		{Name: "created", Value: contact.Created},
		{Name: "modified", Value: contact.Modified},
	}

	if _, err = tx.Exec(createContactSQL, params...); err != nil {
		return err
	}
	return nil
}

const retrieveContactSQL = `SELECT * FROM contacts WHERE id=:id;`

func (tx *Tx) RetrieveContact(id uuid.UUID) (contact *Contact, err error) {
	contact = &Contact{}
	if err = contact.Scan(tx.QueryRow(retrieveContactSQL, sql.Named("id", id))); err != nil {
		return nil, err
	}
	return contact, nil
}

const updateContactSQL = `UPDATE contacts SET
name=:name, email=:email, role=:role, company_id=:companyID, ivms101=:ivms101, modified=:modified
WHERE id=:id;`

func (tx *Tx) UpdateContact(contact *Contact) (err error) {
	contact.Modified = fields.TimeNow()
	params := []sql.NamedArg{
		{Name: "id", Value: contact.ID},
		{Name: "name", Value: contact.Name},
		{Name: "email", Value: contact.Email},
		{Name: "role", Value: contact.Role},
		{Name: "companyID", Value: contact.CompanyID},
		{Name: "ivms101", Value: contact.IVMS101},
		{Name: "modified", Value: contact.Modified},
	}

	if _, err = tx.Exec(updateContactSQL, params...); err != nil {
		return err
	}
	return nil
}

const deleteContactSQL = `DELETE FROM contacts WHERE id=:id;`

func (tx *Tx) DeleteContact(id uuid.UUID) (err error) {
	if _, err = tx.Exec(deleteContactSQL, sql.Named("id", id)); err != nil {
		return err
	}
	return nil
}

//============================================================================
// Database CRUD
//============================================================================

func (db *DB) ListContacts(ctx context.Context, companyID uuid.UUID) (_ []*Contact, err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: true}); err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var iter *Iterator[*Contact]
	if iter, err = tx.ListContacts(companyID); err != nil {
		return nil, err
	}
	return iter.List()
}

func (db *DB) CreateContact(ctx context.Context, contact *Contact) (err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return err
	}
	defer tx.Rollback()

	if err = tx.CreateContact(contact); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) RetrieveContact(ctx context.Context, id uuid.UUID) (contact *Contact, err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: true}); err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return tx.RetrieveContact(id)
}

func (db *DB) UpdateContact(ctx context.Context, contact *Contact) (err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return err
	}
	defer tx.Rollback()

	if err = tx.UpdateContact(contact); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) DeleteContact(ctx context.Context, id uuid.UUID) (err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return err
	}
	defer tx.Rollback()

	if err = tx.DeleteContact(id); err != nil {
		return err
	}
	return tx.Commit()
}
