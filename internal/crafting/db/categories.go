package db

import (
	"context"
	"database/sql"
	"fmt"
)

// CategoryPriorityStore handles category priority data access.
type CategoryPriorityStore struct {
	db *DB
}

// NewCategoryPriorityStore creates a new CategoryPriorityStore.
func NewCategoryPriorityStore(db *DB) *CategoryPriorityStore {
	return &CategoryPriorityStore{db: db}
}

// GetPriorityTier returns the priority tier for a category (1-6).
// Returns 6 (lowest) for unlisted categories.
func (s *CategoryPriorityStore) GetPriorityTier(ctx context.Context, category string) (int, error) {
	var tier sql.NullInt64

	err := s.db.QueryRowContext(ctx, `
		SELECT priority_tier FROM category_priorities WHERE category = ?
	`, category).Scan(&tier)

	if err == sql.ErrNoRows {
		// Unlisted category gets default tier 6
		return 6, nil
	}
	if err != nil {
		return 6, fmt.Errorf("querying category tier: %w", err)
	}

	if !tier.Valid {
		return 6, nil
	}

	return int(tier.Int64), nil
}

// GetAllCategories returns all categories with their priority tiers.
func (s *CategoryPriorityStore) GetAllCategories(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT category, priority_tier FROM category_priorities
	`)
	if err != nil {
		return nil, fmt.Errorf("querying all categories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	categories := make(map[string]int)
	for rows.Next() {
		var category string
		var tier int
		if err := rows.Scan(&category, &tier); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}
		categories[category] = tier
	}

	return categories, rows.Err()
}

// InitializeDefaultPriorities populates the table with default priority tiers.
// Uses INSERT OR IGNORE to be idempotent.
func (s *CategoryPriorityStore) InitializeDefaultPriorities(ctx context.Context) error {
	// Default priority tiers as specified in design
	defaults := map[string]int{
		"Shipbuilding":       1,
		"Legendary":          1,
		"Utility":            2,
		"Mining":             2,
		"Gas Processing":     2,
		"Ice Refining":       2,
		"Equipment":          2,
		"Components":         3,
		"Weapons":            4,
		"Drones":             4,
		"Electronic Warfare": 4,
		"Defense":            4,
		"Stealth":            4,
		"Refining":           5,
	}

	return s.db.InTransaction(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT OR IGNORE INTO category_priorities (category, priority_tier)
			VALUES (?, ?)
		`)
		if err != nil {
			return fmt.Errorf("preparing statement: %w", err)
		}
		defer func() { _ = stmt.Close() }()

		for category, tier := range defaults {
			_, err := stmt.ExecContext(ctx, category, tier)
			if err != nil {
				return fmt.Errorf("inserting category %s: %w", category, err)
			}
		}

		return nil
	})
}
