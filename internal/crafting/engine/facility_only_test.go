package engine

import (
	"context"
	"testing"

	"github.com/rsned/spacemolt-crafting-server/internal/crafting/db"
	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// TestCraftQuery_FacilityOnlyFiltering verifies facility-only recipes are
// excluded by default and included when IncludeFacilityOnly is set.
func TestCraftQuery_FacilityOnlyFiltering(t *testing.T) {
	ctx := context.Background()

	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	defer func() { _ = database.Close() }()
	if err := db.InitSchema(ctx, database.DB); err != nil {
		t.Fatalf("initializing schema: %v", err)
	}
	if err := database.CategoryPriorities().InitializeDefaultPriorities(ctx); err != nil {
		t.Fatalf("initializing category priorities: %v", err)
	}

	recipeStore := db.NewRecipeStore(database)
	recipes := []crafting.Recipe{
		{
			ID:       "ship_widget",
			Name:     "Ship Widget",
			Category: "Components",
			Inputs:   []crafting.RecipeInput{{ItemID: "iron", Quantity: 1}},
			Outputs:  []crafting.RecipeOutput{{ItemID: "widget", Quantity: 1}},
		},
		{
			ID:           "facility_widget",
			Name:         "Facility Widget",
			Category:     "Components",
			FacilityOnly: true,
			Inputs:       []crafting.RecipeInput{{ItemID: "iron", Quantity: 1}},
			Outputs:      []crafting.RecipeOutput{{ItemID: "fwidget", Quantity: 1}},
		},
	}
	if err := recipeStore.BulkInsertRecipes(ctx, recipes); err != nil {
		t.Fatalf("BulkInsertRecipes failed: %v", err)
	}

	e := New(database)
	base := crafting.CraftQueryRequest{
		Components:    []crafting.Component{{ID: "iron", Quantity: 10}},
		Limit:         100,
		MinMatchRatio: 0.0,
		Strategy:      crafting.StrategyUseInventoryFirst,
	}

	contains := func(matches []crafting.CraftableMatch, id string) bool {
		for _, m := range matches {
			if m.Recipe.ID == id {
				return true
			}
		}
		return false
	}

	// Default: facility-only excluded, normal included.
	resp, err := e.CraftQuery(ctx, base)
	if err != nil {
		t.Fatalf("CraftQuery failed: %v", err)
	}
	if !contains(resp.Craftable, "ship_widget") {
		t.Error("expected ship_widget in default results")
	}
	if contains(resp.Craftable, "facility_widget") {
		t.Error("facility_widget should be excluded by default")
	}

	// With flag: facility-only included.
	withFlag := base
	withFlag.IncludeFacilityOnly = true
	resp2, err := e.CraftQuery(ctx, withFlag)
	if err != nil {
		t.Fatalf("CraftQuery with IncludeFacilityOnly failed: %v", err)
	}
	if !contains(resp2.Craftable, "facility_widget") {
		t.Error("facility_widget should be included when IncludeFacilityOnly=true")
	}
}
