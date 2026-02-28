package engine

import (
	"context"
	"testing"

	"github.com/rsned/spacemolt-crafting-server/internal/crafting/db"
	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// setupTestEngine creates a test engine with an in-memory database.
func setupTestEngine(t *testing.T) *Engine {
	t.Helper()

	// Create in-memory database
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}

	// Initialize schema
	if err := db.InitSchema(context.Background(), database.DB); err != nil {
		_ = database.Close()
		t.Fatalf("initializing schema: %v", err)
	}

	// Initialize category priorities
	if err := database.CategoryPriorities().InitializeDefaultPriorities(context.Background()); err != nil {
		_ = database.Close()
		t.Fatalf("initializing category priorities: %v", err)
	}

	return New(database)
}

// indexOf finds the index of a recipe with the given category in the results.
// Returns -1 if not found.
func indexOf(results []crafting.CraftableMatch, category string) int {
	for i, r := range results {
		if r.Recipe.Category == category {
			return i
		}
	}
	return -1
}

// TestCraftQuery_CategoryPrioritySorting verifies that craft_query results
// are sorted by category tier, with lower tiers appearing first.
func TestCraftQuery_CategoryPrioritySorting(t *testing.T) {
	ctx := context.Background()
	e := setupTestEngine(t)

	// Query for all recipes with no component filter
	req := crafting.CraftQueryRequest{
		Components:   []crafting.Component{},
		Limit:        100,
		MinMatchRatio: 0.0, // Accept all matches to see sorting
		Strategy:     crafting.StrategyUseInventoryFirst,
	}

	resp, err := e.CraftQuery(ctx, req)
	if err != nil {
		t.Fatalf("CraftQuery failed: %v", err)
	}

	if len(resp.Craftable) == 0 {
		t.Skip("No recipes found in database")
	}

	// Verify that if we have tier 1 and tier 4/5 recipes, tier 1 comes first
	tier1Idx := indexOf(resp.Craftable, "Shipbuilding")
	tier4Idx := indexOf(resp.Craftable, "Mining")
	tier5Idx := indexOf(resp.Craftable, "Components")

	// Only verify if we found recipes from these categories
	foundMultipleTiers := false
	if tier1Idx >= 0 && (tier4Idx >= 0 || tier5Idx >= 0) {
		foundMultipleTiers = true
		if tier4Idx >= 0 && tier1Idx > tier4Idx {
			t.Errorf("Tier 1 category (Shipbuilding) should appear before tier 4 (Mining): got tier1Idx=%d, tier4Idx=%d", tier1Idx, tier4Idx)
		}
		if tier5Idx >= 0 && tier1Idx > tier5Idx {
			t.Errorf("Tier 1 category (Shipbuilding) should appear before tier 5 (Components): got tier1Idx=%d, tier5Idx=%d", tier1Idx, tier5Idx)
		}
	}

	if !foundMultipleTiers {
		t.Skip("Not enough variety in recipe categories to test tier sorting")
	}
}

// TestUnknownCategory_LastInResults verifies that recipes from unknown
// categories appear last in results (tier 6).
func TestUnknownCategory_LastInResults(t *testing.T) {
	ctx := context.Background()
	e := setupTestEngine(t)

	// Get all recipes to check category ordering
	req := crafting.CraftQueryRequest{
		Components:   []crafting.Component{},
		Limit:        100,
		MinMatchRatio: 0.0,
		Strategy:     crafting.StrategyUseInventoryFirst,
	}

	resp, err := e.CraftQuery(ctx, req)
	if err != nil {
		t.Fatalf("CraftQuery failed: %v", err)
	}

	if len(resp.Craftable) == 0 {
		t.Skip("No recipes found")
	}

	// Find any recipe with an unknown category (not in default priorities)
	unknownIdx := -1
	for i, r := range resp.Craftable {
		tier := e.getCategoryTier(r.Recipe.Category)
		if tier == 6 {
			unknownIdx = i
			break
		}
	}

	// If we found an unknown category, verify it's not at the beginning
	if unknownIdx >= 0 && unknownIdx < len(resp.Craftable)/2 {
		t.Errorf("Unknown category (tier 6) should appear in last half of results: found at index %d of %d", unknownIdx, len(resp.Craftable))
	}
}
