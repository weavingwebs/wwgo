package wwdb

import (
	"context"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/subchen/go-trylock/v2"
	"sync"
)

// Ensures that this instance (cli/api) also honours the locks even if it is
// using the same DB connection etc.
type lock struct {
	mut  trylock.TryLocker
	conn *sqlx.Conn
}

var lockMutexes = make(map[string]*lock)
var lockMutexesLock = &sync.Mutex{}

func Lock(ctx context.Context, db *sqlx.DB, name string) error {
	lockMutexesLock.Lock()
	lockMutex, ok := lockMutexes[name]
	if !ok {
		lockMutex = &lock{
			mut: trylock.New(),
		}
		lockMutexes[name] = lockMutex
	}
	lockMutexesLock.Unlock()
	if !lockMutex.mut.TryLock(ctx) {
		return errors.Errorf("failed to aquire lock %s", name)
	}

	// Obtain a specific DB connection to run the lock statements on.
	var err error
	lockMutex.conn, err = db.Connx(ctx)
	if err != nil {
		return errors.Errorf("failed to obtain a DB connection")
	}

	// Lock via DB.
	row := lockMutex.conn.QueryRowxContext(ctx, `SELECT GET_LOCK(?, 30)`, name)
	var success bool
	if err := row.Scan(&success); err != nil {
		_ = lockMutex.conn.Close()
		return errors.Wrapf(err, "failed to lock %s", name)
	}
	if !success {
		_ = lockMutex.conn.Close()
		return errors.Errorf("could not aquire lock, timed out")
	}
	return nil
}

func Unlock(ctx context.Context, name string) error {
	lockMutexesLock.Lock()
	lockMutex, ok := lockMutexes[name]
	lockMutexesLock.Unlock()
	if ok {
		defer lockMutex.mut.Unlock()
		defer func() { _ = lockMutex.conn.Close() }()

		_, err := lockMutex.conn.ExecContext(ctx, `SELECT RELEASE_LOCK(?)`, name)
		if err != nil {
			return errors.Wrapf(err, "failed to release lock %s", name)
		}
	}

	return nil
}
