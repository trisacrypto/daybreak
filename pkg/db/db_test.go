package db_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/trisacrypto/daybreak/pkg/db"
	"go.rtnl.ai/x/dsn"
)

type Suite struct {
	suite.Suite

	db  *db.DB
	tmp string
	dsn *dsn.DSN
}

func TestDB(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (s *Suite) SetupSuite() {
	var err error
	require := s.Require()

	s.tmp, err = os.MkdirTemp("", "daybreak-test-")
	require.NoError(err)

	s.dsn = &dsn.DSN{
		Provider: dsn.SQLite,
		Path:     filepath.Join(s.tmp, "daybreak-test.db"),
	}

	s.db, err = db.Open(s.dsn)
	s.Require().NoError(err)
}

func (s *Suite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}

	if s.tmp != "" {
		os.RemoveAll(s.tmp)
	}

	s.db = nil
	s.dsn = nil
	s.tmp = ""
}

func (s *Suite) SetupTest() {}

func (s *Suite) TearDownTest() {
	err := s.TruncateTables()
	s.Require().NoError(err)
}

//============================================================================
// Helper Functions
//============================================================================

const rowIDCountQuery = `SELECT COUNT(rowid) AS count FROM `

func (p *Suite) Count(tx *sql.Tx, table string) (count int, err error) {
	query := rowIDCountQuery + table
	row := tx.QueryRow(query)
	err = row.Scan(&count)
	return count, err
}

func (s *Suite) LoadFixtures() (err error) {
	var tx *db.Tx
	if tx, err = s.db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: false, Isolation: sql.LevelSerializable}); err != nil {
		return err
	}
	defer tx.Rollback()

	var paths []string
	if paths, err = filepath.Glob("testdata/*.sql"); err != nil {
		return err
	}

	for _, path := range paths {
		if err = s.LoadFixture(path, tx); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Suite) LoadFixture(path string, tx *db.Tx) (err error) {
	var data []byte
	if data, err = os.ReadFile(path); err != nil {
		return err
	}

	var stmt *sql.Stmt
	if stmt, err = tx.Prepare(string(data)); err != nil {
		return err
	}
	defer stmt.Close()

	if _, err = stmt.Exec(); err != nil {
		return err
	}
	return nil
}

func (s *Suite) TruncateTables() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var tx *db.Tx
	if tx, err = s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: false, Isolation: sql.LevelSerializable}); err != nil {
		return err
	}
	defer tx.Rollback()

	var tables []string
	if tables, err = s.listTables(tx); err != nil {
		return err
	}

	for _, table := range tables {
		if _, err = tx.Exec("DELETE FROM " + table); err != nil {
			return err
		}
	}

	return tx.Commit()
}

const sqliteListTablesQuery = `SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`

func (s *Suite) listTables(tx *db.Tx) (tables []string, err error) {
	var rows *sql.Rows
	if rows, err = tx.Query(sqliteListTablesQuery); err != nil {
		return nil, err
	}
	defer rows.Close()

	tables = make([]string, 0)
	for rows.Next() {
		var table string
		if err = rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}

	return tables, rows.Err()
}
