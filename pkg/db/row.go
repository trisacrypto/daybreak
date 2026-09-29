package db

import "database/sql"

type Row struct {
	*sql.Row
}

func (r *Row) Err() error {
	if err := r.Row.Err(); err != nil {
		return dbe(err)
	}
	return nil
}
