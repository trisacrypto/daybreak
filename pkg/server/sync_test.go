package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/errors"
)

// The companies and contacts in the testdata fixtures, with their modified dates.
// cSpell:ignore globex initech anneliese priya
var (
	acmeID      = uuid.MustParse("21c407b4-0f7b-4536-b3b3-2d40ed17b16d") // 2025-11-29
	globexID    = uuid.MustParse("9c865355-07b7-4013-b4c1-1d986def39d5") // 2024-06-18
	initechID   = uuid.MustParse("2b507068-495b-4237-af70-852180a91d99") // 2024-12-18
	franklinID  = uuid.MustParse("135a608a-5c3b-4339-b40b-0a98e574a883") // 2025-03-22 (acme)
	jessicaID   = uuid.MustParse("9f572069-92ea-4fed-9a43-34a61652f679") // 2025-10-05 (acme)
	hansID      = uuid.MustParse("4d2f8c1e-6b7a-4e35-a9d0-73c5e2f1b864") // 2024-06-18 (globex)
	annelieseID = uuid.MustParse("b81e3a57-0c94-4f2d-8e6b-5a7d1c9f0e23") // 2025-01-23 (globex)
	priyaID     = uuid.MustParse("e6a09d3b-2f18-47c6-b5d4-91f0a7c3e852") // 2024-12-18 (initech)
)

// A timestamp that splits the fixtures: acme, franklin, jessica, and anneliese were
// modified after it; globex, initech, hans, and priya were modified before it.
var modifiedSince = time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)

func (s *Suite) TestSync() {
	s.T().Cleanup(s.ResetDB)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	syncs, downloads := s.licenseCounters(ctx)
	s.requireNoLastModified(ctx)

	// Case 1: without a last modified timestamp in the database the server cannot
	// return a 304, even though nothing was modified after the header timestamp.
	s.Run("NoLastModified", func() {
		require := s.Require()

		earliest := time.Now().UTC().Truncate(time.Second)
		reply, lastModified, err := s.client.Sync(ctx, time.Now().Add(-1*time.Hour))
		latest := time.Now().UTC()

		require.NoError(err, "could not sync")
		require.Empty(reply.Companies, "no companies were modified in the last hour")
		require.Empty(reply.Contacts, "no contacts were modified in the last hour")
		require.Empty(reply.DeletedCompanies, "no companies have been deleted")
		require.Empty(reply.DeletedContacts, "no contacts have been deleted")

		// The last modified header falls back to the time of the request.
		require.False(lastModified.IsZero(), "expected a last modified header")
		require.False(lastModified.Before(earliest), "last modified %s is before %s", lastModified, earliest)
		require.False(lastModified.After(latest), "last modified %s is after %s", lastModified, latest)
		require.True(lastModified.Equal(reply.LastModified), "last modified header should match the reply")

		s.requireNoLastModified(ctx)

		syncs++
		s.requireLicenseCounters(ctx, syncs, downloads)
	})

	// NOTE: if this setup fails all of the subtests that follow it will fail.
	dbModified := s.modifyDataset(ctx)

	// Case 2: without the header the whole dataset is returned with no deletions,
	// even though a company and a contact have been deleted.
	s.Run("NoIfModifiedSince", func() {
		require := s.Require()

		reply, lastModified, err := s.client.Sync(ctx, time.Time{})
		require.NoError(err, "could not sync")
		require.ElementsMatch([]uuid.UUID{acmeID, globexID}, modifiedIDs(reply.Companies), "expected all companies")
		require.ElementsMatch([]uuid.UUID{franklinID, hansID, annelieseID}, modifiedIDs(reply.Contacts), "expected all contacts")
		require.Empty(reply.DeletedCompanies, "deleted companies should not be returned without the header")
		require.Empty(reply.DeletedContacts, "deleted contacts should not be returned without the header")

		require.False(lastModified.IsZero(), "expected a last modified header")
		require.True(dbModified.Equal(lastModified), "last modified header should be the database timestamp")
		require.True(dbModified.Equal(reply.LastModified), "last modified reply should be the database timestamp")

		syncs++
		s.requireLicenseCounters(ctx, syncs, downloads)
	})

	// Case 3: the header is the last modified timestamp returned by the prior sync.
	s.Run("NotModified", func() {
		require := s.Require()

		reply, lastModified, err := s.client.Sync(ctx, dbModified)
		require.ErrorIs(err, errors.ErrNotModified, "expected a 304 response")
		require.Nil(reply, "no reply should be returned when not modified")

		require.False(lastModified.IsZero(), "expected a last modified header")
		require.True(dbModified.Equal(lastModified), "last modified header should be the database timestamp")

		// A 304 response is not counted as a sync.
		s.requireLicenseCountersUnchanged(ctx, syncs, downloads)
	})

	// Case 4: the header is before the last modified timestamp in the database, so
	// only the records modified or deleted after the header timestamp are returned.
	s.Run("ModifiedSince", func() {
		require := s.Require()

		reply, lastModified, err := s.client.Sync(ctx, modifiedSince)
		require.NoError(err, "could not sync")
		require.ElementsMatch([]uuid.UUID{acmeID}, modifiedIDs(reply.Companies), "expected only the companies modified since")
		require.ElementsMatch([]uuid.UUID{franklinID, annelieseID}, modifiedIDs(reply.Contacts), "expected only the contacts modified since")
		require.ElementsMatch([]uuid.UUID{initechID}, modifiedIDs(reply.DeletedCompanies), "expected the deleted company")
		require.ElementsMatch([]uuid.UUID{jessicaID}, modifiedIDs(reply.DeletedContacts), "expected the deleted contact")

		require.False(lastModified.IsZero(), "expected a last modified header")
		require.True(dbModified.Equal(lastModified), "last modified header should be the database timestamp")
		require.True(dbModified.Equal(reply.LastModified), "last modified reply should be the database timestamp")

		syncs++
		s.requireLicenseCounters(ctx, syncs, downloads)
	})
}

func (s *Suite) TestDownload() {
	s.T().Cleanup(s.ResetDB)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	syncs, downloads := s.licenseCounters(ctx)
	s.requireNoLastModified(ctx)

	// Case 1: without a last modified timestamp in the database the server cannot
	// return a 304, even though nothing was modified after the header timestamp.
	s.Run("NoLastModified", func() {
		require := s.Require()
		buf := &bytes.Buffer{}

		earliest := time.Now().UTC().Truncate(time.Second)
		lastModified, err := s.client.Download(ctx, time.Now().Add(-1*time.Hour), buf)
		latest := time.Now().UTC()

		require.NoError(err, "could not download")
		companies, contacts := s.readArchive(buf.Bytes())
		require.ElementsMatch([]uuid.UUID{acmeID, globexID, initechID}, companies, "expected all companies")
		require.ElementsMatch([]uuid.UUID{franklinID, jessicaID, hansID, annelieseID, priyaID}, contacts, "expected all contacts")

		// The last modified header falls back to the time of the request.
		require.False(lastModified.IsZero(), "expected a last modified header")
		require.False(lastModified.Before(earliest), "last modified %s is before %s", lastModified, earliest)
		require.False(lastModified.After(latest), "last modified %s is after %s", lastModified, latest)

		s.requireNoLastModified(ctx)

		downloads++
		s.requireLicenseCounters(ctx, syncs, downloads)
	})

	// NOTE: if this setup fails all of the subtests that follow it will fail.
	dbModified := s.modifyDataset(ctx)

	// Case 2: without the header the whole dataset is returned with no deletions.
	s.Run("NoIfModifiedSince", func() {
		require := s.Require()
		buf := &bytes.Buffer{}

		lastModified, err := s.client.Download(ctx, time.Time{}, buf)
		require.NoError(err, "could not download")
		companies, contacts := s.readArchive(buf.Bytes())
		require.ElementsMatch([]uuid.UUID{acmeID, globexID}, companies, "expected all companies without the deleted company")
		require.ElementsMatch([]uuid.UUID{franklinID, hansID, annelieseID}, contacts, "expected all contacts without the deleted contacts")

		require.False(lastModified.IsZero(), "expected a last modified header")
		require.True(dbModified.Equal(lastModified), "last modified header should be the database timestamp")

		downloads++
		s.requireLicenseCounters(ctx, syncs, downloads)
	})

	// Case 3: the header is the last modified timestamp returned by the prior download.
	s.Run("NotModified", func() {
		require := s.Require()
		buf := &bytes.Buffer{}

		lastModified, err := s.client.Download(ctx, dbModified, buf)
		require.ErrorIs(err, errors.ErrNotModified, "expected a 304 response")
		require.Zero(buf.Len(), "nothing should be written when not modified")

		require.False(lastModified.IsZero(), "expected a last modified header")
		require.True(dbModified.Equal(lastModified), "last modified header should be the database timestamp")

		// A 304 response is not counted as a download.
		s.requireLicenseCountersUnchanged(ctx, syncs, downloads)
	})

	// Case 4: the header is before the last modified timestamp in the database.
	// NOTE: a download is always the whole database, not the subset modified since.
	s.Run("ModifiedSince", func() {
		require := s.Require()
		buf := &bytes.Buffer{}

		lastModified, err := s.client.Download(ctx, modifiedSince, buf)
		require.NoError(err, "could not download")
		companies, contacts := s.readArchive(buf.Bytes())
		require.ElementsMatch([]uuid.UUID{acmeID, globexID}, companies, "expected all companies without the deleted company")
		require.ElementsMatch([]uuid.UUID{franklinID, hansID, annelieseID}, contacts, "expected all contacts without the deleted contacts")

		require.False(lastModified.IsZero(), "expected a last modified header")
		require.True(dbModified.Equal(lastModified), "last modified header should be the database timestamp")

		downloads++
		s.requireLicenseCounters(ctx, syncs, downloads)
	})
}

//============================================================================
// Helper Functions
//============================================================================

// Deletes a contact and a company through the API so that the database has a last
// modified timestamp and deleted records. Returns the last modified timestamp of the
// database as it is expected in the Last-Modified header. The dataset is then:
//
//   - companies: acme (modified now) and globex (2024-06-18)
//   - contacts: franklin (2025-03-22), hans (2024-06-18), and anneliese (2025-01-23)
//   - deleted companies: initech (which also removes priya)
//   - deleted contacts: jessica
func (s *Suite) modifyDataset(ctx context.Context) time.Time {
	require := s.Require()
	require.NoError(s.client.DeleteContact(ctx, jessicaID), "could not delete contact")
	require.NoError(s.client.DeleteCompany(ctx, initechID), "could not delete company")

	modified, err := s.server.db.LastModifiedHTTP(ctx)
	require.NoError(err, "could not get last modified timestamp from database")
	require.False(modified.IsZero(), "expected a last modified timestamp in the database")
	return modified
}

func (s *Suite) requireNoLastModified(ctx context.Context) {
	modified, err := s.server.db.LastModified(ctx)
	s.Require().NoError(err, "could not get last modified timestamp from database")
	s.Require().True(modified.IsZero(), "expected no last modified timestamp in the database")
}

// Returns the usage counters of the license that the suite client authenticates with.
func (s *Suite) licenseCounters(ctx context.Context) (syncs, downloads int64) {
	license, err := s.server.db.RetrieveLicense(ctx, adminID)
	s.Require().NoError(err, "could not retrieve license from database")
	return license.Syncs, license.Downloads
}

// The server updates the license counters after the response is sent, so wait for
// the counters to reach the expected values.
func (s *Suite) requireLicenseCounters(ctx context.Context, syncs, downloads int64) {
	s.Require().Eventually(func() bool {
		license, err := s.server.db.RetrieveLicense(ctx, adminID)
		return err == nil && license.Syncs == syncs && license.Downloads == downloads
	}, 2*time.Second, 10*time.Millisecond, "expected %d syncs and %d downloads", syncs, downloads)
}

// Because the server updates the license counters after the response is sent, watch
// the counters for a short time to ensure that they stay at the expected values.
func (s *Suite) requireLicenseCountersUnchanged(ctx context.Context, syncs, downloads int64) {
	s.Require().Never(func() bool {
		license, err := s.server.db.RetrieveLicense(ctx, adminID)
		return err != nil || license.Syncs != syncs || license.Downloads != downloads
	}, 200*time.Millisecond, 20*time.Millisecond, "expected %d syncs and %d downloads", syncs, downloads)
}

// Returns the IDs of the companies and contacts in a downloaded zip archive.
func (s *Suite) readArchive(data []byte) (companies, contacts []uuid.UUID) {
	require := s.Require()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(err, "could not read download as a zip archive")

	// Each file in the archive is a stream of newline delimited JSON objects.
	readIDs := func(name string) (ids []uuid.UUID) {
		f, err := zr.Open(name)
		require.NoError(err, "could not open %s in archive", name)
		defer f.Close()

		decoder := json.NewDecoder(f)
		for {
			var record struct {
				ID uuid.UUID `json:"id"`
			}

			if err = decoder.Decode(&record); err == io.EOF {
				return ids
			}

			require.NoError(err, "could not decode record in %s", name)
			ids = append(ids, record.ID)
		}
	}

	_, err = zr.Open("primaries.json")
	require.NoError(err, "could not open primaries.json in archive")

	return readIDs("companies.json"), readIDs("contacts.json")
}

func modifiedIDs(items []*api.Modified) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}
