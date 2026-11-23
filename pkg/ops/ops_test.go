package ops

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trisacrypto/daybreak/pkg/db"
	"go.rtnl.ai/x/dsn"
)

func TestExportImport(t *testing.T) {
	ctx := t.Context()
	tmp := t.TempDir()

	// Create the source database and populate it with the testdata fixtures.
	src := openTestDB(t, filepath.Join(tmp, "src.db"))
	loadFixtures(t, src)

	srcCompanies, srcContacts := snapshot(t, src)
	require.NotEmpty(t, srcCompanies, "expected the fixtures to create companies in the source database")
	require.NotEmpty(t, srcContacts, "expected the fixtures to create contacts in the source database")

	// Export the source database to a zip file in the temporary directory.
	archive := filepath.Join(tmp, "export.zip")
	require.NoError(t, Export(ctx, src, archive), "could not export the source database")
	require.FileExists(t, archive, "expected the export to create a zip file")

	// Create an empty destination database and import the zip file into it.
	dst := openTestDB(t, filepath.Join(tmp, "dst.db"))
	dstCompanies, dstContacts := snapshot(t, dst)
	require.Empty(t, dstCompanies, "expected the destination database to have no companies before import")
	require.Empty(t, dstContacts, "expected the destination database to have no contacts before import")

	require.NoError(t, Import(ctx, dst, archive), "could not import the zip file into the destination database")

	// The destination database should now be equivalent to the source database.
	dstCompanies, dstContacts = snapshot(t, dst)
	require.Equal(t, srcCompanies, dstCompanies, "companies in the destination database do not match the source database")
	require.Equal(t, srcContacts, dstContacts, "contacts in the destination database do not match the source database")
}

//============================================================================
// Helper Functions
//============================================================================

// Opens a sqlite database at the specified path, applying all migrations, and closes
// it when the test completes.
func openTestDB(t *testing.T, path string) *db.DB {
	t.Helper()

	conn, err := db.Open(&dsn.DSN{Provider: dsn.SQLite, Path: path})
	require.NoError(t, err, "could not open database at %s", path)

	t.Cleanup(func() {
		require.NoError(t, conn.Close(), "could not close database at %s", path)
	})

	return conn
}

// Executes every *.sql file in the testdata directory against the database.
func loadFixtures(t *testing.T, conn *db.DB) {
	t.Helper()

	paths, err := filepath.Glob("testdata/*.sql")
	require.NoError(t, err, "could not glob the testdata directory")
	require.NotEmpty(t, paths, "no sql fixtures found in the testdata directory")

	tx, err := conn.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: false})
	require.NoError(t, err, "could not begin fixtures transaction")
	defer tx.Rollback()

	for _, path := range paths {
		data, err := os.ReadFile(path)
		require.NoError(t, err, "could not read fixture %s", path)

		_, err = tx.Exec(string(data))
		require.NoError(t, err, "could not execute fixture %s", path)
	}

	require.NoError(t, tx.Commit(), "could not commit fixtures transaction")
}

// Returns all of the companies and contacts in the database in a deterministic order
// with normalized JSON fields so that two databases can be compared for equivalence.
func snapshot(t *testing.T, conn *db.DB) (companies []*db.Company, contacts []*db.Contact) {
	t.Helper()

	var err error
	companies, err = conn.ListCompanies(t.Context())
	require.NoError(t, err, "could not list companies")

	slices.SortFunc(companies, func(a, b *db.Company) int {
		return bytes.Compare(a.ID[:], b.ID[:])
	})

	for _, company := range companies {
		company.IVMS101.JSONB = company.IVMS101.JSONB.Normalize()
	}

	contacts, err = conn.ListContacts(t.Context())
	require.NoError(t, err, "could not list contacts")

	slices.SortFunc(contacts, func(a, b *db.Contact) int {
		return bytes.Compare(a.ID[:], b.ID[:])
	})

	for _, contact := range contacts {
		contact.IVMS101.JSONB = contact.IVMS101.JSONB.Normalize()
	}

	return companies, contacts
}
