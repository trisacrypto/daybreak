package api

import (
	"database/sql"
	"encoding/json"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
)

type GDSRecord struct {
	Source              string          `json:"source"`
	DirectoryID         uuid.UUID       `json:"directory_id"`
	RegisteredDirectory string          `json:"registered_directory"`
	Protocol            string          `json:"protocol"`
	CommonName          string          `json:"common_name"`
	Endpoint            string          `json:"endpoint"`
	Name                string          `json:"name"`
	Website             string          `json:"website"`
	Country             string          `json:"country"`
	BusinessCategory    string          `json:"business_category"`
	VASPCategories      []string        `json:"vasp_categories"`
	VerifiedOn          time.Time       `json:"verified_on"`
	IVMSRecord          json.RawMessage `json:"ivms_record"`
	LEI                 string          `json:"lei"`
	Contacts            []*GDSContact   `json:"contacts"`
}

type GDSContact struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func Fixture(path string) (records []*GDSRecord, err error) {
	var file *os.File
	if file, err = os.Open(path); err != nil {
		return nil, err
	}
	defer file.Close()

	records = make([]*GDSRecord, 0)
	if err = json.NewDecoder(file).Decode(&records); err != nil {
		return nil, err
	}
	return records, nil
}

func (g *GDSRecord) Model() *db.Company {
	return &db.Company{
		ID:               g.DirectoryID,
		LEI:              sql.NullString{String: g.LEI, Valid: g.LEI != ""},
		Domain:           g.CommonName,
		Name:             g.Name,
		Website:          g.Website,
		Country:          g.Country,
		BusinessCategory: sql.NullString{String: g.BusinessCategory, Valid: g.BusinessCategory != ""},
		VASPCategories:   g.VASPCategories,
		IVMS101:          fields.NullJSONB{JSONB: fields.JSONB(g.IVMSRecord), Valid: g.IVMSRecord != nil},
		VerifiedOn:       fields.Time(g.VerifiedOn),
	}
}

func (g *GDSContact) Model() *db.Contact {
	return &db.Contact{
		Name:  g.Name,
		Email: g.Email,
		Role:  sql.NullString{String: g.Role, Valid: g.Role != ""},
	}
}
