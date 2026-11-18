package db

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
	"github.com/trisacrypto/daybreak/pkg/errors"
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
	NumContacts      int64
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
		&c.NumContacts,
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

const listCompaniesSQL = `
SELECT c.id, c.lei, c.domain, c.name, c.website, c.country, c.business_category, c.vasp_categories, count(co.id) as num_contacts, c.ivms101, c.primary_contact_id, c.verified_on, c.created, c.modified
    FROM companies c
LEFT JOIN contacts co ON c.id = co.company_id
GROUP BY c.id;
`

func (tx *Tx) ListCompanies() (companies *Iterator[Company, *Company], err error) {
	var rows *sql.Rows
	if rows, err = tx.Query(listCompaniesSQL); err != nil {
		return nil, err
	}
	return Iterate[Company](rows), nil
}

const recentCompaniesSQL = `
SELECT c.id, c.lei, c.domain, c.name, c.website, c.country, c.business_category, c.vasp_categories, count(co.id) as num_contacts, c.ivms101, c.primary_contact_id, c.verified_on, c.created, c.modified
    FROM companies c
LEFT JOIN contacts co ON c.id = co.company_id
GROUP BY c.id
ORDER BY c.modified DESC
LIMIT :limit;
`

func (tx *Tx) RecentCompanies(limit int) (companies *Iterator[Company, *Company], err error) {
	var rows *sql.Rows
	if rows, err = tx.Query(recentCompaniesSQL, sql.Named("limit", limit)); err != nil {
		return nil, err
	}
	return Iterate[Company](rows), nil
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

	// When companies are modified, update the modified timestamp for the database.
	if err = tx.UpdateModified(); err != nil {
		return err
	}
	return nil
}

const retrieveCompanySQL = `
SELECT c.id, c.lei, c.domain, c.name, c.website, c.country, c.business_category, c.vasp_categories, count(co.id) as num_contacts, c.ivms101, c.primary_contact_id, c.verified_on, c.created, c.modified
    FROM companies c
LEFT JOIN contacts co ON c.id = co.company_id
WHERE c.id = :id
GROUP BY c.id;
`

func (tx *Tx) RetrieveCompany(id uuid.UUID) (company *Company, err error) {
	company = &Company{}
	if err = company.Scan(tx.QueryRow(retrieveCompanySQL, sql.Named("id", id))); err != nil {
		return nil, err
	}
	return company, nil
}

const updateCompanySQL = `UPDATE companies SET
lei=:lei, domain=:domain, name=:name, website=:website, country=:country, business_category=:businessCategory, vasp_categories=:vaspCategories, ivms101=:ivms101, primary_contact_id=:primaryContactID, modified=:modified
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
		{Name: "modified", Value: company.Modified},
	}

	var result sql.Result
	if result, err = tx.Exec(updateCompanySQL, params...); err != nil {
		return err
	}

	if rowsAffected, err := result.RowsAffected(); err != nil || rowsAffected == 0 {
		if err != nil {
			return err
		}
		return errors.ErrNotFound
	}

	// When companies are modified, update the modified timestamp for the database.
	if err = tx.UpdateModified(); err != nil {
		return err
	}
	return nil
}

const verifyCompanySQL = `UPDATE companies SET verified_on=:verifiedOn, modified=:modified WHERE id=:id;`

func (tx *Tx) VerifyCompany(company *Company) (err error) {
	company.VerifiedOn = fields.TimeNow()
	company.Modified = company.VerifiedOn

	params := []sql.NamedArg{
		{Name: "id", Value: company.ID},
		{Name: "verifiedOn", Value: company.VerifiedOn},
		{Name: "modified", Value: company.Modified},
	}

	var result sql.Result
	if result, err = tx.Exec(verifyCompanySQL, params...); err != nil {
		return err
	}

	if rowsAffected, err := result.RowsAffected(); err != nil || rowsAffected == 0 {
		if err != nil {
			return err
		}
		return errors.ErrNotFound
	}

	// When companies are modified, update the modified timestamp for the database.
	if err = tx.UpdateModified(); err != nil {
		return err
	}
	return nil
}

const (
	deleteCompanySQL = `DELETE FROM companies WHERE id=:id;`
	markDeletedSQL   = `INSERT INTO deleted_companies (id, modified) VALUES (:id, :modified);`
)

func (tx *Tx) DeleteCompany(id uuid.UUID) (err error) {
	var result sql.Result
	if result, err = tx.Exec(deleteCompanySQL, sql.Named("id", id)); err != nil {
		return err
	}

	if rowsAffected, err := result.RowsAffected(); err != nil || rowsAffected == 0 {
		if err != nil {
			return err
		}
		return errors.ErrNotFound
	}

	if _, err = tx.Exec(markDeletedSQL, sql.Named("id", id), sql.Named("modified", fields.TimeNow())); err != nil {
		return err
	}

	// When companies are modified, update the modified timestamp for the database.
	if err = tx.UpdateModified(); err != nil {
		return err
	}

	return nil
}

const companyModifiedSQL = `UPDATE companies SET modified=:modified WHERE id=:id;`

// Called by contacts to indicate that they've been modified and therefore the company
// has also been modified. This will also update the modified timestamp for the database
// so that the contacts table can trickle up the update timestamps for caches.
func (tx *Tx) CompanyModified(id uuid.UUID, modified fields.Timestamp) (err error) {
	if modified.IsZero() {
		modified = fields.TimeNow()
	}

	params := []sql.NamedArg{
		{Name: "id", Value: id},
		{Name: "modified", Value: modified},
	}

	if _, err = tx.Exec(companyModifiedSQL, params...); err != nil {
		return err
	}

	// When companies are modified, update the modified timestamp for the database.
	if err = tx.UpdateModified(); err != nil {
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

	var iter *Iterator[Company, *Company]
	if iter, err = tx.ListCompanies(); err != nil {
		return nil, err
	}

	return iter.List()
}

func (db *DB) RecentCompanies(ctx context.Context, limit int) (companies []*Company, err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: true}); err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var iter *Iterator[Company, *Company]
	if iter, err = tx.RecentCompanies(limit); err != nil {
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

func (db *DB) VerifyCompany(ctx context.Context, company *Company) (err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return err
	}
	defer tx.Rollback()

	if err = tx.VerifyCompany(company); err != nil {
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
