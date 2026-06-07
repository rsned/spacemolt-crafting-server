package db

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// TestRecipeNewFieldsRoundTrip verifies facility_only, no_recycle, and
// fuel_output survive a write/read cycle through both GetRecipe and
// GetAllRecipes.
func TestRecipeNewFieldsRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer func() { _ = db.Close() }()

	store := NewRecipeStore(db)
	recipes := []crafting.Recipe{
		{
			ID:           "onboard_h2o2_fuel_synthesis",
			Name:         "Onboard H2O2 Fuel Synthesis",
			Category:     "Ship Passive",
			CraftingTime: 0,
			FuelOutput:   12,
			Inputs:       []crafting.RecipeInput{{ItemID: "water_ice", Quantity: 8}},
		},
		{
			ID:           "facility_widget",
			Name:         "Facility Widget",
			Category:     "Facility Only",
			FacilityOnly: true,
			NoRecycle:    true,
			Outputs:      []crafting.RecipeOutput{{ItemID: "widget", Quantity: 1}},
		},
	}
	if err := store.BulkInsertRecipes(ctx, recipes); err != nil {
		t.Fatalf("BulkInsertRecipes failed: %v", err)
	}

	got, err := store.GetRecipe(ctx, "onboard_h2o2_fuel_synthesis")
	if err != nil {
		t.Fatalf("GetRecipe failed: %v", err)
	}
	if got.FuelOutput != 12 {
		t.Errorf("FuelOutput = %d, want 12", got.FuelOutput)
	}
	if got.FacilityOnly || got.NoRecycle {
		t.Errorf("expected fuel recipe to be non-facility, recyclable; got facility=%v no_recycle=%v", got.FacilityOnly, got.NoRecycle)
	}

	got2, err := store.GetRecipe(ctx, "facility_widget")
	if err != nil {
		t.Fatalf("GetRecipe failed: %v", err)
	}
	if !got2.FacilityOnly {
		t.Error("FacilityOnly = false, want true")
	}
	if !got2.NoRecycle {
		t.Error("NoRecycle = false, want true")
	}

	all, err := store.GetAllRecipes(ctx)
	if err != nil {
		t.Fatalf("GetAllRecipes failed: %v", err)
	}
	byID := make(map[string]crafting.Recipe, len(all))
	for _, r := range all {
		byID[r.ID] = r
	}
	if byID["facility_widget"].FacilityOnly != true {
		t.Error("GetAllRecipes did not preserve FacilityOnly")
	}
	if byID["onboard_h2o2_fuel_synthesis"].FuelOutput != 12 {
		t.Error("GetAllRecipes did not preserve FuelOutput")
	}
}

// TestSkillEmpireRestrictionRoundTrip verifies empire_restriction survives a
// write/read cycle and that unrestricted skills read back as empty.
func TestSkillEmpireRestrictionRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer func() { _ = db.Close() }()

	store := NewSkillStore(db)
	skills := []crafting.Skill{
		{ID: "crimson_fury", Name: "Crimson Fury", Category: "Combat", MaxLevel: 100, EmpireRestriction: "crimson"},
		{ID: "armor", Name: "Armor", Category: "Combat", MaxLevel: 100},
	}
	if err := store.BulkInsertSkills(ctx, skills); err != nil {
		t.Fatalf("BulkInsertSkills failed: %v", err)
	}

	restricted, err := store.GetSkill(ctx, "crimson_fury")
	if err != nil {
		t.Fatalf("GetSkill failed: %v", err)
	}
	if restricted.EmpireRestriction != "crimson" {
		t.Errorf("EmpireRestriction = %q, want %q", restricted.EmpireRestriction, "crimson")
	}

	unrestricted, err := store.GetSkill(ctx, "armor")
	if err != nil {
		t.Fatalf("GetSkill failed: %v", err)
	}
	if unrestricted.EmpireRestriction != "" {
		t.Errorf("EmpireRestriction = %q, want empty", unrestricted.EmpireRestriction)
	}
}

// TestItemAttributesRoundTrip verifies the items.attributes JSON blob is
// persisted and read back intact.
func TestItemAttributesRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer func() { _ = db.Close() }()

	store := NewItemStore(db)
	attrs := json.RawMessage(`{"slot":"weapon","damage":42}`)
	items := []crafting.Item{
		{ID: "antimatter_torpedoes", Name: "Antimatter Torpedoes", Category: "ammo", Attributes: attrs},
		{ID: "plain_item", Name: "Plain Item", Category: "module"},
	}
	if err := store.BulkInsertItems(ctx, items); err != nil {
		t.Fatalf("BulkInsertItems failed: %v", err)
	}

	var stored string
	if err := db.QueryRowContext(ctx, `SELECT attributes FROM items WHERE id = ?`, "antimatter_torpedoes").Scan(&stored); err != nil {
		t.Fatalf("querying attributes: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stored), &decoded); err != nil {
		t.Fatalf("attributes is not valid JSON: %v (%q)", err, stored)
	}
	if decoded["slot"] != "weapon" {
		t.Errorf("attributes slot = %v, want weapon", decoded["slot"])
	}

	var plain string
	if err := db.QueryRowContext(ctx, `SELECT attributes FROM items WHERE id = ?`, "plain_item").Scan(&plain); err != nil {
		t.Fatalf("querying plain attributes: %v", err)
	}
	if plain != "{}" {
		t.Errorf("plain item attributes = %q, want {}", plain)
	}
}
