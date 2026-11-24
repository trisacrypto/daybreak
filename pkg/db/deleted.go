package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
)

type DeletedCompany struct {
	ID       uuid.UUID
	Modified time.Time
}

type DeletedContact struct {
	ID        uuid.UUID
	CompanyID uuid.UUID
	Modified  time.Time
}

func (c *DeletedCompany) Scan(scanner Scanner) error {
	return scanner.Scan(
		&c.ID,
		&c.Modified,
	)
}

func (c *DeletedContact) Scan(scanner Scanner) error {
	return scanner.Scan(
		&c.ID,
		&c.CompanyID,
		&c.Modified,
	)
}

//============================================================================
// Transaction Helpers
//============================================================================

const (
	listALLDeletedCompaniesSQL = `SELECT * FROM deleted_companies ORDER BY modified DESC`
	listDeletedCompaniesSQL    = `SELECT * FROM deleted_companies WHERE modified > :since ORDER BY modified DESC`
)

func (tx *Tx) ListDeletedCompanies(since time.Time) (companies *Iterator[DeletedCompany, *DeletedCompany], err error) {
	query := listALLDeletedCompaniesSQL
	params := make([]sql.NamedArg, 0, 1)
	if !since.IsZero() {
		query = listDeletedCompaniesSQL
		params = append(params, sql.Named("since", fields.Time(since)))
	}

	var rows *sql.Rows
	if rows, err = tx.Query(query, params...); err != nil {
		return nil, err
	}

	return Iterate[DeletedCompany](rows), nil
}

const (
	listALLDeletedContactsSQL        = `SELECT * FROM deleted_contacts ORDER BY modified DESC`
	listDeletedContactsSQL           = `SELECT * FROM deleted_contacts WHERE modified > :since ORDER BY modified DESC`
	listAllCompanyDeletedContactsSQL = `SELECT * FROM deleted_contacts WHERE company_id = :companyID ORDER BY modified DESC`
	listCompanyDeletedContactsSQL    = `SELECT * FROM deleted_contacts WHERE company_id = :companyID AND modified > :since ORDER BY modified DESC`
)

func (tx *Tx) ListDeletedContacts(since time.Time, companyID uuid.UUID) (contacts *Iterator[DeletedContact, *DeletedContact], err error) {
	query := listALLDeletedContactsSQL
	params := make([]sql.NamedArg, 0, 2)

	switch {
	case !since.IsZero() && companyID == uuid.Nil:
		query = listDeletedContactsSQL
		params = append(params, sql.Named("since", fields.Time(since)))
	case since.IsZero() && companyID != uuid.Nil:
		query = listAllCompanyDeletedContactsSQL
		params = append(params, sql.Named("companyID", companyID))
	case !since.IsZero() && companyID != uuid.Nil:
		query = listCompanyDeletedContactsSQL
		params = append(params, sql.Named("since", fields.Time(since)))
		params = append(params, sql.Named("companyID", companyID))
	}

	var rows *sql.Rows
	if rows, err = tx.Query(query, params...); err != nil {
		return nil, err
	}

	return Iterate[DeletedContact](rows), nil
}
