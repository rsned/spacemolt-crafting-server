// Package sync handles synchronization of data from SpaceMolt.
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/rsned/spacemolt-crafting-server/internal/crafting/db"
	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// Syncer handles data synchronization from SpaceMolt.
type Syncer struct {
	db *db.DB
}

// NewSyncer creates a new Syncer.
func NewSyncer(database *db.DB) *Syncer {
	return &Syncer{db: database}
}

// unwrapItems tries to unmarshal data as a {"items": [...]} envelope first,
// falling back to the raw data as a plain array.
func unwrapItems(data []byte) (json.RawMessage, error) {
	var envelope struct {
		Items json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Items) > 0 {
		return envelope.Items, nil
	}
	return data, nil
}

// ItemImport represents the expected format of item data from SpaceMolt.
type ItemImport struct {
	ID          string `json:"id"`
	TypeID      string `json:"type_id,omitempty"` // Fallback for ID
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	Type        string `json:"type,omitempty"` // Fallback for category
	Rarity      string `json:"rarity,omitempty"`
	Size        int    `json:"size,omitempty"`
	BaseValue   int    `json:"base_value,omitempty"`
	Stackable   bool   `json:"stackable,omitempty"`
	Tradeable   bool   `json:"tradeable,omitempty"`

	// Non-standard fields to ignore
	CPUUsage    int    `json:"cpu_usage,omitempty"`
	PowerUsage  int    `json:"power_usage,omitempty"`
	ShieldBonus int    `json:"shield_bonus,omitempty"`
	Special     string `json:"special,omitempty"`
}

// RecipeImport represents the expected format of recipe data from SpaceMolt.
type RecipeImport struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Category        string `json:"category,omitempty"`
	CraftingTime    int    `json:"crafting_time,omitempty"`
	BaseQuality     int    `json:"base_quality,omitempty"`
	SkillQualityMod int    `json:"skill_quality_mod,omitempty"`

	// Inputs (was components)
	Inputs []struct {
		ID       string `json:"id,omitempty"`
		ItemID   string `json:"item_id,omitempty"`
		Quantity int    `json:"quantity"`
	} `json:"inputs,omitempty"`

	// Components (legacy support)
	Components []struct {
		ID       string `json:"id,omitempty"`
		ItemID   string `json:"item_id,omitempty"`
		Quantity int    `json:"quantity"`
	} `json:"components,omitempty"`

	// Outputs - now supports multiple
	Outputs []struct {
		ItemID     string `json:"item_id,omitempty"`
		ID         string `json:"id,omitempty"`
		Quantity   int    `json:"quantity"`
		QualityMod bool   `json:"quality_mod,omitempty"`
	} `json:"outputs,omitempty"`

	// Skills may be in various formats
	Skills []struct {
		ID            string `json:"id,omitempty"`
		SkillID       string `json:"skill_id,omitempty"`
		Level         int    `json:"level,omitempty"`
		LevelRequired int    `json:"level_required,omitempty"`
	} `json:"skills,omitempty"`

	// RequiredSkills as a map (catalog format: {"crafting_advanced": 2})
	RequiredSkills map[string]int `json:"required_skills,omitempty"`

	// Legacy single output support
	Output struct {
		ItemID   string `json:"item_id,omitempty"`
		ID       string `json:"id,omitempty"`
		Quantity int    `json:"quantity"`
	} `json:"output,omitempty"`
	OutputItemID   string `json:"output_item_id,omitempty"`
	OutputQuantity int    `json:"output_quantity,omitempty"`
}

// SkillImport represents the expected format of skill data from SpaceMolt.
type SkillImport struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	Category       string          `json:"category,omitempty"`
	MaxLevel       int             `json:"max_level,omitempty"`
	TrainingSource string          `json:"training_source,omitempty"`
	XPPerLevel     json.RawMessage `json:"xp_per_level,omitempty"`
	BonusPerLevel  json.RawMessage `json:"bonus_per_level,omitempty"`
	RequiredSkills json.RawMessage `json:"required_skills,omitempty"`

	Prerequisites []struct {
		SkillID string `json:"skill_id,omitempty"`
		ID      string `json:"id,omitempty"`
		Level   int    `json:"level,omitempty"`
	} `json:"prerequisites,omitempty"`

	// XP thresholds per level
	Levels []struct {
		Level      int `json:"level"`
		XPRequired int `json:"xp_required,omitempty"`
		XP         int `json:"xp,omitempty"`
	} `json:"levels,omitempty"`

	XPThresholds []int `json:"xp_thresholds,omitempty"`
}

// ImportItemsFromFile imports items from a JSON file.
func (s *Syncer) ImportItemsFromFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	itemsData, err := unwrapItems(data)
	if err != nil {
		return fmt.Errorf("unwrapping items: %w", err)
	}

	var imports []ItemImport
	if err := json.Unmarshal(itemsData, &imports); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}

	items := make([]crafting.Item, 0, len(imports))
	for _, imp := range imports {
		// Use type_id as fallback for empty id
		id := imp.ID
		if id == "" {
			id = imp.TypeID
		}
		if id == "" {
			continue // Still no id, skip this entry
		}

		// Use type as fallback for empty category, default to "module"
		category := imp.Category
		if category == "" {
			category = imp.Type
		}
		if category == "" {
			category = "module"
		}

		items = append(items, crafting.Item{
			ID:          id,
			Name:        imp.Name,
			Description: imp.Description,
			Category:    category,
			Rarity:      imp.Rarity,
			Size:        imp.Size,
			BaseValue:   imp.BaseValue,
			Stackable:   imp.Stackable,
			Tradeable:   imp.Tradeable,
		})
	}

	itemStore := db.NewItemStore(s.db)
	if err := itemStore.BulkInsertItems(ctx, items); err != nil {
		return fmt.Errorf("inserting items: %w", err)
	}

	if err := s.db.SetSyncMetadata(ctx, "items_last_sync", time.Now().Format(time.RFC3339)); err != nil {
		return err
	}
	if err := s.db.SetSyncMetadata(ctx, "items_count", fmt.Sprintf("%d", len(items))); err != nil {
		return err
	}

	return nil
}

// ImportRecipesFromFile imports recipes from a JSON file.
func (s *Syncer) ImportRecipesFromFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	itemsData, err := unwrapItems(data)
	if err != nil {
		return fmt.Errorf("unwrapping items: %w", err)
	}

	var imports []RecipeImport
	if err := json.Unmarshal(itemsData, &imports); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}

	recipes := make([]crafting.Recipe, 0, len(imports))
	for _, imp := range imports {
		recipe := transformRecipe(imp)
		recipes = append(recipes, recipe)
	}

	recipeStore := db.NewRecipeStore(s.db)
	if err := recipeStore.BulkInsertRecipes(ctx, recipes); err != nil {
		return fmt.Errorf("inserting recipes: %w", err)
	}

	// Update sync metadata
	if err := s.db.SetSyncMetadata(ctx, "recipes_last_sync", time.Now().Format(time.RFC3339)); err != nil {
		return err
	}
	if err := s.db.SetSyncMetadata(ctx, "recipes_count", fmt.Sprintf("%d", len(recipes))); err != nil {
		return err
	}

	return nil
}

// ImportSkillsFromFile imports skills from a JSON file.
func (s *Syncer) ImportSkillsFromFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	itemsData, err := unwrapItems(data)
	if err != nil {
		return fmt.Errorf("unwrapping items: %w", err)
	}

	var imports []SkillImport
	if err := json.Unmarshal(itemsData, &imports); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}

	skills := make([]crafting.Skill, 0, len(imports))
	for _, imp := range imports {
		skill := transformSkill(imp)
		skills = append(skills, skill)
	}

	skillStore := db.NewSkillStore(s.db)
	if err := skillStore.BulkInsertSkills(ctx, skills); err != nil {
		return fmt.Errorf("inserting skills: %w", err)
	}

	// Update sync metadata
	if err := s.db.SetSyncMetadata(ctx, "skills_last_sync", time.Now().Format(time.RFC3339)); err != nil {
		return err
	}
	if err := s.db.SetSyncMetadata(ctx, "skills_count", fmt.Sprintf("%d", len(skills))); err != nil {
		return err
	}

	return nil
}

// transformRecipe converts import format to domain format.
func transformRecipe(imp RecipeImport) crafting.Recipe {
	recipe := crafting.Recipe{
		ID:              imp.ID,
		Name:            imp.Name,
		Description:     imp.Description,
		Category:        imp.Category,
		CraftingTime:    imp.CraftingTime,
		BaseQuality:     imp.BaseQuality,
		SkillQualityMod: imp.SkillQualityMod,
	}

	// Handle inputs - try both "inputs" and "components" fields
	inputSources := imp.Inputs
	if len(inputSources) == 0 {
		inputSources = imp.Components // fallback to legacy field
	}

	for _, inp := range inputSources {
		itemID := inp.ID
		if itemID == "" {
			itemID = inp.ItemID
		}
		if itemID == "" {
			continue
		}
		recipe.Inputs = append(recipe.Inputs, crafting.RecipeInput{
			ItemID:   itemID,
			Quantity: inp.Quantity,
		})
	}

	// Handle outputs - try multiple outputs first
	if len(imp.Outputs) > 0 {
		for _, out := range imp.Outputs {
			itemID := out.ID
			if itemID == "" {
				itemID = out.ItemID
			}
			if itemID == "" {
				continue
			}
			recipe.Outputs = append(recipe.Outputs, crafting.RecipeOutput{
				ItemID:     itemID,
				Quantity:   out.Quantity,
				QualityMod: out.QualityMod,
			})
		}
	} else {
		// Fallback to legacy single output format
		var outputItemID string
		var outputQuantity int

		if imp.Output.ItemID != "" {
			outputItemID = imp.Output.ItemID
			outputQuantity = imp.Output.Quantity
		} else if imp.Output.ID != "" {
			outputItemID = imp.Output.ID
			outputQuantity = imp.Output.Quantity
		} else if imp.OutputItemID != "" {
			outputItemID = imp.OutputItemID
			outputQuantity = imp.OutputQuantity
		}

		if outputQuantity == 0 {
			outputQuantity = 1
		}

		if outputItemID != "" {
			recipe.Outputs = append(recipe.Outputs, crafting.RecipeOutput{
				ItemID:     outputItemID,
				Quantity:   outputQuantity,
				QualityMod: false,
			})
		}
	}

	// Transform skill requirements from Skills array
	for _, sk := range imp.Skills {
		skillID := sk.SkillID
		if skillID == "" {
			skillID = sk.ID
		}
		if skillID == "" {
			continue
		}
		level := sk.LevelRequired
		if level == 0 {
			level = sk.Level
		}
		recipe.SkillsRequired = append(recipe.SkillsRequired, crafting.SkillRequirement{
			SkillID:       skillID,
			LevelRequired: level,
		})
	}

	// If no Skills array entries, convert RequiredSkills map to SkillsRequired slice
	if len(recipe.SkillsRequired) == 0 && len(imp.RequiredSkills) > 0 {
		recipe.RequiredSkills = imp.RequiredSkills
		for skillID, level := range imp.RequiredSkills {
			recipe.SkillsRequired = append(recipe.SkillsRequired, crafting.SkillRequirement{
				SkillID:       skillID,
				LevelRequired: level,
			})
		}
	}

	return recipe
}

// transformSkill converts import format to domain format.
func transformSkill(imp SkillImport) crafting.Skill {
	skill := crafting.Skill{
		ID:             imp.ID,
		Name:           imp.Name,
		Description:    imp.Description,
		Category:       imp.Category,
		MaxLevel:       imp.MaxLevel,
		TrainingSource: imp.TrainingSource,
		XPPerLevel:     imp.XPPerLevel,
		BonusPerLevel:  imp.BonusPerLevel,
		RequiredSkills: imp.RequiredSkills,
	}
	if skill.MaxLevel == 0 {
		skill.MaxLevel = 10
	}

	// Transform prerequisites from array
	for _, p := range imp.Prerequisites {
		skillID := p.SkillID
		if skillID == "" {
			skillID = p.ID
		}
		if skillID == "" {
			continue
		}
		skill.Prerequisites = append(skill.Prerequisites, crafting.SkillRequirement{
			SkillID:       skillID,
			LevelRequired: p.Level,
		})
	}

	// If no Prerequisites array, parse RequiredSkills JSON as map[string]int
	if len(skill.Prerequisites) == 0 && len(imp.RequiredSkills) > 0 {
		var reqMap map[string]int
		if json.Unmarshal(imp.RequiredSkills, &reqMap) == nil {
			for skillID, level := range reqMap {
				skill.Prerequisites = append(skill.Prerequisites, crafting.SkillRequirement{
					SkillID:       skillID,
					LevelRequired: level,
				})
			}
		}
	}

	// Transform XP thresholds
	if len(imp.XPThresholds) > 0 {
		skill.XPThresholds = imp.XPThresholds
	} else if len(imp.Levels) > 0 {
		for _, lvl := range imp.Levels {
			xp := lvl.XPRequired
			if xp == 0 {
				xp = lvl.XP
			}
			skill.XPThresholds = append(skill.XPThresholds, xp)
		}
	}

	// If still no XP thresholds, parse XPPerLevel JSON as []int
	if len(skill.XPThresholds) == 0 && len(imp.XPPerLevel) > 0 {
		var xpList []int
		if json.Unmarshal(imp.XPPerLevel, &xpList) == nil && len(xpList) > 0 {
			skill.XPThresholds = xpList
		}
	}

	return skill
}

// ImportMarketDataFromFile imports market data from a JSON file.
func (s *Syncer) ImportMarketDataFromFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	var imports []struct {
		ComponentID string    `json:"component_id"` // legacy support
		ItemID      string    `json:"item_id"`      // new field
		StationID   string    `json:"station_id"`
		BuyPrice    int       `json:"buy_price"`
		SellPrice   int       `json:"sell_price"`
		Volume24h   int       `json:"volume_24h,omitempty"`
		Timestamp   time.Time `json:"timestamp,omitempty"`
	}

	if err := json.Unmarshal(data, &imports); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}

	marketStore := db.NewMarketStore(s.db)

	points := make([]db.MarketDataPoint, 0, len(imports))
	for _, imp := range imports {
		ts := imp.Timestamp
		if ts.IsZero() {
			ts = time.Now()
		}

		itemID := imp.ItemID
		if itemID == "" {
			itemID = imp.ComponentID // fallback to legacy field
		}

		points = append(points, db.MarketDataPoint{
			ItemID:    itemID,
			StationID: imp.StationID,
			BuyPrice:  imp.BuyPrice,
			SellPrice: imp.SellPrice,
			Volume24h: imp.Volume24h,
			Timestamp: ts,
		})
	}

	if err := marketStore.ImportMarketData(ctx, points); err != nil {
		return fmt.Errorf("importing market data: %w", err)
	}

	// Refresh summaries
	if err := marketStore.RefreshPriceSummaries(ctx); err != nil {
		return fmt.Errorf("refreshing summaries: %w", err)
	}

	// Update metadata
	if err := s.db.SetSyncMetadata(ctx, "market_last_sync", time.Now().Format(time.RFC3339)); err != nil {
		return err
	}

	return nil
}

// ClearAll removes all data from the database.
func (s *Syncer) ClearAll(ctx context.Context) error {
	itemStore := db.NewItemStore(s.db)
	recipeStore := db.NewRecipeStore(s.db)
	skillStore := db.NewSkillStore(s.db)
	marketStore := db.NewMarketStore(s.db)

	if err := itemStore.ClearItems(ctx); err != nil {
		return err
	}
	if err := recipeStore.ClearRecipes(ctx); err != nil {
		return err
	}
	if err := skillStore.ClearSkills(ctx); err != nil {
		return err
	}
	if err := marketStore.ClearMarketData(ctx); err != nil {
		return err
	}

	return nil
}
