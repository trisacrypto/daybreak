package db

import "database/sql"

type Tx struct {
	*sql.Tx
	opts *sql.TxOptions
}

func (t *Tx) Query(query string, nargs ...sql.NamedArg) (rows *sql.Rows, err error) {
	if rows, err = t.Tx.Query(query, args(nargs)...); err != nil {
		return nil, dbe(err)
	}
	return rows, nil
}

func (t *Tx) QueryRow(query string, nargs ...sql.NamedArg) *Row {
	row := t.Tx.QueryRow(query, args(nargs)...)
	return &Row{Row: row}
}

func (t *Tx) Exec(query string, nargs ...sql.NamedArg) (result sql.Result, err error) {
	if result, err = t.Tx.Exec(query, args(nargs)...); err != nil {
		return nil, dbe(err)
	}
	return result, nil
}

func args(args []sql.NamedArg) []any {
	pargs := make([]any, 0, len(args))
	for _, arg := range args {
		pargs = append(pargs, arg)
	}
	return pargs
}
