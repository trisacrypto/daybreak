package db

import (
	"context"
	"database/sql"
)

//============================================================================
// Dashboard Info
//============================================================================

type DashboardInfo struct {
	Companies int64
	Contacts  int64
	Countries int64
	Licenses  int64
}

const (
	companyCountSQL = `SELECT COUNT(*) FROM companies;`
	contactCountSQL = `SELECT COUNT(*) FROM contacts;`
	countryCountSQL = `SELECT COUNT(DISTINCT country) FROM companies WHERE country IS NOT NULL AND country != '';`
	licenseCountSQL = `SELECT COUNT(*) FROM licenses;`
)

func (tx *Tx) DashboardInfo() (info *DashboardInfo, err error) {
	info = &DashboardInfo{}

	if err = tx.QueryRow(companyCountSQL).Scan(&info.Companies); err != nil {
		return nil, err
	}
	if err = tx.QueryRow(contactCountSQL).Scan(&info.Contacts); err != nil {
		return nil, err
	}
	if err = tx.QueryRow(countryCountSQL).Scan(&info.Countries); err != nil {
		return nil, err
	}
	if err = tx.QueryRow(licenseCountSQL).Scan(&info.Licenses); err != nil {
		return nil, err
	}

	return info, nil
}

func (db *DB) DashboardInfo(ctx context.Context) (info *DashboardInfo, err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: true}); err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return tx.DashboardInfo()
}

//============================================================================
// Dashboard Global VASPs Chart
//============================================================================

type CountryCount struct {
	Count   int64
	Country string
}

func (c *CountryCount) Scan(scanner Scanner) (err error) {
	if err = scanner.Scan(&c.Country, &c.Count); err != nil {
		return dbe(err)
	}
	return nil
}

const countsByCountrySQL = `SELECT country, COUNT(*) AS count FROM companies GROUP BY country ORDER BY COUNT DESC;`

func (tx *Tx) CountryCounts() (counts *Iterator[CountryCount, *CountryCount], err error) {
	var rows *sql.Rows
	if rows, err = tx.Query(countsByCountrySQL); err != nil {
		return nil, err
	}

	counts = Iterate[CountryCount](rows)
	return counts, nil
}

func (db *DB) CountryCounts(ctx context.Context) (counts []*CountryCount, err error) {
	var tx *Tx
	if tx, err = db.Begin(ctx, &sql.TxOptions{ReadOnly: true}); err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var iter *Iterator[CountryCount, *CountryCount]
	if iter, err = tx.CountryCounts(); err != nil {
		return nil, err
	}
	defer iter.Close()

	return iter.List()
}

//============================================================================
// Dashboard Data Readiness Charts
//============================================================================

//============================================================================
// Dashboard Counterparties
//============================================================================

//============================================================================
// Dashboard Licenses
//============================================================================
