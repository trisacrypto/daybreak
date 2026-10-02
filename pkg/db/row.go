package db

import "database/sql"

//============================================================================
// Row Wrapper
//============================================================================

type Row struct {
	*sql.Row
}

func (r *Row) Err() error {
	if err := r.Row.Err(); err != nil {
		return dbe(err)
	}
	return nil
}

//============================================================================
// Scanner Interface
//============================================================================

// Scanner is an interface for *sql.Rows and *sql.Row so that models can implement how
// they scan fields into their struct without having to specify every field every time.
type Scanner interface {
	Scan(dest ...any) error
}

type Model interface {
	Scan(scanner Scanner) error
}

//============================================================================
// Iterator
//============================================================================

type Iterator[M Model] struct {
	rows *sql.Rows
}

func Iterate[M Model](rows *sql.Rows) *Iterator[M] {
	return &Iterator[M]{rows: rows}
}

func (i *Iterator[M]) Next() bool {
	return i.rows.Next()
}

func (i *Iterator[M]) Err() error {
	return i.rows.Err()
}

func (i *Iterator[M]) Close() error {
	return i.rows.Close()
}

func (i *Iterator[M]) Model() (M, error) {
	var model M
	if err := i.rows.Scan(&model); err != nil {
		return model, err
	}
	return model, nil
}

func (i *Iterator[M]) List() (models []M, err error) {
	models = make([]M, 0)
	defer i.Close()

	for i.Next() {
		var model M
		if model, err = i.Model(); err != nil {
			return models, err
		}
		models = append(models, model)
	}
	return models, i.Err()
}
