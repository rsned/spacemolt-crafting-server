package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// ItemStore handles item data access.
type ItemStore struct {
	db *DB
}

// NewItemStore creates a new ItemStore.
func NewItemStore(db *DB) *ItemStore {
	return &ItemStore{db: db}
}

// BulkInsertItems inserts multiple items in a transaction.
func (s *ItemStore) BulkInsertItems(ctx context.Context, items []crafting.Item) error {
	return s.db.InTransaction(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT OR REPLACE INTO items
			(id, name, description, category, rarity, size, base_value, stackable, tradeable, attributes)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("preparing item statement: %w", err)
		}
		defer func() { _ = stmt.Close() }()

		for _, item := range items {
			attributes := "{}"
			if len(item.Attributes) > 0 {
				attributes = string(item.Attributes)
			}
			_, err := stmt.ExecContext(ctx,
				item.ID, item.Name, item.Description, item.Category,
				item.Rarity, item.Size, item.BaseValue, item.Stackable, item.Tradeable, attributes,
			)
			if err != nil {
				return fmt.Errorf("inserting item %s: %w", item.ID, err)
			}
		}

		return nil
	})
}

// ClearItems removes all item data.
func (s *ItemStore) ClearItems(ctx context.Context) error {
	return s.db.InTransaction(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM items`)
		return err
	})
}
