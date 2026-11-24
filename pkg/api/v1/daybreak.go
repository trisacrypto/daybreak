package api

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/db"
)

type Modified struct {
	ID       uuid.UUID `json:"id"`
	Modified time.Time `json:"modified"`
}

type SyncResponse struct {
	LastModified     time.Time   `json:"last_modified"`     // The last modified time of the database
	Companies        []*Modified `json:"companies"`         // Insert or update these companies
	DeletedCompanies []*Modified `json:"deleted_companies"` // Delete these companies
	Contacts         []*Modified `json:"contacts"`          // Insert or update these contacts
	DeletedContacts  []*Modified `json:"deleted_contacts"`  // Delete these contacts
}

func NewModified(model any) *Modified {
	switch m := model.(type) {
	case *db.Company:
		return &Modified{
			ID:       m.ID,
			Modified: m.Modified.Time(),
		}
	case *db.Contact:
		return &Modified{
			ID:       m.ID,
			Modified: m.Modified.Time(),
		}
	case *db.Modified:
		return &Modified{
			ID:       m.ID,
			Modified: m.Modified.Time(),
		}
	case *db.DeletedCompany:
		return &Modified{
			ID:       m.ID,
			Modified: m.Modified,
		}
	case *db.DeletedContact:
		return &Modified{
			ID:       m.ID,
			Modified: m.Modified,
		}
	default:
		panic(fmt.Errorf("unknown model type: %T", model))
	}
}

func NewModifiedList[T any, M db.Model[T]](iter *db.Iterator[T, M]) (_ []*Modified, err error) {
	defer iter.Close()

	var list []*Modified
	for iter.Next() {
		var model M
		if model, err = iter.Model(); err != nil {
			return nil, err
		}
		list = append(list, NewModified(model))
	}

	if err = iter.Err(); err != nil {
		return nil, err
	}
	return list, nil
}
