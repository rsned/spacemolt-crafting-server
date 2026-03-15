package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps a sql.DB with crafting-specific methods.
type DB struct {
	*sql.DB
	catPri *CategoryPriorityStore
}

// Open opens a SQLite database at the given path.
// If the path is ":memory:", an in-memory database is created.
func Open(path string) (*DB, error) {
	// Enable foreign keys and WAL mode for better concurrency
	dsn := fmt.Sprintf("%s?_foreign_keys=on&_journal_mode=WAL", path)

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Verify connection
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	db := &DB{DB: sqlDB}
	db.catPri = NewCategoryPriorityStore(db)

	return db, nil
}

// OpenAndInit opens the database and initializes the schema.
func OpenAndInit(ctx context.Context, path string) (*DB, error) {
	db, err := Open(path)
	if err != nil {
		return nil, err
	}

	if err := InitSchema(ctx, db.DB); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	// Initialize category priorities
	if err := db.catPri.InitializeDefaultPriorities(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing category priorities: %w", err)
	}

	return db, nil
}

// CategoryPriorities returns the category priority store.
func (db *DB) CategoryPriorities() *CategoryPriorityStore {
	return db.catPri
}

// InTransaction executes fn within a transaction.
// If fn returns an error, the transaction is rolled back.
// Otherwise, it is committed.
func (db *DB) InTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// InsertOrderBookEntry inserts a single order into the market_order_book table.
func (db *DB) InsertOrderBookEntry(ctx context.Context, batchID, itemID, stationID, orderType string, price, volume int, source, recordedAt string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO market_order_book
		(batch_id, item_id, station_id, empire_id, order_type, price_per_unit, volume_available, player_stall_name, recorded_at)
		VALUES (?, ?, ?, NULL, ?, ?, ?, ?, ?)
	`, batchID, itemID, stationID, orderType, price, volume, source, recordedAt)
	if err != nil {
		return fmt.Errorf("inserting order book entry: %w", err)
	}
	return nil
}

// GetSyncMetadata retrieves a metadata value by key.
func (db *DB) GetSyncMetadata(ctx context.Context, key string) (string, error) {
	var value string
	err := db.QueryRowContext(ctx,
		`SELECT value FROM sync_metadata WHERE key = ?`,
		key,
	).Scan(&value)

	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("querying sync metadata: %w", err)
	}

	return value, nil
}

// SetSyncMetadata sets a metadata value.
func (db *DB) SetSyncMetadata(ctx context.Context, key, value string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO sync_metadata (key, value, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at
	`, key, value)

	if err != nil {
		return fmt.Errorf("setting sync metadata: %w", err)
	}

	return nil
}
