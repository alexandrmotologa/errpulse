package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps an active SQLite connection configured for WAL mode.
type DB struct {
	*sql.DB
	path string
}

// Open initializes or opens a SQLite database file with WAL pragmas.
func Open(dbPath string) (*DB, error) {
	dsn := ""
	if dbPath == ":memory:" {
		dsn = "file:memdb_errpulse?mode=memory&cache=shared&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	} else {
		dir := filepath.Dir(dbPath)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed creating db directory %s: %w", dir, err)
			}
		}
		dsn = fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=cache_size(-2000)&_pragma=temp_store(MEMORY)", dbPath)
	}

	rawDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed opening sqlite database: %w", err)
	}

	rawDB.SetMaxOpenConns(10) // Concurrent connections for WAL mode
	rawDB.SetMaxIdleConns(5)

	if err := rawDB.Ping(); err != nil {
		rawDB.Close()
		return nil, fmt.Errorf("failed pinging sqlite database: %w", err)
	}

	if err := migrateSchema(rawDB); err != nil {
		rawDB.Close()
		return nil, err
	}

	db := &DB{
		DB:   rawDB,
		path: dbPath,
	}

	// Ensure default project exists
	if err := db.ensureDefaultProject(); err != nil {
		rawDB.Close()
		return nil, fmt.Errorf("failed provisioning default project: %w", err)
	}

	return db, nil
}

func (db *DB) ensureDefaultProject() error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		_, err = db.Exec(`
			INSERT INTO projects (id, name, slug, public_key, secret_key)
			VALUES (1, 'Default Project', 'default', 'errpulse_default_key', 'errpulse_default_secret')
		`)
		if err != nil {
			return err
		}
	}
	return nil
}
