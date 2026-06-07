package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migration represents a database schema migration.
type Migration struct {
	ID      string
	UpSQL   string
	DownSQL string
}

// MigrationTracker tracks which database migrations have been applied.
type MigrationTracker struct {
	db *DB
}

// NewMigrationTracker creates a new migration tracker.
func NewMigrationTracker(db *DB) *MigrationTracker {
	return &MigrationTracker{db: db}
}

// IsApplied checks if a migration has been applied.
func (m *MigrationTracker) IsApplied(ctx context.Context, migrationID string) (bool, error) {
	var count int
	err := m.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE migration_id = ?`,
		migrationID,
	).Scan(&count)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// RecordApplied records that a migration has been applied.
func (m *MigrationTracker) RecordApplied(ctx context.Context, migrationID string) error {
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO schema_migrations (migration_id, applied_at) VALUES (?, datetime('now'))`,
		migrationID,
	)
	return err
}

// GetCurrentVersion returns the most recently applied migration ID.
func (m *MigrationTracker) GetCurrentVersion(ctx context.Context) (string, error) {
	var version string
	err := m.db.QueryRowContext(ctx,
		`SELECT migration_id FROM schema_migrations ORDER BY applied_at DESC LIMIT 1`,
	).Scan(&version)

	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return version, nil
}

// Migrator applies database migrations.
type Migrator struct {
	db      *DB
	tracker *MigrationTracker
}

// NewMigrator creates a new migrator.
func NewMigrator(db *DB) *Migrator {
	return &Migrator{
		db:      db,
		tracker: NewMigrationTracker(db),
	}
}

// Apply applies a migration if it hasn't been applied yet.
func (m *Migrator) Apply(ctx context.Context, migration *Migration) error {
	// Check if already applied
	applied, err := m.tracker.IsApplied(ctx, migration.ID)
	if err != nil {
		return err
	}
	if applied {
		// Already applied, skip (idempotent)
		return nil
	}

	// Apply migration in a transaction
	return m.db.InTransaction(ctx, func(tx *sql.Tx) error {
		// Execute the Up migration
		if _, err := tx.ExecContext(ctx, migration.UpSQL); err != nil {
			return err
		}

		// Record as applied
		_, err = tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (migration_id, applied_at) VALUES (?, datetime('now'))`,
			migration.ID,
		)
		return err
	})
}

// GetMigration005 returns the market data enhancement migration.
func GetMigration005() (*Migration, error) {
	data, err := migrationFS.ReadFile("migrations/005_add_enhanced_market_tables.sql")
	if err != nil {
		return nil, err
	}

	return &Migration{
		ID:    "005_add_enhanced_market_tables",
		UpSQL: string(data),
		DownSQL: `
			DROP TABLE IF EXISTS market_order_book;
			DROP TABLE IF EXISTS market_price_stats;
		`,
	}, nil
}

// GetMigration006 returns the stations table migration.
func GetMigration006() (*Migration, error) {
	data, err := migrationFS.ReadFile("migrations/006_add_stations_table.sql")
	if err != nil {
		return nil, err
	}

	return &Migration{
		ID:      "006_add_stations_table",
		UpSQL:   string(data),
		DownSQL: `DROP TABLE IF EXISTS stations;`,
	}, nil
}

// ApplyMigration006 applies migration 006 (stations table).
func ApplyMigration006(ctx context.Context, db *DB) error {
	migration, err := GetMigration006()
	if err != nil {
		return err
	}

	migrator := NewMigrator(db)
	return migrator.Apply(ctx, migration)
}

// ApplyMigration005 applies migration 005 with special handling for empire_id column.
func ApplyMigration005(ctx context.Context, db *DB) error {
	migration, err := GetMigration005()
	if err != nil {
		return err
	}

	// Check if migration already applied
	tracker := NewMigrationTracker(db)
	applied, err := tracker.IsApplied(ctx, migration.ID)
	if err != nil {
		return err
	}
	if applied {
		return nil // Already applied
	}

	return db.InTransaction(ctx, func(tx *sql.Tx) error {
		// First, add empire_id column if it doesn't exist
		// Check if column exists using PRAGMA
		var hasColumn bool
		rows, err := tx.QueryContext(ctx, `PRAGMA table_info(market_prices)`)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue any
		for rows.Next() {
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
				return err
			}
			if name == "empire_id" {
				hasColumn = true
				break
			}
		}

		if !hasColumn {
			if _, err := tx.ExecContext(ctx, `ALTER TABLE market_prices ADD COLUMN empire_id TEXT`); err != nil {
				return err
			}
		}

		// Now execute the rest of the migration
		if _, err := tx.ExecContext(ctx, migration.UpSQL); err != nil {
			return err
		}

		// Record as applied
		_, err = tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (migration_id, applied_at) VALUES (?, datetime('now'))`,
			migration.ID,
		)
		return err
	})
}

// GetMigration007 returns the illegal_recipes table migration.
func GetMigration007() (*Migration, error) {
	data, err := migrationFS.ReadFile("migrations/007_illegal_recipes.sql")
	if err != nil {
		return nil, err
	}

	return &Migration{
		ID:      "007_illegal_recipes",
		UpSQL:   string(data),
		DownSQL: `DROP TABLE IF EXISTS illegal_recipes;`,
	}, nil
}

// ApplyMigration007 applies migration 007 (illegal_recipes table).
func ApplyMigration007(ctx context.Context, db *DB) error {
	migration, err := GetMigration007()
	if err != nil {
		return err
	}

	migrator := NewMigrator(db)
	return migrator.Apply(ctx, migration)
}

// GetMigration008 returns the crafting gates removal migration.
func GetMigration008() (*Migration, error) {
	data, err := migrationFS.ReadFile("migrations/008_remove_crafting_gates.sql")
	if err != nil {
		return nil, err
	}

	return &Migration{
		ID:    "008_remove_crafting_gates",
		UpSQL: string(data),
		DownSQL: `
			CREATE TABLE IF NOT EXISTS recipe_skills (
				recipe_id TEXT NOT NULL,
				skill_id TEXT NOT NULL,
				level_required INTEGER NOT NULL,
				PRIMARY KEY (recipe_id, skill_id),
				FOREIGN KEY (recipe_id) REFERENCES recipes(id) ON DELETE CASCADE
			);
		`,
	}, nil
}

// ApplyMigration008 applies migration 008 (remove crafting gates and quality system).
// This migration is safe for both fresh and existing databases.
func ApplyMigration008(ctx context.Context, db *DB) error {
	// Check if already applied
	tracker := NewMigrationTracker(db)
	applied, err := tracker.IsApplied(ctx, "008_remove_crafting_gates")
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	// For fresh databases (built from updated schema.sql), the columns
	// and table may not exist. Check before attempting drops.
	return db.InTransaction(ctx, func(tx *sql.Tx) error {
		// Drop recipe_skills table if it exists
		if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS recipe_skills`); err != nil {
			return err
		}

		// Check and drop columns from recipes table
		for _, col := range []string{"base_quality", "skill_quality_mod", "required_skills"} {
			if hasColumn(ctx, tx, "recipes", col) {
				if _, err := tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE recipes DROP COLUMN %s`, col)); err != nil {
					return err
				}
			}
		}

		// Check and drop quality_mod from recipe_outputs
		if hasColumn(ctx, tx, "recipe_outputs", "quality_mod") {
			if _, err := tx.ExecContext(ctx, `ALTER TABLE recipe_outputs DROP COLUMN quality_mod`); err != nil {
				return err
			}
		}

		// Record as applied
		_, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (migration_id, applied_at) VALUES (?, datetime('now'))`,
			"008_remove_crafting_gates",
		)
		return err
	})
}

// ApplyMigration009 applies migration 009 (catalog fields added after game
// version 0.271.3). It adds recipe flags, the skill empire_restriction column,
// and the items.attributes JSON blob. Each column addition is guarded so the
// migration is safe on both fresh and existing databases.
func ApplyMigration009(ctx context.Context, db *DB) error {
	tracker := NewMigrationTracker(db)
	applied, err := tracker.IsApplied(ctx, "009_add_catalog_fields")
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	type colAdd struct {
		table  string
		column string
		ddl    string
	}
	additions := []colAdd{
		{"recipes", "facility_only", `ALTER TABLE recipes ADD COLUMN facility_only BOOLEAN DEFAULT 0`},
		{"recipes", "no_recycle", `ALTER TABLE recipes ADD COLUMN no_recycle BOOLEAN DEFAULT 0`},
		{"recipes", "fuel_output", `ALTER TABLE recipes ADD COLUMN fuel_output INTEGER DEFAULT 0`},
		{"skills", "empire_restriction", `ALTER TABLE skills ADD COLUMN empire_restriction TEXT`},
		{"items", "attributes", `ALTER TABLE items ADD COLUMN attributes TEXT DEFAULT '{}'`},
	}

	return db.InTransaction(ctx, func(tx *sql.Tx) error {
		for _, a := range additions {
			if hasColumn(ctx, tx, a.table, a.column) {
				continue
			}
			if _, err := tx.ExecContext(ctx, a.ddl); err != nil {
				return fmt.Errorf("adding %s.%s: %w", a.table, a.column, err)
			}
		}

		_, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (migration_id, applied_at) VALUES (?, datetime('now'))`,
			"009_add_catalog_fields",
		)
		return err
	})
}

// hasColumn checks if a table has a specific column.
func hasColumn(ctx context.Context, tx *sql.Tx, table, column string) bool {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false
	}
	defer func() { _ = rows.Close() }()

	var cid int
	var name, ctype string
	var notnull, pk int
	var dfltValue any
	for rows.Next() {
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}
