package db

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/trisacrypto/daybreak/pkg/db/fields"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"go.rtnl.ai/x/rlog"
)

const janitorInterval = 1 * time.Hour

type Janitor struct {
	db     *DB
	stop   chan struct{}
	ticker *time.Ticker

	allSessions   *sql.Stmt
	deleteSession *sql.Stmt
}

func (j *Janitor) Stop() {
	j.ticker.Stop()
	close(j.stop)
}

func (j *Janitor) Run() (err error) {
	// Initialize the stop channel and ticker.
	j.stop = make(chan struct{}, 1)
	j.ticker = time.NewTicker(janitorInterval)

	// Initialize the statements.
	if j.allSessions, err = j.db.DB.Prepare(allSessionsSQL); err != nil {
		return err
	}

	if j.deleteSession, err = j.db.DB.Prepare(deleteSessionSQL); err != nil {
		return err
	}

	// Start the janitor routine.
	rlog.Debug("starting database janitor routine", slog.String("interval", janitorInterval.String()))

	// Execute the cleanup routine at startup to detect errors early.
	if err = j.Cleanup(); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-j.ticker.C:
				if err := j.Cleanup(); err != nil {
					rlog.Error("error cleaning up database", slog.String("error", err.Error()))
				}
			case <-j.stop:
				return
			}
		}
	}()

	return nil
}

func (j *Janitor) Cleanup() (err error) {
	if j.db.uri.ReadOnly() {
		return nil
	}

	if cerr := j.cleanupSessions(); cerr != nil {
		err = errors.Join(err, cerr)
	}

	return err
}

func (j *Janitor) cleanupSessions() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var tx *sql.Tx
	if tx, err = j.db.DB.BeginTx(ctx, nil); err != nil {
		return err
	}
	defer tx.Rollback()

	var rows *sql.Rows
	if rows, err = tx.StmtContext(ctx, j.allSessions).QueryContext(ctx); err != nil {
		return err
	}
	defer rows.Close()

	now := fields.TimeNow()
	for rows.Next() {
		var (
			id      string
			expires fields.Timestamp
		)

		if err = rows.Scan(&id, &expires); err != nil {
			return err
		}

		if expires.Before(now) || expires.IsZero() {
			if _, err = tx.StmtContext(ctx, j.deleteSession).ExecContext(ctx, id); err != nil {
				return err
			}
		}
	}

	if err = rows.Err(); err != nil {
		return err
	}

	return tx.Commit()
}
