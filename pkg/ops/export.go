package ops

import (
	"context"

	"github.com/trisacrypto/daybreak/pkg/api/v1"
)

type Exporter interface {
	Open(ctx context.Context) error
	Companies() (<-chan *api.Company, error)
	Contacts() (<-chan *api.Contact, error)
	DeletedCompanies() (<-chan *api.Company, error)
	DeletedContacts() (<-chan *api.Contact, error)
	Close() error
}
