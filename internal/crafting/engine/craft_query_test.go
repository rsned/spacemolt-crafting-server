package engine

import (
	"context"
	"testing"

	"github.com/rsned/spacemolt-crafting-server/internal/crafting/db"
	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// testEngine creates a test engine with schema and migration initialized.
func testEngine(t *testing.T) *Engine {
	t.Helper()

	ctx := context.Background()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}

	// Initialize schema
	if err := db.InitSchema(ctx, database.DB); err != nil {
		_ = database.Close()
		t.Fatalf("initializing schema: %v", err)
	}

	// Initialize category priorities
	if err := database.CategoryPriorities().InitializeDefaultPriorities(ctx); err != nil {
		_ = database.Close()
		t.Fatalf("initializing category priorities: %v", err)
	}

	// Apply migration 007 for illegal recipes
	if err := db.ApplyMigration007(ctx, database); err != nil {
		_ = database.Close()
		t.Fatalf("applying migration 007: %v", err)
	}

	// Create a new engine instance (not closing the DB)
	return New(database)
}

// TestCraftQuery_IllegalRecipes verifies that illegal recipe status
// is properly populated in craft_query results.
func TestCraftQuery_IllegalRecipes(t *testing.T) {
	ctx := context.Background()
	engine := testEngine(t)

	// Add test recipe
	database := engine.db
	_, err := database.ExecContext(ctx, `
		INSERT INTO recipes (id, name, description, category) VALUES
			('craft_ammo_std', 'Standard Ammo', 'Standard ammunition', 'Ammunition')
	`)
	if err != nil {
		t.Fatalf("inserting test recipe: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO recipe_inputs (recipe_id, item_id, quantity) VALUES
			('craft_ammo_std', 'ore_iron', 10),
			('craft_ammo_std', 'chem_propellant', 5)
	`)
	if err != nil {
		t.Fatalf("inserting recipe inputs: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO recipe_outputs (recipe_id, item_id, quantity) VALUES
			('craft_ammo_std', 'ammo_std', 100)
	`)
	if err != nil {
		t.Fatalf("inserting recipe outputs: %v", err)
	}

	// Mark a recipe as illegal
	err = engine.illegalStore.MarkIllegal(ctx, "craft_ammo_std", "test ban", "test location")
	if err != nil {
		t.Fatalf("failed to mark illegal: %v", err)
	}

	// Query that would return the illegal recipe
	results, err := engine.CraftQuery(ctx, crafting.CraftQueryRequest{
		Components: []crafting.Component{
			{ID: "ore_iron", Quantity: 10},
			{ID: "chem_propellant", Quantity: 5},
		},
		IncludeAmmunition: true,
	})
	if err != nil {
		t.Fatalf("craft query failed: %v", err)
	}

	// Find the illegal recipe in results
	var illegalRecipe *crafting.Recipe
	for i := range results.Craftable {
		if results.Craftable[i].Recipe.ID == "craft_ammo_std" {
			illegalRecipe = &results.Craftable[i].Recipe
			break
		}
	}

	// Also check partial results
	if illegalRecipe == nil {
		for i := range results.PartialComponents {
			if results.PartialComponents[i].Recipe.ID == "craft_ammo_std" {
				illegalRecipe = &results.PartialComponents[i].Recipe
				break
			}
		}
	}

	// Also check blocked by skills results
	if illegalRecipe == nil {
		for i := range results.BlockedBySkills {
			if results.BlockedBySkills[i].Recipe.ID == "craft_ammo_std" {
				illegalRecipe = &results.BlockedBySkills[i].Recipe
				break
			}
		}
	}

	if illegalRecipe == nil {
		t.Fatal("illegal recipe not found in results")
	}

	// Verify illegal status is set
	if illegalRecipe.IllegalStatus == nil {
		t.Fatal("expected illegal_status to be set")
	}
	if !illegalRecipe.IllegalStatus.IsIllegal {
		t.Error("expected IsIllegal to be true")
	}
	if illegalRecipe.IllegalStatus.BanReason != "test ban" {
		t.Errorf("expected ban reason 'test ban', got '%s'", illegalRecipe.IllegalStatus.BanReason)
	}
}
