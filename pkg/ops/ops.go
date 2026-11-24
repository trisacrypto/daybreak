package ops

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/trisacrypto/daybreak/pkg/db"
)

// Transfer data from an exporter to an importer. The Import operation transfers
// data from a file exporter to a database importer. The Export operation transfers
// data from a database exporter to a file importer. The order of operations is:
//
//  1. Open the importer then the exporter with a shared cancelable context and
//     error channel; the exporter sends errors on the channel and the importer
//     receives them. If the exporter fails to open, the importer is canceled.
//  2. Transfer companies from the exporter to the importer
//  3. Transfer contacts from the exporter to the importer
//  4. Transfer primary contacts from the exporter to the importer
//  5. Close the exporter then the importer
//
// Each transfer step gets a channel of records from the exporter and passes it to
// the importer. If any step fails, including closing the exporter or the importer,
// the context is canceled, Cancel is called on the importer then the exporter, and
// the error that caused the failure is returned.
func Transfer(ctx context.Context, importer Importer, exporter Exporter) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errc := make(chan error, 1)
	if err = importer.Open(ctx, errc); err != nil {
		return err
	}

	if err = exporter.Open(ctx, errc); err != nil {
		importer.Cancel()
		return err
	}

	defer func() {
		if err == nil {
			if err = exporter.Close(); err == nil {
				if err = importer.Close(); err == nil {
					return
				}
			}
		}

		cancel()
		importer.Cancel()
		exporter.Cancel()
	}()

	var companies <-chan *db.Company
	if companies, err = exporter.Companies(); err != nil {
		return err
	}

	if err = importer.Companies(companies); err != nil {
		return err
	}

	var contacts <-chan *db.Contact
	if contacts, err = exporter.Contacts(); err != nil {
		return err
	}

	if err = importer.Contacts(contacts); err != nil {
		return err
	}

	var primaries <-chan *db.PrimaryContact
	if primaries, err = exporter.PrimaryContacts(); err != nil {
		return err
	}

	if err = importer.PrimaryContacts(primaries); err != nil {
		return err
	}

	return nil
}

// Download the zip file of the database to the response writer.
func Download(ctx context.Context, c *gin.Context, db *db.DB) (err error) {
	exporter := &DatabaseExporter{db: db}
	importer := &StreamImporter{w: c.Writer}
	return Transfer(ctx, importer, exporter)
}
