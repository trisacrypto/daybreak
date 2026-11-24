package db_test

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
)

// DeleteCompany and DeleteContact mark records as deleted at the current time, so to
// test the ordering and the since filter of the deleted lists, the tests insert the
// deleted records at fixed times with the same statements the delete methods use.
const (
	markCompanyDeletedSQL = `INSERT INTO deleted_companies (id, modified) VALUES (:id, :modified);`
	markContactDeletedSQL = `INSERT INTO deleted_contacts (id, company_id, modified) VALUES (:id, :companyID, :modified);`
)

func (s *Suite) TestDeletedCompanies() {
	s.Require().NoError(s.LoadFixtures(), "could not load fixtures")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// The deleted companies in the order that they were deleted: b and c were deleted
	// on the same day, and no company was deleted on March 3rd.
	var (
		a = &db.DeletedCompany{ID: uuid.MustParse("922ae51b-2f5c-41a3-8f5d-982b51e08211"), Modified: time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC)}
		b = &db.DeletedCompany{ID: uuid.MustParse("f6efe189-6cad-463f-b724-4850378bbace"), Modified: time.Date(2025, 3, 2, 8, 0, 0, 0, time.UTC)}
		c = &db.DeletedCompany{ID: uuid.MustParse("cb31ec40-2b18-4493-a059-c9b394c7273d"), Modified: time.Date(2025, 3, 2, 16, 30, 0, 250*int(time.Millisecond), time.UTC)}
		d = &db.DeletedCompany{ID: uuid.MustParse("31dc1b73-880b-4514-8638-e14d9a95693d"), Modified: time.Date(2025, 3, 4, 0, 0, 0, 0, time.UTC)}
	)

	list := func(since time.Time) []*db.DeletedCompany {
		require := s.Require()

		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		require.NoError(err)
		defer tx.Rollback()

		iter, err := tx.ListDeletedCompanies(since)
		require.NoError(err)

		companies, err := iter.List()
		require.NoError(err)
		return companies
	}

	// Checks that the actual companies are the expected companies in the same order.
	check := func(actual []*db.DeletedCompany, expected ...*db.DeletedCompany) {
		require := s.Require()

		ids := func(companies []*db.DeletedCompany) []string {
			ids := make([]string, 0, len(companies))
			for _, company := range companies {
				ids = append(ids, company.ID.String())
			}
			return ids
		}

		require.Equal(ids(expected), ids(actual), "unexpected deleted companies or ordering")
		for i, company := range actual {
			require.True(expected[i].Modified.Equal(company.Modified), "unexpected modified timestamp at index %d", i)
		}
	}

	s.Run("Empty", func() {
		check(list(time.Time{}))
		check(list(a.Modified))
	})

	// NOTE: if the deleted companies cannot be inserted all following subtests will fail.
	func() {
		require := s.Require()

		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: false})
		require.NoError(err)
		defer tx.Rollback()

		// Insert out of order so that the list cannot be sorted by insertion.
		for _, company := range []*db.DeletedCompany{c, a, d, b} {
			_, err = tx.Exec(markCompanyDeletedSQL, sql.Named("id", company.ID), sql.Named("modified", fields.Time(company.Modified)))
			require.NoError(err)
		}
		require.NoError(tx.Commit())
	}()

	testCases := []struct {
		name     string
		since    time.Time
		expected []*db.DeletedCompany
	}{
		{
			"All",
			time.Time{},
			[]*db.DeletedCompany{d, c, b, a},
		},
		{
			"SinceBeforeAll",
			time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			[]*db.DeletedCompany{d, c, b, a},
		},
		{
			"SinceOnDayWithoutDeletes",
			time.Date(2025, 3, 3, 12, 0, 0, 0, time.UTC),
			[]*db.DeletedCompany{d},
		},
		{
			// The same instant as 2025-03-03T02:00:00Z, which is after b and c.
			"SinceInAnotherTimezone",
			time.Date(2025, 3, 2, 20, 0, 0, 0, time.FixedZone("CST", -6*60*60)),
			[]*db.DeletedCompany{d},
		},
		{
			"SinceBetweenDeletesOnSameDay",
			time.Date(2025, 3, 2, 12, 0, 0, 0, time.UTC),
			[]*db.DeletedCompany{d, c},
		},
		{
			// A company deleted exactly at the since timestamp is not returned.
			"SinceIsExclusive",
			c.Modified,
			[]*db.DeletedCompany{d},
		},
		{
			"SinceAfterAll",
			time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC),
			nil,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			check(list(tc.since), tc.expected...)
		})
	}

	// A company deleted by DeleteCompany is listed and is the most recently deleted.
	s.Run("DeleteCompany", func() {
		require := s.Require()
		companyID := uuid.MustParse("2cece8e8-4b2d-4d31-a871-124438a0d81b")

		// Timestamps are truncated to milliseconds, so wait long enough to guarantee
		// that the company is deleted strictly after the before timestamp.
		before := fields.TimeNow().Time()
		time.Sleep(2 * time.Millisecond)

		err := s.db.DeleteCompany(ctx, companyID)
		require.NoError(err)

		companies := list(time.Time{})
		require.Len(companies, 5)
		require.Equal(companyID, companies[0].ID)
		require.True(companies[0].Modified.After(before), "expected the company to be deleted after the test started")
		require.False(companies[0].Modified.After(time.Now()), "expected the company to be deleted before now")
		check(companies[1:], d, c, b, a)

		// Only the company deleted by the test was deleted since the test started.
		check(list(before), companies[0])

		// When syncing from the most recent deletion, there is nothing new to return.
		check(list(companies[0].Modified))
	})
}

func (s *Suite) TestDeletedContacts() {
	s.Require().NoError(s.LoadFixtures(), "could not load fixtures")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Deleted contacts must reference a company, so use companies from the fixtures.
	var (
		alpha   = uuid.MustParse("2cece8e8-4b2d-4d31-a871-124438a0d81b")
		bravo   = uuid.MustParse("6cf40352-2fa3-4988-88f3-ee7101a9b704")
		charlie = uuid.MustParse("60c362fb-3ddd-4fa3-9be8-5a73bd46748c")
	)

	// The deleted contacts in the order that they were deleted: b, c, and d were
	// deleted on the same day, and no contact was deleted on March 3rd.
	var (
		a = &db.DeletedContact{ID: uuid.MustParse("864887ca-aab4-434a-b453-6dd16a136fe1"), CompanyID: alpha, Modified: time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC)}
		b = &db.DeletedContact{ID: uuid.MustParse("3beb5058-8498-4254-983a-8f0d40fb1a31"), CompanyID: bravo, Modified: time.Date(2025, 3, 2, 8, 0, 0, 0, time.UTC)}
		c = &db.DeletedContact{ID: uuid.MustParse("946c1ced-f366-445e-a7fe-8dd1501a1ce8"), CompanyID: alpha, Modified: time.Date(2025, 3, 2, 16, 30, 0, 250*int(time.Millisecond), time.UTC)}
		d = &db.DeletedContact{ID: uuid.MustParse("74657cc6-5882-4936-84b9-e126c94aa2c3"), CompanyID: bravo, Modified: time.Date(2025, 3, 2, 20, 0, 0, 0, time.UTC)}
		e = &db.DeletedContact{ID: uuid.MustParse("9f1be568-0154-44b2-91aa-19c62b48a9f5"), CompanyID: alpha, Modified: time.Date(2025, 3, 4, 0, 0, 0, 0, time.UTC)}
	)

	list := func(since time.Time, companyID uuid.UUID) []*db.DeletedContact {
		require := s.Require()

		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		require.NoError(err)
		defer tx.Rollback()

		iter, err := tx.ListDeletedContacts(since, companyID)
		require.NoError(err)

		contacts, err := iter.List()
		require.NoError(err)
		return contacts
	}

	// Checks that the actual contacts are the expected contacts in the same order.
	check := func(actual []*db.DeletedContact, expected ...*db.DeletedContact) {
		require := s.Require()

		ids := func(contacts []*db.DeletedContact) []string {
			ids := make([]string, 0, len(contacts))
			for _, contact := range contacts {
				ids = append(ids, contact.ID.String())
			}
			return ids
		}

		require.Equal(ids(expected), ids(actual), "unexpected deleted contacts or ordering")
		for i, contact := range actual {
			require.Equal(expected[i].CompanyID, contact.CompanyID, "unexpected company at index %d", i)
			require.True(expected[i].Modified.Equal(contact.Modified), "unexpected modified timestamp at index %d", i)
		}
	}

	s.Run("Empty", func() {
		check(list(time.Time{}, uuid.Nil))
		check(list(a.Modified, uuid.Nil))
		check(list(time.Time{}, alpha))
		check(list(a.Modified, alpha))
	})

	// NOTE: if the deleted contacts cannot be inserted all following subtests will fail.
	func() {
		require := s.Require()

		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: false})
		require.NoError(err)
		defer tx.Rollback()

		// Insert out of order so that the list cannot be sorted by insertion.
		for _, contact := range []*db.DeletedContact{c, e, a, d, b} {
			_, err = tx.Exec(markContactDeletedSQL, sql.Named("id", contact.ID), sql.Named("companyID", contact.CompanyID), sql.Named("modified", fields.Time(contact.Modified)))
			require.NoError(err)
		}
		require.NoError(tx.Commit())
	}()

	testCases := []struct {
		name      string
		since     time.Time
		companyID uuid.UUID
		expected  []*db.DeletedContact
	}{
		// No since timestamp and no company.
		{
			"All",
			time.Time{}, uuid.Nil,
			[]*db.DeletedContact{e, d, c, b, a},
		},
		// Since timestamp only.
		{
			"SinceBeforeAll",
			time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC), uuid.Nil,
			[]*db.DeletedContact{e, d, c, b, a},
		},
		{
			"SinceOnDayWithoutDeletes",
			time.Date(2025, 3, 3, 12, 0, 0, 0, time.UTC), uuid.Nil,
			[]*db.DeletedContact{e},
		},
		{
			// The same instant as 2025-03-03T02:00:00Z, which is after b, c, and d.
			"SinceInAnotherTimezone",
			time.Date(2025, 3, 2, 20, 0, 0, 0, time.FixedZone("CST", -6*60*60)), uuid.Nil,
			[]*db.DeletedContact{e},
		},
		{
			"SinceBetweenDeletesOnSameDay",
			time.Date(2025, 3, 2, 12, 0, 0, 0, time.UTC), uuid.Nil,
			[]*db.DeletedContact{e, d, c},
		},
		{
			// A contact deleted exactly at the since timestamp is not returned.
			"SinceIsExclusive",
			d.Modified, uuid.Nil,
			[]*db.DeletedContact{e},
		},
		{
			"SinceAfterAll",
			time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC), uuid.Nil,
			nil,
		},
		// Company only.
		{
			"Company",
			time.Time{}, alpha,
			[]*db.DeletedContact{e, c, a},
		},
		{
			"OtherCompany",
			time.Time{}, bravo,
			[]*db.DeletedContact{d, b},
		},
		{
			"CompanyWithoutDeletes",
			time.Time{}, charlie,
			nil,
		},
		{
			"UnknownCompany",
			time.Time{}, uuid.MustParse("0b8a5a5c-7d6e-4a4f-9d4b-5a1e1f0c2b3d"),
			nil,
		},
		// Company and since timestamp.
		{
			"CompanySinceBeforeAll",
			time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC), alpha,
			[]*db.DeletedContact{e, c, a},
		},
		{
			"CompanySinceOnDayWithoutDeletes",
			time.Date(2025, 3, 3, 12, 0, 0, 0, time.UTC), alpha,
			[]*db.DeletedContact{e},
		},
		{
			// The other company has deleted contacts, but none since the timestamp.
			"OtherCompanySinceOnDayWithoutDeletes",
			time.Date(2025, 3, 3, 12, 0, 0, 0, time.UTC), bravo,
			nil,
		},
		{
			"CompanySinceBetweenDeletesOnSameDay",
			time.Date(2025, 3, 2, 12, 0, 0, 0, time.UTC), bravo,
			[]*db.DeletedContact{d},
		},
		{
			// A contact deleted exactly at the since timestamp is not returned.
			"CompanySinceIsExclusive",
			c.Modified, alpha,
			[]*db.DeletedContact{e},
		},
		{
			"CompanySinceAfterAll",
			time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC), alpha,
			nil,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			check(list(tc.since, tc.companyID), tc.expected...)
		})
	}

	// A contact deleted by DeleteContact is listed and is the most recently deleted.
	s.Run("DeleteContact", func() {
		require := s.Require()
		contactID := uuid.MustParse("4bc26eeb-0aec-4440-8fc7-1f1869422f16")

		// Timestamps are truncated to milliseconds, so wait long enough to guarantee
		// that the contact is deleted strictly after the before timestamp.
		before := fields.TimeNow().Time()
		time.Sleep(2 * time.Millisecond)

		err := s.db.DeleteContact(ctx, contactID)
		require.NoError(err)

		contacts := list(time.Time{}, uuid.Nil)
		require.Len(contacts, 6)
		require.Equal(contactID, contacts[0].ID)
		require.Equal(bravo, contacts[0].CompanyID)
		require.True(contacts[0].Modified.After(before), "expected the contact to be deleted after the test started")
		require.False(contacts[0].Modified.After(time.Now()), "expected the contact to be deleted before now")
		check(contacts[1:], e, d, c, b, a)

		// The contact is only listed for the company that it belonged to.
		check(list(time.Time{}, bravo), contacts[0], d, b)
		check(list(time.Time{}, alpha), e, c, a)

		// Only the contact deleted by the test was deleted since the test started.
		check(list(before, uuid.Nil), contacts[0])
		check(list(before, bravo), contacts[0])
		check(list(before, alpha))

		// When syncing from the most recent deletion, there is nothing new to return.
		check(list(contacts[0].Modified, uuid.Nil))
		check(list(contacts[0].Modified, bravo))
	})
}
