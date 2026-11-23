package ops

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/db"
)

// Import the companies and contacts from a zip file to a database.
func Import(ctx context.Context, db *db.DB, path string) (err error) {
	exporter := &ZipExporter{path: path}
	importer := &DatabaseImporter{db: db}
	return Transfer(ctx, importer, exporter)
}

type Importer interface {
	Open(ctx context.Context, errc <-chan error) error
	Companies(<-chan *db.Company) error
	Contacts(<-chan *db.Contact) error
	PrimaryContacts(<-chan *db.PrimaryContact) error
	Close() error
	Cancel() error
}

type BaseImporter struct {
	ctx  context.Context
	errc <-chan error
}

func (i *BaseImporter) Check() (err error) {
	select {
	case err = <-i.errc:
		if err != nil {
			return err
		}
	case <-i.ctx.Done():
		return i.ctx.Err()
	default:
		break
	}
	return nil
}

//============================================================================
// Database Importer
//============================================================================

type DatabaseImporter struct {
	BaseImporter
	db            *db.DB
	tx            *db.Tx
	insertCompany *sql.Stmt
	insertContact *sql.Stmt
	updateCompany *sql.Stmt
}

const (
	insertCompanySQL = `INSERT INTO companies
	(id, lei, domain, name, website, country, business_category, vasp_categories, ivms101, verified_on, created, modified) VALUES
	(:id, :lei, :domain, :name, :website, :country, :business_category, :vasp_categories, :ivms101, :verified_on, :created, :modified)`

	insertContactSQL = `INSERT INTO contacts
	(id, name, email, role, company_id, ivms101, created, modified) VALUES
	(:id, :name, :email, :role, :company_id, :ivms101, :created, :modified)`

	updateCompanySQL = `UPDATE companies SET primary_contact_id = :contact_id WHERE id = :company_id`
)

func (i *DatabaseImporter) Open(ctx context.Context, errc <-chan error) (err error) {
	if i.db == nil {
		return errors.New("no database connection available")
	}

	if i.tx != nil {
		i.tx.Rollback()
		i.tx = nil
	}

	if i.tx, err = i.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: false, Isolation: sql.LevelSerializable}); err != nil {
		return err
	}

	if i.insertCompany, err = i.tx.Prepare(insertCompanySQL); err != nil {
		i.tx.Rollback()
		return err
	}

	if i.insertContact, err = i.tx.Prepare(insertContactSQL); err != nil {
		i.tx.Rollback()
		i.insertCompany.Close()
		return err
	}

	if i.updateCompany, err = i.tx.Prepare(updateCompanySQL); err != nil {
		i.tx.Rollback()
		i.insertContact.Close()
		i.insertCompany.Close()
		return err
	}

	i.ctx = ctx
	i.errc = errc
	return nil
}

func (i *DatabaseImporter) Close() (err error) {
	if i.tx != nil {
		err = errors.Join(err, i.tx.Commit())
		i.tx = nil
	}

	if i.insertCompany != nil {
		err = errors.Join(err, i.insertCompany.Close())
		i.insertCompany = nil
	}

	if i.insertContact != nil {
		err = errors.Join(err, i.insertContact.Close())
		i.insertContact = nil
	}

	if i.updateCompany != nil {
		err = errors.Join(err, i.updateCompany.Close())
		i.updateCompany = nil
	}

	return err
}

func (i *DatabaseImporter) Cancel() (err error) {
	if i.tx != nil {
		err = errors.Join(err, i.tx.Rollback())
		i.tx = nil
	}
	if i.insertCompany != nil {
		err = errors.Join(err, i.insertCompany.Close())
		i.insertCompany = nil
	}

	if i.insertContact != nil {
		err = errors.Join(err, i.insertContact.Close())
		i.insertContact = nil
	}

	if i.updateCompany != nil {
		err = errors.Join(err, i.updateCompany.Close())
		i.updateCompany = nil
	}
	return err
}

func (i *DatabaseImporter) Companies(companies <-chan *db.Company) error {
	for company := range companies {
		params := []any{
			sql.Named("id", company.ID),
			sql.Named("lei", company.LEI),
			sql.Named("domain", company.Domain),
			sql.Named("name", company.Name),
			sql.Named("website", company.Website),
			sql.Named("country", company.Country),
			sql.Named("business_category", company.BusinessCategory),
			sql.Named("vasp_categories", company.VASPCategories),
			sql.Named("ivms101", company.IVMS101),
			sql.Named("verified_on", company.VerifiedOn),
			sql.Named("created", company.Created),
			sql.Named("modified", company.Modified),
		}

		if _, err := i.insertCompany.ExecContext(i.ctx, params...); err != nil {
			return err
		}
	}
	return i.Check()
}

func (i *DatabaseImporter) Contacts(contacts <-chan *db.Contact) error {
	for contact := range contacts {
		params := []any{
			sql.Named("id", contact.ID),
			sql.Named("name", contact.Name),
			sql.Named("email", contact.Email),
			sql.Named("role", contact.Role),
			sql.Named("company_id", contact.CompanyID),
			sql.Named("ivms101", contact.IVMS101),
			sql.Named("created", contact.Created),
			sql.Named("modified", contact.Modified),
		}
		if _, err := i.insertContact.ExecContext(i.ctx, params...); err != nil {
			return err
		}
	}
	return i.Check()
}

// Because of the circular foreign key constraint on the companies and contacts table,
// we must do a second pass to update the companies table with the primary contact ID.
func (i *DatabaseImporter) PrimaryContacts(primaryContacts <-chan *db.PrimaryContact) error {
	for primaryContact := range primaryContacts {
		params := []any{
			sql.Named("company_id", primaryContact.CompanyID),
			sql.Named("contact_id", primaryContact.ContactID),
		}
		if _, err := i.updateCompany.ExecContext(i.ctx, params...); err != nil {
			return err
		}
	}
	return i.Check()
}

//============================================================================
// Zip Importer
//============================================================================

type ZipImporter struct {
	BaseImporter
	path string
	f    *os.File
	zw   *zip.Writer
}

func (i *ZipImporter) Open(ctx context.Context, errc <-chan error) (err error) {
	if i.f, err = os.Create(i.path); err != nil {
		return err
	}

	i.zw = zip.NewWriter(i.f)
	i.ctx = ctx
	i.errc = errc
	return nil
}

func (i *ZipImporter) Close() (err error) {
	if i.zw != nil {
		err = errors.Join(err, i.zw.Close())
		i.zw = nil
	}

	if i.f != nil {
		err = errors.Join(err, i.f.Close())
		i.f = nil
	}

	return err
}

func (i *ZipImporter) Cancel() (err error) {
	err = i.Close()
	os.Remove(i.path)
	return err
}

func (i *ZipImporter) Companies(companies <-chan *db.Company) (err error) {
	var f io.Writer
	if f, err = i.zw.Create(companiesFile); err != nil {
		return err
	}

	encoder := json.NewEncoder(f)
	for company := range companies {
		if err = encoder.Encode(api.NewCompany(company)); err != nil {
			return err
		}
	}
	return i.Check()
}

func (i *ZipImporter) Contacts(contacts <-chan *db.Contact) (err error) {
	var f io.Writer
	if f, err = i.zw.Create(contactsFile); err != nil {
		return err
	}

	encoder := json.NewEncoder(f)
	for contact := range contacts {
		if err = encoder.Encode(api.NewContact(contact)); err != nil {
			return err
		}
	}
	return i.Check()
}

// For a zip import the primary contacts are ignored since they are stored in the
// Company file as part of the company JSON data.
func (i *ZipImporter) PrimaryContacts(primaryContacts <-chan *db.PrimaryContact) (err error) {
	var f io.Writer
	if f, err = i.zw.Create(primaryContactsFile); err != nil {
		return err
	}

	encoder := json.NewEncoder(f)
	for primaryContact := range primaryContacts {
		if err = encoder.Encode(primaryContact); err != nil {
			return err
		}
	}
	return i.Check()
}
