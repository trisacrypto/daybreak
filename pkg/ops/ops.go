package ops

import (
	"context"

	"github.com/trisacrypto/daybreak/pkg/db"
)

// Transfers data from an exporter to an importer. The Import operation transfers
// data from a file exporter to a database importer. The Export operation transfers
// data from a database exporter to a file importer. The order of operations is:
//
// 1. Open the importer then the exporter
// 2. Export companies from the exporter to the importer
// 3. Export contacts from the exporter to the importer
// 4. Export deleted companies from the exporter to the importer
// 5. Export deleted contacts from the exporter to the importer
// 6. Close the exporter then the importer
func Transfer(ctx context.Context, importer Importer, exporter Exporter) (err error) {
	errc := make(chan error, 1)
	if err = importer.Open(ctx, errc); err != nil {
		return err
	}

	if err = exporter.Open(ctx, errc); err != nil {
		importer.Cancel()
		return err
	}

	defer func() {
		if err != nil {
			importer.Cancel()
			exporter.Cancel()
		} else {
			importer.Close()
			exporter.Close()
		}
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
