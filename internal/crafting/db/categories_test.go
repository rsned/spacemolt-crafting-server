package db

import (
	"context"
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()

	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}

	if err := InitSchema(context.Background(), db.DB); err != nil {
		_ = db.Close()
		t.Fatalf("initializing schema: %v", err)
	}

	return db
}

func TestGetPriorityTier_KnownCategory(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer func() { _ = db.Close() }()

	store := NewCategoryPriorityStore(db)

	// Initialize defaults
	err := store.InitializeDefaultPriorities(ctx)
	if err != nil {
		t.Fatalf("InitializeDefaultPriorities failed: %v", err)
	}

	// Test known high-priority category
	tier, err := store.GetPriorityTier(ctx, "Shipbuilding")
	if err != nil {
		t.Fatalf("GetPriorityTier failed: %v", err)
	}
	if tier != 1 {
		t.Errorf("Expected tier 1 for Shipbuilding, got %d", tier)
	}
}

func TestGetPriorityTier_UnknownCategory(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer func() { _ = db.Close() }()

	store := NewCategoryPriorityStore(db)

	// Initialize defaults
	err := store.InitializeDefaultPriorities(ctx)
	if err != nil {
		t.Fatalf("InitializeDefaultPriorities failed: %v", err)
	}

	// Test unknown category returns default tier 6
	tier, err := store.GetPriorityTier(ctx, "UnknownCategory")
	if err != nil {
		t.Fatalf("GetPriorityTier failed: %v", err)
	}
	if tier != 6 {
		t.Errorf("Expected tier 6 for unknown category, got %d", tier)
	}
}

func TestGetAllCategories(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer func() { _ = db.Close() }()

	store := NewCategoryPriorityStore(db)

	// Initialize defaults
	err := store.InitializeDefaultPriorities(ctx)
	if err != nil {
		t.Fatalf("InitializeDefaultPriorities failed: %v", err)
	}

	// Get all categories
	categories, err := store.GetAllCategories(ctx)
	if err != nil {
		t.Fatalf("GetAllCategories failed: %v", err)
	}

	// Verify expected categories exist
	expectedCategories := []string{
		"Shipbuilding", "Legendary", "Utility", "Mining",
		"Components", "Weapons", "Refining",
	}

	for _, cat := range expectedCategories {
		tier, ok := categories[cat]
		if !ok {
			t.Errorf("Category %s not found in map", cat)
		}
		if tier < 1 || tier > 6 {
			t.Errorf("Invalid tier %d for category %s", tier, cat)
		}
	}
}

func TestInitializeDefaultPriorities(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer func() { _ = db.Close() }()

	store := NewCategoryPriorityStore(db)

	// First call should insert defaults
	err := store.InitializeDefaultPriorities(ctx)
	if err != nil {
		t.Fatalf("First InitializeDefaultPriorities failed: %v", err)
	}

	// Second call should be idempotent (no error on duplicate keys)
	err = store.InitializeDefaultPriorities(ctx)
	if err != nil {
		t.Fatalf("Second InitializeDefaultPriorities failed: %v", err)
	}

	// Verify a specific category was inserted
	tier, err := store.GetPriorityTier(ctx, "Legendary")
	if err != nil {
		t.Fatalf("GetPriorityTier failed: %v", err)
	}
	if tier != 1 {
		t.Errorf("Expected tier 1 for Legendary, got %d", tier)
	}
}
