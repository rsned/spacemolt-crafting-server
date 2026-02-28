package engine

import (
	"context"
	"testing"

	"github.com/rsned/spacemolt-crafting-server/internal/crafting/db"
	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

func TestCalculateProfitAnalysisWithMarketStats(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer func() { _ = database.Close() }()

	// Initialize schema and migration
	if err := db.InitSchema(ctx, database.DB); err != nil {
		t.Fatalf("initializing schema: %v", err)
	}

	if err := db.ApplyMigration005(ctx, database); err != nil {
		t.Fatalf("applying migration 005: %v", err)
	}

	// Add test items
	_, err = database.ExecContext(ctx, `
		INSERT INTO items (id, name, base_value, category) VALUES
			('ore_iron', 'Iron Ore', 1, 'ore'),
			('comp_steel', 'Steel Component', 100, 'component')
	`)
	if err != nil {
		t.Fatalf("inserting test items: %v", err)
	}

	// Add market price stats (simulating real market data)
	_, err = database.ExecContext(ctx, `
		INSERT INTO market_price_stats
		(item_id, station_id, empire_id, order_type, stat_method, representative_price,
		 sample_count, total_volume, min_price, max_price, stddev, confidence_score, last_updated)
		VALUES
			('comp_steel', 'Test Station', NULL, 'sell', 'volume_weighted', 150,
			 50, 10000, 140, 160, 5.5, 0.95, datetime('now')),
			('comp_steel', 'Test Station', NULL, 'buy', 'second_price', 120,
			 30, 5000, 110, 130, 3.2, 0.85, datetime('now')),
			('ore_iron', 'Test Station', NULL, 'buy', 'median', 5,
			 10, 1000, 3, 8, 1.5, 0.7, datetime('now'))
	`)
	if err != nil {
		t.Fatalf("inserting market stats: %v", err)
	}

	// Create engine
	eng := New(database)

	// Create a test recipe
	recipe := &crafting.Recipe{
		ID:   "recipe_steel",
		Name: "Steel Component",
		Inputs: []crafting.RecipeInput{
			{ItemID: "ore_iron", Quantity: 10},
		},
		Outputs: []crafting.RecipeOutput{
			{ItemID: "comp_steel", Quantity: 1},
		},
	}

	t.Run("calculates profit with market data", func(t *testing.T) {
		analysis, err := eng.calculateProfitAnalysis(ctx, recipe, "Test Station", 5)
		if err != nil {
			t.Fatalf("calculateProfitAnalysis failed: %v", err)
		}

		if analysis == nil {
			t.Fatal("expected analysis, got nil")
		}

		// Output: 1 comp_steel at 150 = 150
		// Input: 10 ore_iron at 5 = 50
		// Profit: 150 - 50 = 100
		if analysis.OutputSellPrice != 150 {
			t.Errorf("expected output sell price 150, got %d", analysis.OutputSellPrice)
		}

		if analysis.InputCost != 50 {
			t.Errorf("expected input cost 50, got %d", analysis.InputCost)
		}

		if analysis.ProfitPerUnit != 100 {
			t.Errorf("expected profit per unit 100, got %d", analysis.ProfitPerUnit)
		}

		// NEW fields from Phase 3
		if analysis.MSRP != 100 {
			t.Errorf("expected MSRP 100, got %d", analysis.MSRP)
		}

		if analysis.MarketStatus != "high_confidence" {
			t.Errorf("expected market status 'high_confidence', got '%s'", analysis.MarketStatus)
		}

		if analysis.PricingMethod != "volume_weighted" {
			t.Errorf("expected pricing method 'volume_weighted', got '%s'", analysis.PricingMethod)
		}

		if analysis.SampleCount != 50 {
			t.Errorf("expected sample count 50, got %d", analysis.SampleCount)
		}
	})

	t.Run("returns nil when no station specified", func(t *testing.T) {
		analysis, err := eng.calculateProfitAnalysis(ctx, recipe, "", 5)
		if err != nil {
			t.Fatalf("calculateProfitAnalysis failed: %v", err)
		}

		if analysis != nil {
			t.Error("expected nil analysis when no station specified, got analysis")
		}
	})
}
