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

func (r *Row) Scan(dest ...any) error {
	if err := r.Row.Scan(dest...); err != nil {
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

// Model is a pointer-receiver constraint: M must be *T and implement Scan. This allows
// the Iterator to allocate a non-nil *T via new(T) before scanning into it.
type Model[T any] interface {
	*T
	Scan(scanner Scanner) error
}

//============================================================================
// Iterator
//============================================================================

type Iterator[T any, M Model[T]] struct {
	rows *sql.Rows
}

func Iterate[T any, M Model[T]](rows *sql.Rows) *Iterator[T, M] {
	return &Iterator[T, M]{rows: rows}
}

func (i *Iterator[T, M]) Next() bool {
	return i.rows.Next()
}

func (i *Iterator[T, M]) Err() error {
	return i.rows.Err()
}

func (i *Iterator[T, M]) Close() error {
	return i.rows.Close()
}

func (i *Iterator[T, M]) Model() (M, error) {
	model := M(new(T))
	if err := model.Scan(i.rows); err != nil {
		return model, err
	}
	return model, nil
}

func (i *Iterator[T, M]) List() (models []M, err error) {
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
