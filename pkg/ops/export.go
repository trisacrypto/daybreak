package ops

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"io/fs"

	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
)

// Export the companies and contacts from a database to a zip file.
func Export(ctx context.Context, db *db.DB, path string) (err error) {
	exporter := &DatabaseExporter{db: db}
	importer := &ZipImporter{path: path}
	return Transfer(ctx, importer, exporter)
}

type Exporter interface {
	Open(ctx context.Context, errc chan<- error) error
	Companies() (<-chan *db.Company, error)
	Contacts() (<-chan *db.Contact, error)
	PrimaryContacts() (<-chan *db.PrimaryContact, error)
	Close() error
	Cancel() error
}

//============================================================================
// Database Exporter
//============================================================================

type DatabaseExporter struct {
	db   *db.DB
	tx   *db.Tx
	ctx  context.Context
	errc chan<- error
}

func (e *DatabaseExporter) Open(ctx context.Context, errc chan<- error) (err error) {
	if e.db == nil {
		return errors.New("no database connection available")
	}

	if e.tx != nil {
		e.tx.Rollback()
		e.tx = nil
	}

	if e.tx, err = e.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true}); err != nil {
		return err
	}

	e.ctx = ctx
	e.errc = errc
	return nil
}

func (e *DatabaseExporter) Close() (err error) {
	if e.tx != nil {
		err = e.tx.Rollback()
		e.tx = nil
	}
	return err
}

func (e *DatabaseExporter) Cancel() (err error) {
	return e.Close()
}

func (e *DatabaseExporter) Companies() (_ <-chan *db.Company, err error) {
	// Get the iterator for the companies.
	var companies *db.Iterator[db.Company, *db.Company]
	if companies, err = e.tx.ListCompanies(); err != nil {
		return nil, err
	}

	// Create the channel then execute the goroutine to send the companies to the channel.
	out := make(chan *db.Company, 1)
	go dbExport(e.ctx, out, e.errc, companies)

	return out, nil
}

func (e *DatabaseExporter) Contacts() (_ <-chan *db.Contact, err error) {
	// Get the iterator for the contacts.
	var contacts *db.Iterator[db.Contact, *db.Contact]
	if contacts, err = e.tx.ListContacts(); err != nil {
		return nil, err
	}

	// Create the channel then execute the goroutine to send the contacts to the channel.
	out := make(chan *db.Contact, 1)
	go dbExport(e.ctx, out, e.errc, contacts)

	return out, nil
}

func (e *DatabaseExporter) PrimaryContacts() (_ <-chan *db.PrimaryContact, err error) {
	var contacts *db.Iterator[db.PrimaryContact, *db.PrimaryContact]
	if contacts, err = e.tx.ListPrimaryContacts(); err != nil {
		return nil, err
	}

	out := make(chan *db.PrimaryContact, 1)
	go dbExport(e.ctx, out, e.errc, contacts)

	return out, nil
}

func dbExport[T any, M db.Model[T]](ctx context.Context, C chan<- M, E chan<- error, iter *db.Iterator[T, M]) {
	defer close(C)
	defer iter.Close()

	for iter.Next() {
		model, err := iter.Model()
		if err != nil {
			E <- err
			return
		}

		select {
		case <-ctx.Done():
			return
		case C <- model:
			continue
		}
	}

	if err := iter.Err(); err != nil {
		E <- err
	}
}

//============================================================================
// Zip Exporter
//============================================================================

const (
	companiesFile       = "companies.json"
	contactsFile        = "contacts.json"
	primaryContactsFile = "primaries.json"
)

type ZipExporter struct {
	path string
	zr   *zip.ReadCloser
	ctx  context.Context
	errc chan<- error
}

func (e *ZipExporter) Open(ctx context.Context, errc chan<- error) (err error) {
	if e.zr, err = zip.OpenReader(e.path); err != nil {
		return err
	}

	e.ctx = ctx
	e.errc = errc
	return nil
}

func (e *ZipExporter) Close() (err error) {
	if e.zr != nil {
		err = e.zr.Close()
		e.zr = nil
	}
	return err
}

func (e *ZipExporter) Cancel() (err error) {
	return e.Close()
}

func (e *ZipExporter) Companies() (_ <-chan *db.Company, err error) {
	var f fs.File
	if f, err = e.zr.Open(companiesFile); err != nil {
		return nil, err
	}

	out := make(chan *db.Company, 1)
	go jsonDecode(e.ctx, out, e.errc, f, companyDecoder)

	return out, nil
}

func (e *ZipExporter) Contacts() (_ <-chan *db.Contact, err error) {
	var f fs.File
	if f, err = e.zr.Open(contactsFile); err != nil {
		return nil, err
	}

	out := make(chan *db.Contact, 1)
	go jsonDecode(e.ctx, out, e.errc, f, contactDecoder)

	return out, nil
}

func (e *ZipExporter) PrimaryContacts() (_ <-chan *db.PrimaryContact, err error) {
	var f fs.File
	if f, err = e.zr.Open(primaryContactsFile); err != nil {
		return nil, err
	}

	out := make(chan *db.PrimaryContact, 1)
	go jsonDecode(e.ctx, out, e.errc, f, primaryContactDecoder)

	return out, nil
}

func jsonDecode[T any, M db.Model[T]](ctx context.Context, C chan<- M, E chan<- error, r io.ReadCloser, decoder func(decoder *json.Decoder) func() (M, error)) {
	defer close(C)
	defer r.Close()

	decode := decoder(json.NewDecoder(r))
	for {
		model, err := decode()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				E <- err
			}
			return
		}

		select {
		case <-ctx.Done():
			return
		case C <- model:
			continue
		}
	}
}

func companyDecoder(decoder *json.Decoder) func() (*db.Company, error) {
	return func() (company *db.Company, err error) {
		dt := new(api.Company)
		if err = decoder.Decode(dt); err != nil {
			return nil, err
		}

		company = dt.Model()
		company.ID = dt.ID
		company.Created = fields.Time(dt.Created)
		company.Modified = fields.Time(dt.Modified)
		return company, nil
	}
}

func contactDecoder(decoder *json.Decoder) func() (*db.Contact, error) {
	return func() (contact *db.Contact, err error) {
		dt := new(api.Contact)
		if err = decoder.Decode(dt); err != nil {
			return nil, err
		}

		contact = dt.Model()
		contact.ID = dt.ID
		contact.Created = fields.Time(dt.Created)
		contact.Modified = fields.Time(dt.Modified)
		return contact, nil
	}
}

func primaryContactDecoder(decoder *json.Decoder) func() (*db.PrimaryContact, error) {
	return func() (primaryContact *db.PrimaryContact, err error) {
		dt := new(db.PrimaryContact)
		if err = decoder.Decode(dt); err != nil {
			return nil, err
		}
		return dt, nil
	}
}
