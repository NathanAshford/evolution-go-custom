package whatsmeow_service

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Pool limits for the shared device store. Postgres defaults to 100 client
// slots for the WHOLE server, and this process already keeps two other pools
// (users DB and auth DB, 25 each), so the device store gets a comparable share
// rather than database/sql's default of "unlimited".
const (
	storeMaxOpenConns    = 25
	storeMaxIdleConns    = 5
	storeConnMaxLifetime = 5 * time.Minute
	storeConnMaxIdleTime = time.Minute
)

// The whatsmeow device store is a database, and a *sqlstore.Container is just a
// handle onto it. sqlstore.New calls sql.Open, which builds a WHOLE NEW
// connection pool — so calling it per connection attempt opens pools that are
// never reused and never closed.
//
// In production that emptied Postgres: every StartClient, every QR poll and
// every auto-reconnect leaked another pool, until the server answered
//
//	pq: sorry, too many clients already
//
// to everything, the retries included — so no instance could start or show a QR
// again. The failure feeds itself, which is why it took the whole fleet down at
// once rather than one instance at a time.
//
// The container is safe to share: it wraps a *sql.DB, which is itself a
// concurrent, pooled handle meant to be long-lived and used from many
// goroutines. So we build exactly one per process, behind sync.Once, and every
// instance borrows it.
type storeContainerCache struct {
	once      sync.Once
	container *sqlstore.Container
	db        *sql.DB
	err       error
}

// sharedStoreContainer is the one device store this process opens.
var sharedStoreContainer storeContainerCache

// get returns the process-wide device-store container, creating it on first use.
//
// A failure is cached deliberately: if the database is unreachable, reopening a
// pool per attempt is exactly what caused the outage. Callers see the error and
// back off through the normal reconnect path instead.
func (c *storeContainerCache) get(ctx context.Context, dialect, address string, log waLog.Logger) (*sqlstore.Container, error) {
	c.once.Do(func() {
		db, err := sql.Open(dialect, address)
		if err != nil {
			c.err = fmt.Errorf("failed to open the whatsmeow device store: %w", err)
			return
		}

		// Bound the pool before anything can use it. Without this, database/sql
		// allows unlimited open connections, so a burst of instances connecting
		// at once could still exhaust the server — a smaller version of the bug
		// above.
		db.SetMaxOpenConns(storeMaxOpenConns)
		db.SetMaxIdleConns(storeMaxIdleConns)
		db.SetConnMaxLifetime(storeConnMaxLifetime)
		db.SetConnMaxIdleTime(storeConnMaxIdleTime)

		container := sqlstore.NewWithDB(db, dialect, log)
		if err := container.Upgrade(ctx); err != nil {
			_ = db.Close()
			c.err = fmt.Errorf("failed to upgrade the whatsmeow device store: %w", err)
			return
		}

		c.container = container
		c.db = db
	})

	return c.container, c.err
}

// close releases the shared container. For shutdown only — instances must never
// close it, since they all share the one handle.
func (c *storeContainerCache) close() error {
	if c.container == nil {
		return nil
	}
	return c.container.Close()
}

// CloseDeviceStore releases the process-wide device store. For shutdown only.
func CloseDeviceStore() error {
	return sharedStoreContainer.close()
}

// deviceStore returns the shared device store, opening it on first use.
func (w *whatsmeowService) deviceStore() (*sqlstore.Container, error) {
	var log waLog.Logger
	if w.config.WaDebug != "" {
		log = waLog.Stdout("Database", w.config.WaDebug, true)
	}

	dialect, address := "postgres", w.config.PostgresAuthDB
	if address == "" {
		dialect = "sqlite"
		address = fmt.Sprintf(
			"file:%s/dbdata/main.db?_pragma=foreign_keys(1)&_busy_timeout=5000&cache=shared&mode=rwc&_journal_mode=WAL",
			w.exPath,
		)
	}

	return sharedStoreContainer.get(context.Background(), dialect, address, log)
}
