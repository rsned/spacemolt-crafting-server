package sync

import (
	"encoding/json"
	"testing"
)

// TestTransformRecipeNewFields verifies the new recipe catalog flags are mapped
// from the import format to the domain type.
func TestTransformRecipeNewFields(t *testing.T) {
	imp := RecipeImport{
		ID:           "facility_widget",
		Name:         "Facility Widget",
		Category:     "Facility Only",
		FacilityOnly: true,
		NoRecycle:    true,
		FuelOutput:   12,
	}
	r := transformRecipe(imp)
	if !r.FacilityOnly {
		t.Error("FacilityOnly not mapped")
	}
	if !r.NoRecycle {
		t.Error("NoRecycle not mapped")
	}
	if r.FuelOutput != 12 {
		t.Errorf("FuelOutput = %d, want 12", r.FuelOutput)
	}
}

// TestTransformSkillEmpireRestriction verifies empire_restriction is mapped.
func TestTransformSkillEmpireRestriction(t *testing.T) {
	imp := SkillImport{ID: "crimson_fury", Name: "Crimson Fury", EmpireRestriction: "crimson"}
	s := transformSkill(imp)
	if s.EmpireRestriction != "crimson" {
		t.Errorf("EmpireRestriction = %q, want crimson", s.EmpireRestriction)
	}
}

// TestExtractItemAttributes verifies core fields are excluded and extra catalog
// fields are preserved verbatim, and that an item with no extra fields yields a
// nil blob.
func TestExtractItemAttributes(t *testing.T) {
	raw := map[string]json.RawMessage{
		"id":         json.RawMessage(`"antimatter_torpedoes"`),
		"name":       json.RawMessage(`"Antimatter Torpedoes"`),
		"category":   json.RawMessage(`"ammo"`),
		"base_value": json.RawMessage(`16300`),
		"slot":       json.RawMessage(`"weapon"`),
		"damage":     json.RawMessage(`42`),
		"effect":     json.RawMessage(`{"type":"ammo"}`),
	}
	got := extractItemAttributes(raw)
	if got == nil {
		t.Fatal("expected non-nil attributes")
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("attributes not valid JSON: %v", err)
	}
	for _, core := range []string{"id", "name", "category", "base_value"} {
		if _, ok := decoded[core]; ok {
			t.Errorf("core field %q leaked into attributes", core)
		}
	}
	for _, extra := range []string{"slot", "damage", "effect"} {
		if _, ok := decoded[extra]; !ok {
			t.Errorf("extra field %q missing from attributes", extra)
		}
	}

	// Item with only core fields -> nil.
	coreOnly := map[string]json.RawMessage{
		"id":   json.RawMessage(`"plain"`),
		"name": json.RawMessage(`"Plain"`),
	}
	if got := extractItemAttributes(coreOnly); got != nil {
		t.Errorf("expected nil attributes for core-only item, got %s", got)
	}
}
