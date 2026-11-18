package db_test

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
	"github.com/trisacrypto/daybreak/pkg/errors"
)

func (s *Suite) TestCompanies() {
	s.Require().NoError(s.LoadFixtures(), "could not load fixtures")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// The company created in CreateCompany is shared by the subtests that follow it,
	// along with the last modified timestamp of the database that each subtest
	// compares against and then advances.
	var lastModified fields.Timestamp
	company := &db.Company{
		LEI:              sql.NullString{String: "5493001KJTIIGC8Y1R12", Valid: true},
		Domain:           "kestrelvault.example.com",
		Name:             "Kestrel Vault Inc.",
		Website:          "https://kestrelvault.example.com",
		Country:          "US",
		BusinessCategory: sql.NullString{String: "BUSINESS_ENTITY", Valid: true},
		VASPCategories:   fields.StringArray{"Exchange", "Custodian"},
	}

	// Timestamps are truncated to milliseconds, so wait long enough between writes
	// to guarantee that a changed timestamp is strictly after the previous one.
	tick := func() { time.Sleep(2 * time.Millisecond) }

	s.Run("ListCompanies", func() {
		require := s.Require()

		companies, err := s.db.ListCompanies(ctx)
		require.NoError(err)
		require.Len(companies, 16)
	})

	s.Run("RecentCompanies", func() {
		require := s.Require()

		companies, err := s.db.RecentCompanies(ctx, 4)
		require.NoError(err)
		require.Len(companies, 4)

		// The fixtures ordered by most recently modified.
		expected := []uuid.UUID{
			uuid.MustParse("3a0e747e-a93a-4c40-b8bb-1992c7adea88"),
			uuid.MustParse("6cf40352-2fa3-4988-88f3-ee7101a9b704"),
			uuid.MustParse("ff69b2df-b153-47ec-980e-93e48209a9a4"),
			uuid.MustParse("f6c88bde-f581-430f-b354-ae88d414b8d3"),
		}

		for i, company := range companies {
			require.Equal(expected[i], company.ID, "unexpected company at index %d", i)
		}
	})

	// NOTE: if create company fails all of the subtests that follow it will fail.
	s.Run("CreateCompany", func() {
		require := s.Require()

		// Loading fixtures does not touch the daybreak table.
		modified, err := s.db.LastModified(ctx)
		require.NoError(err)
		require.True(modified.IsZero())

		err = s.db.CreateCompany(ctx, company)
		require.NoError(err)
		require.NotEqual(uuid.Nil, company.ID)
		require.False(company.Created.IsZero())
		require.True(company.Modified.Equal(company.Created))

		lastModified, err = s.db.LastModified(ctx)
		require.NoError(err)
		require.False(lastModified.IsZero(), "expected create to update last modified")
	})

	s.Run("RetrieveCompany", func() {
		require := s.Require()

		actual, err := s.db.RetrieveCompany(ctx, company.ID)
		require.NoError(err)

		require.Equal(company.ID, actual.ID)
		require.Equal(company.LEI, actual.LEI)
		require.Equal(company.Domain, actual.Domain)
		require.Equal(company.Name, actual.Name)
		require.Equal(company.Website, actual.Website)
		require.Equal(company.Country, actual.Country)
		require.Equal(company.BusinessCategory, actual.BusinessCategory)
		require.True(company.VASPCategories.Equal(actual.VASPCategories))
		require.Zero(actual.NumContacts)
		require.False(actual.IVMS101.Valid)
		require.False(actual.PrimaryContact.Valid)
		require.True(actual.VerifiedOn.IsZero())
		require.True(company.Created.Equal(actual.Created))
		require.True(company.Modified.Equal(actual.Modified))

		modified, err := s.db.LastModified(ctx)
		require.NoError(err)
		require.True(lastModified.Equal(modified), "expected retrieve to not change last modified")
	})

	s.Run("UpdateCompany", func() {
		require := s.Require()
		tick()

		created, previous := company.Created, company.Modified
		company.Name = "Kestrel Vault Holdings Inc."
		company.Country = "CA"

		err := s.db.UpdateCompany(ctx, company)
		require.NoError(err)
		require.True(company.Modified.After(previous))

		actual, err := s.db.RetrieveCompany(ctx, company.ID)
		require.NoError(err)
		require.Equal("Kestrel Vault Holdings Inc.", actual.Name)
		require.Equal("CA", actual.Country)
		require.True(created.Equal(actual.Created))
		require.True(company.Modified.Equal(actual.Modified))

		modified, err := s.db.LastModified(ctx)
		require.NoError(err)
		require.True(modified.After(lastModified), "expected update to change last modified")
		lastModified = modified
	})

	s.Run("VerifyCompany", func() {
		require := s.Require()
		tick()

		previous := company.Modified
		require.True(company.VerifiedOn.IsZero())

		err := s.db.VerifyCompany(ctx, company)
		require.NoError(err)
		require.False(company.VerifiedOn.IsZero())
		require.True(company.Modified.After(previous))

		actual, err := s.db.RetrieveCompany(ctx, company.ID)
		require.NoError(err)
		require.True(company.VerifiedOn.Equal(actual.VerifiedOn))
		require.True(company.Modified.Equal(actual.Modified))

		modified, err := s.db.LastModified(ctx)
		require.NoError(err)
		require.True(modified.After(lastModified), "expected verify to change last modified")
		lastModified = modified
	})

	s.Run("DeleteCompany", func() {
		require := s.Require()
		tick()

		err := s.db.DeleteCompany(ctx, company.ID)
		require.NoError(err)

		_, err = s.db.RetrieveCompany(ctx, company.ID)
		require.ErrorIs(err, errors.ErrNotFound)

		modified, err := s.db.LastModified(ctx)
		require.NoError(err)
		require.True(modified.After(lastModified), "expected delete to change last modified")

		// The company should be the only record in the deleted companies table.
		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		require.NoError(err)
		defer tx.Rollback()

		count, err := s.Count(tx.Tx, "deleted_companies")
		require.NoError(err)
		require.Equal(1, count)

		var deleted fields.Timestamp
		err = tx.QueryRow("SELECT modified FROM deleted_companies WHERE id=:id", sql.Named("id", company.ID)).Scan(&deleted)
		require.NoError(err, "expected company to be in the deleted companies table")
		require.False(deleted.IsZero())
	})
}
