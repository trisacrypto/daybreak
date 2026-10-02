package db

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
)

type Company struct {
	ID               uuid.UUID
	LEI              sql.NullString
	Domain           string
	Name             string
	Website          string
	Country          string
	BusinessCategory sql.NullString
	VASPCategories   fields.StringArray
	IVMS101          fields.NullJSONB
	PrimaryContact   uuid.NullUUID
	VerifiedOn       fields.Timestamp
	Created          fields.Timestamp
	Modified         fields.Timestamp
}

func (c *Company) Scan(scanner Scanner) error {
	return scanner.Scan(
		&c.ID,
		&c.LEI,
		&c.Domain,
		&c.Name,
		&c.Website,
		&c.Country,
		&c.BusinessCategory,
		&c.VASPCategories,
		&c.IVMS101,
		&c.PrimaryContact,
		&c.VerifiedOn,
		&c.Created,
		&c.Modified,
	)
}

//============================================================================
// Transaction CRUD
//============================================================================

const listCompaniesSQL = `SELECT * FROM companies;`

func (tx *Tx) ListCompanies() (companies *Iterator[*Company], err error) {
	var rows *sql.Rows
	if rows, err = tx.Query(listCompaniesSQL); err != nil {
		return nil, err
	}
	return Iterate[*Company](rows), nil
}

const createCompanySQL = `INSERT INTO companies
	(id, lei, domain, name, website, country, business_category, vasp_categories, ivms101, primary_contact_id, verified_on, created, modified) VALUES
	(:id, :lei, :domain, :name, :website, :country, :businessCategory, :vaspCategories, :ivms101, :primaryContactID, :verifiedOn, :created, :modified);
`

func (tx *Tx) CreateCompany(company *Company) (err error) {
	if company.ID == uuid.Nil {
		company.ID = uuid.New()
	}

	company.Created = fields.TimeNow()
	company.Modified = company.Created

	params := []sql.NamedArg{
		{Name: "id", Value: company.ID},
		{Name: "lei", Value: company.LEI},
		{Name: "domain", Value: company.Domain},
		{Name: "name", Value: company.Name},
		{Name: "website", Value: company.Website},
		{Name: "country", Value: company.Country},
		{Name: "businessCategory", Value: company.BusinessCategory},
		{Name: "vaspCategories", Value: company.VASPCategories},
		{Name: "ivms101", Value: company.IVMS101},
		{Name: "primaryContactID", Value: company.PrimaryContact},
		{Name: "verifiedOn", Value: company.VerifiedOn},
		{Name: "created", Value: company.Created},
		{Name: "modified", Value: company.Modified},
	}

	if _, err = tx.Exec(createCompanySQL, params...); err != nil {
		return err
	}
	return nil
}

const retrieveCompanySQL = `SELECT * FROM companies WHERE id=:id;`

func (tx *Tx) RetrieveCompany(id uuid.UUID) (company *Company, err error) {
	company = &Company{}
	if err = company.Scan(tx.QueryRow(retrieveCompanySQL, sql.Named("id", id))); err != nil {
		return nil, err
	}
	return company, nil
}

const updateCompanySQL = `UPDATE companies SET
lei=:lei, domain=:domain, name=:name, website=:website, country=:country, business_category=:businessCategory, vasp_categories=:vaspCategories, ivms101=:ivms101, primary_contact_id=:primaryContactID, verified_on=:verifiedOn, modified=:modified
WHERE id=:id;`

func (tx *Tx) UpdateCompany(company *Company) (err error) {
	company.Modified = fields.TimeNow()

	params := []sql.NamedArg{
		{Name: "id", Value: company.ID},
		{Name: "lei", Value: company.LEI},
		{Name: "domain", Value: company.Domain},
		{Name: "name", Value: company.Name},
		{Name: "website", Value: company.Website},
		{Name: "country", Value: company.Country},
		{Name: "businessCategory", Value: company.BusinessCategory},
		{Name: "vaspCategories", Value: company.VASPCategories},
		{Name: "ivms101", Value: company.IVMS101},
		{Name: "primaryContactID", Value: company.PrimaryContact},
		{Name: "verifiedOn", Value: company.VerifiedOn},
		{Name: "modified", Value: company.Modified},
	}

	if _, err = tx.Exec(updateCompanySQL, params...); err != nil {
		return err
	}
	return nil
}

const deleteCompanySQL = `DELETE FROM companies WHERE id=:id;`

func (tx *Tx) DeleteCompany(id uuid.UUID) (err error) {
	if _, err = tx.Exec(deleteCompanySQL, sql.Named("id", id)); err != nil {
		return err
	}
	return nil
}

//============================================================================
// Database CRUD
//============================================================================

func (db *DB) ListCompanies(ctx context.Context) (_ []*Company, err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: true}); err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var iter *Iterator[*Company]
	if iter, err = tx.ListCompanies(); err != nil {
		return nil, err
	}

	return iter.List()
}

func (db *DB) CreateCompany(ctx context.Context, company *Company) (err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return err
	}
	defer tx.Rollback()

	if err = tx.CreateCompany(company); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) RetrieveCompany(ctx context.Context, id uuid.UUID) (company *Company, err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: true}); err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return tx.RetrieveCompany(id)
}

func (db *DB) UpdateCompany(ctx context.Context, company *Company) (err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return err
	}
	defer tx.Rollback()

	if err = tx.UpdateCompany(company); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) DeleteCompany(ctx context.Context, id uuid.UUID) (err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return err
	}
	defer tx.Rollback()

	if err = tx.DeleteCompany(id); err != nil {
		return err
	}
	return tx.Commit()
}
