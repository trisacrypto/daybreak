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

func (s *Suite) TestContacts() {
	s.Require().NoError(s.LoadFixtures(), "could not load fixtures")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	companyID := uuid.MustParse("2cece8e8-4b2d-4d31-a871-124438a0d81b")

	// The contact created in CreateContact is shared by the subtests that follow it,
	// along with the modified timestamp of its company and the last modified
	// timestamp of the database that each subtest compares against and then advances.
	var companyModified, lastModified fields.Timestamp
	contact := &db.Contact{
		Name:      "Genevieve Lachance",
		Email:     "genevieve.lachance@mapleridgecustody.ca",
		Role:      sql.NullString{String: "Chief Compliance Officer", Valid: true},
		CompanyID: companyID,
	}

	// Returns the current modified timestamp of the company and of the database.
	timestamps := func() (company, database fields.Timestamp) {
		require := s.Require()

		record, err := s.db.RetrieveCompany(ctx, companyID)
		require.NoError(err)

		database, err = s.db.LastModified(ctx)
		require.NoError(err)

		return record.Modified, database
	}

	// Timestamps are truncated to milliseconds, so wait long enough between writes
	// to guarantee that a changed timestamp is strictly after the previous one.
	tick := func() { time.Sleep(2 * time.Millisecond) }

	s.Run("ListContacts", func() {
		require := s.Require()

		contacts, err := s.db.ListContacts(ctx)
		require.NoError(err)
		require.Len(contacts, 28)
	})

	s.Run("ListCompanyContacts", func() {
		require := s.Require()
		companyID := uuid.MustParse("6cf40352-2fa3-4988-88f3-ee7101a9b704")

		contacts, err := s.db.ListCompanyContacts(ctx, companyID)
		require.NoError(err)
		require.Len(contacts, 4)

		for _, contact := range contacts {
			require.Equal(companyID, contact.CompanyID)
		}
		require.Equal(uuid.MustParse("4bc26eeb-0aec-4440-8fc7-1f1869422f16"), contacts[0].ID)
	})

	// NOTE: if create contact fails all of the subtests that follow it will fail.
	s.Run("CreateContact", func() {
		require := s.Require()

		// Loading fixtures does not touch the daybreak table.
		previous, modified := timestamps()
		require.False(previous.IsZero())
		require.True(modified.IsZero())

		err := s.db.CreateContact(ctx, contact)
		require.NoError(err)
		require.NotEqual(uuid.Nil, contact.ID)
		require.False(contact.Created.IsZero())
		require.True(contact.Modified.Equal(contact.Created))

		companyModified, lastModified = timestamps()
		require.True(companyModified.After(previous), "expected create to change company modified")
		require.True(companyModified.Equal(contact.Modified))
		require.False(lastModified.IsZero(), "expected create to update last modified")

		// A contact with the same email address cannot be created, even at another company.
		duplicate := &db.Contact{
			Name:      "Genevieve L.",
			Email:     contact.Email,
			CompanyID: uuid.MustParse("60c362fb-3ddd-4fa3-9be8-5a73bd46748c"),
		}

		err = s.db.CreateContact(ctx, duplicate)
		require.ErrorIs(err, errors.ErrAlreadyExists)
	})

	s.Run("RetrieveContact", func() {
		require := s.Require()

		actual, err := s.db.RetrieveContact(ctx, contact.ID)
		require.NoError(err)

		require.Equal(contact.ID, actual.ID)
		require.Equal(contact.Name, actual.Name)
		require.Equal(contact.Email, actual.Email)
		require.Equal(contact.Role, actual.Role)
		require.Equal(contact.CompanyID, actual.CompanyID)
		require.False(actual.IVMS101.Valid)
		require.True(contact.Created.Equal(actual.Created))
		require.True(contact.Modified.Equal(actual.Modified))

		company, modified := timestamps()
		require.True(companyModified.Equal(company), "expected retrieve to not change company modified")
		require.True(lastModified.Equal(modified), "expected retrieve to not change last modified")
	})

	s.Run("UpdateContact", func() {
		require := s.Require()
		tick()

		created, previous := contact.Created, contact.Modified
		contact.Name = "Genevieve Lachance-Roy"
		contact.Role = sql.NullString{String: "Chief Anti-Money Laundering Officer", Valid: true}

		err := s.db.UpdateContact(ctx, contact)
		require.NoError(err)
		require.True(contact.Modified.After(previous))

		actual, err := s.db.RetrieveContact(ctx, contact.ID)
		require.NoError(err)
		require.Equal("Genevieve Lachance-Roy", actual.Name)
		require.Equal("Chief Anti-Money Laundering Officer", actual.Role.String)
		require.True(created.Equal(actual.Created))
		require.True(contact.Modified.Equal(actual.Modified))

		company, modified := timestamps()
		require.True(company.After(companyModified), "expected update to change company modified")
		require.True(company.Equal(contact.Modified))
		require.True(modified.After(lastModified), "expected update to change last modified")
		companyModified, lastModified = company, modified
	})

	s.Run("DeleteContact", func() {
		require := s.Require()
		tick()

		err := s.db.DeleteContact(ctx, contact.ID)
		require.NoError(err)

		_, err = s.db.RetrieveContact(ctx, contact.ID)
		require.ErrorIs(err, errors.ErrNotFound)

		company, modified := timestamps()
		require.True(company.After(companyModified), "expected delete to change company modified")
		require.True(modified.After(lastModified), "expected delete to change last modified")
	})
}
