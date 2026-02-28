package mcp

import (
	"context"
	"testing"

	"github.com/rsned/spacemolt-crafting-server/internal/crafting/db"
	"github.com/rsned/spacemolt-crafting-server/internal/crafting/engine"
	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

func TestRecipeMarketProfitability(t *testing.T) {
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
			('comp_steel', 'Steel Component', 100, 'component'),
			('comp_engine', 'Engine Component', 500, 'component')
	`)
	if err != nil {
		t.Fatalf("inserting test items: %v", err)
	}

	// Add test recipes
	_, err = database.ExecContext(ctx, `
		INSERT INTO recipes (id, name, description, category) VALUES
			('craft_steel', 'Steel Component', 'Crafts steel components', 'Components'),
			('craft_engine', 'Engine Component', 'Crafts engine components', 'Components')
	`)
	if err != nil {
		t.Fatalf("inserting test recipes: %v", err)
	}

	// Add recipe inputs/outputs
	_, err = database.ExecContext(ctx, `
		INSERT INTO recipe_inputs (recipe_id, item_id, quantity) VALUES
			('craft_steel', 'ore_iron', 10),
			('craft_engine', 'comp_steel', 2)
	`)
	if err != nil {
		t.Fatalf("inserting recipe inputs: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO recipe_outputs (recipe_id, item_id, quantity, quality_mod) VALUES
			('craft_steel', 'comp_steel', 1, false),
			('craft_engine', 'comp_engine', 1, false)
	`)
	if err != nil {
		t.Fatalf("inserting recipe outputs: %v", err)
	}

	// Add market stats for some items (ore_iron has market data, comp_steel doesn't)
	market := db.NewMarketStore(database)
	_, err = database.ExecContext(ctx, `
		INSERT INTO market_order_book
		(batch_id, item_id, station_id, order_type, price_per_unit, volume_available, recorded_at)
		VALUES
			('batch1', 'ore_iron', 'Test Station', 'sell', 5, 1000, datetime('now')),
			('batch1', 'ore_iron', 'Test Station', 'sell', 7, 500, datetime('now')),
			('batch1', 'ore_iron', 'Test Station', 'buy', 3, 800, datetime('now'))
	`)
	if err != nil {
		t.Fatalf("inserting market orders: %v", err)
	}

	err = market.RecalculatePriceStats(ctx, "ore_iron", "Test Station")
	if err != nil {
		t.Fatalf("recalculating stats: %v", err)
	}

	eng := engine.New(database)

	t.Run("returns all recipes with market profitability", func(t *testing.T) {
		result, err := eng.RecipeMarketProfitability(ctx, "Test Station", "")
		if err != nil {
			t.Fatalf("RecipeMarketProfitability failed: %v", err)
		}

		if result == nil {
			t.Fatal("expected result, got nil")
		}

		if len(result.Recipes) != 2 {
			t.Errorf("expected 2 recipes, got %d", len(result.Recipes))
		}

		// Find craft_steel recipe
		var steelRecipe *crafting.RecipeMarketProfit
		for _, r := range result.Recipes {
			if r.RecipeID == "craft_steel" {
				steelRecipe = &r
				break
			}
		}

		if steelRecipe == nil {
			t.Fatal("craft_steel recipe not found")
		}

		// ore_iron has buy orders at 3, comp_steel doesn't (MSRP 100)
		// Input: 10 ore_iron @ buy price 3 = 30
		// Output: 1 comp_steel @ MSRP 100 = 100
		// Profit = 100 - 30 = 70
		if steelRecipe.OutputSellPrice != 100 {
			t.Errorf("expected output sell price 100 (MSRP), got %d", steelRecipe.OutputSellPrice)
		}

		if steelRecipe.InputCost != 30 {
			t.Errorf("expected input cost 30, got %d", steelRecipe.InputCost)
		}

		if steelRecipe.Profit != 70 {
			t.Errorf("expected profit 70, got %d", steelRecipe.Profit)
		}

		if steelRecipe.OutputMSRP != 100 {
			t.Errorf("expected output MSRP 100, got %d", steelRecipe.OutputMSRP)
		}

		// Output used MSRP (no market data), should be marked
		if !steelRecipe.OutputUsesMSRP {
			t.Error("expected output to be marked as using MSRP")
		}

		// Input has market data, should not be marked
		if steelRecipe.InputUsesMSRP {
			t.Error("expected input to NOT be marked as using MSRP (has market data)")
		}
	})

	t.Run("sorts by absolute profit descending", func(t *testing.T) {
		result, err := eng.RecipeMarketProfitability(ctx, "Test Station", "")
		if err != nil {
			t.Fatalf("RecipeMarketProfitability failed: %v", err)
		}

		if len(result.Recipes) < 2 {
			t.Fatal("expected at least 2 recipes")
		}

		// Check sorted by profit descending
		for i := 1; i < len(result.Recipes); i++ {
			prevProfit := result.Recipes[i-1].Profit
			currProfit := result.Recipes[i].Profit
			if prevProfit < currProfit {
				t.Errorf("recipes not sorted by profit descending: recipe %d has profit %d, recipe %d has profit %d",
					i-1, prevProfit, i, currProfit)
			}
		}
	})

	t.Run("works without station (uses MSRP for all)", func(t *testing.T) {
		result, err := eng.RecipeMarketProfitability(ctx, "", "")
		if err != nil {
			t.Fatalf("RecipeMarketProfitability failed: %v", err)
		}

		if result == nil {
			t.Fatal("expected result, got nil")
		}

		// All should use MSRP when no station specified
		for _, recipe := range result.Recipes {
			if !recipe.OutputUsesMSRP {
				t.Errorf("recipe %s: expected output to use MSRP when no station specified", recipe.RecipeID)
			}
			if !recipe.InputUsesMSRP {
				t.Errorf("recipe %s: expected input to use MSRP when no station specified", recipe.RecipeID)
			}
		}
	})
}
