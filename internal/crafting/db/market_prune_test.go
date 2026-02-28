package db

import (
	"context"
	"testing"
	"time"
)

func TestPruneOldOrders(t *testing.T) {
	ctx := context.Background()
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer func() { _ = database.Close() }()

	// Initialize schema and migration
	if err := InitSchema(ctx, database.DB); err != nil {
		t.Fatalf("initializing schema: %v", err)
	}

	if err := ApplyMigration005(ctx, database); err != nil {
		t.Fatalf("applying migration 005: %v", err)
	}

	// Add test item
	_, err = database.ExecContext(ctx, `
		INSERT INTO items (id, name, base_value, category) VALUES
			('ore_iron', 'Iron Ore', 1, 'ore')
	`)
	if err != nil {
		t.Fatalf("inserting test item: %v", err)
	}

	market := NewMarketStore(database)

	// Insert orders at different times
	now := time.Now()
	oldTime := now.Add(-10 * 24 * time.Hour) // 10 days ago
	recentTime := now.Add(-1 * 24 * time.Hour) // 1 day ago

	_, err = database.ExecContext(ctx, `
		INSERT INTO market_order_book
		(batch_id, item_id, station_id, order_type, price_per_unit, volume_available, recorded_at)
		VALUES
			('old_batch', 'ore_iron', 'Station A', 'sell', 10, 100, ?),
			('recent_batch', 'ore_iron', 'Station A', 'sell', 20, 200, ?)
	`, oldTime.Format(time.RFC3339), recentTime.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("inserting test orders: %v", err)
	}

	// Verify we have 2 orders
	var countBefore int
	err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM market_order_book`).Scan(&countBefore)
	if err != nil {
		t.Fatalf("counting orders before: %v", err)
	}
	if countBefore != 2 {
		t.Errorf("expected 2 orders before pruning, got %d", countBefore)
	}

	// Prune orders older than 7 days
	deleted, err := market.PruneOldOrders(ctx, 7)
	if err != nil {
		t.Fatalf("pruning orders: %v", err)
	}

	if deleted != 1 {
		t.Errorf("expected 1 order deleted, got %d", deleted)
	}

	// Verify we now have 1 order
	var countAfter int
	err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM market_order_book`).Scan(&countAfter)
	if err != nil {
		t.Fatalf("counting orders after: %v", err)
	}
	if countAfter != 1 {
		t.Errorf("expected 1 order after pruning, got %d", countAfter)
	}

	// Verify the recent order still exists
	var itemID string
	err = database.QueryRowContext(ctx, `SELECT item_id FROM market_order_book WHERE batch_id = 'recent_batch'`).Scan(&itemID)
	if err != nil {
		t.Errorf("recent order should still exist: %v", err)
	}

	// Verify the old order was deleted
	var oldExists int
	err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM market_order_book WHERE batch_id = 'old_batch'`).Scan(&oldExists)
	if err != nil {
		t.Fatalf("checking old order: %v", err)
	}
	if oldExists != 0 {
		t.Error("old order should have been deleted")
	}
}
