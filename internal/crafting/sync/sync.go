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

// RecipeImport represents the expected format of recipe data from SpaceMolt.
// Adjust this based on actual SpaceMolt get_recipes() output format.
type RecipeImport struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Category     string `json:"category,omitempty"`
	CraftTimeSec int    `json:"craft_time_sec,omitempty"`
	
	// Components may be in various formats
	Components []struct {
		ID       string `json:"id,omitempty"`
		ItemID   string `json:"item_id,omitempty"`
		Quantity int    `json:"quantity"`
	} `json:"components,omitempty"`
	
	// Skills may be in various formats
	Skills []struct {
		ID            string `json:"id,omitempty"`
		SkillID       string `json:"skill_id,omitempty"`
		Level         int    `json:"level,omitempty"`
		LevelRequired int    `json:"level_required,omitempty"`
	} `json:"skills,omitempty"`
	
	// Output
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
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	MaxLevel    int    `json:"max_level,omitempty"`
	
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

// ImportRecipesFromFile imports recipes from a JSON file.
func (s *Syncer) ImportRecipesFromFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}
	
	var imports []RecipeImport
	if err := json.Unmarshal(data, &imports); err != nil {
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
	
	var imports []SkillImport
	if err := json.Unmarshal(data, &imports); err != nil {
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
		ID:           imp.ID,
		Name:         imp.Name,
		Description:  imp.Description,
		Category:     imp.Category,
		CraftTimeSec: imp.CraftTimeSec,
	}
	
	// Handle output - try multiple field names
	if imp.Output.ItemID != "" {
		recipe.Output.ItemID = imp.Output.ItemID
		recipe.Output.Quantity = imp.Output.Quantity
	} else if imp.Output.ID != "" {
		recipe.Output.ItemID = imp.Output.ID
		recipe.Output.Quantity = imp.Output.Quantity
	} else if imp.OutputItemID != "" {
		recipe.Output.ItemID = imp.OutputItemID
		recipe.Output.Quantity = imp.OutputQuantity
	}
	if recipe.Output.Quantity == 0 {
		recipe.Output.Quantity = 1
	}
	
	// Transform components
	for _, c := range imp.Components {
		compID := c.ID
		if compID == "" {
			compID = c.ItemID
		}
		if compID == "" {
			continue
		}
		recipe.Components = append(recipe.Components, crafting.RecipeComponent{
			ComponentID: compID,
			Quantity:    c.Quantity,
		})
	}
	
	// Transform skill requirements
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
	
	return recipe
}

// transformSkill converts import format to domain format.
func transformSkill(imp SkillImport) crafting.Skill {
	skill := crafting.Skill{
		ID:          imp.ID,
		Name:        imp.Name,
		Category:    imp.Category,
		Description: imp.Description,
		MaxLevel:    imp.MaxLevel,
	}
	if skill.MaxLevel == 0 {
		skill.MaxLevel = 10
	}
	
	// Transform prerequisites
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
	
	return skill
}

// ImportMarketDataFromFile imports market data from a JSON file.
func (s *Syncer) ImportMarketDataFromFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}
	
	var imports []struct {
		ComponentID string    `json:"component_id"`
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
		points = append(points, db.MarketDataPoint{
			ComponentID: imp.ComponentID,
			StationID:   imp.StationID,
			BuyPrice:    imp.BuyPrice,
			SellPrice:   imp.SellPrice,
			Volume24h:   imp.Volume24h,
			Timestamp:   ts,
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
	recipeStore := db.NewRecipeStore(s.db)
	skillStore := db.NewSkillStore(s.db)
	marketStore := db.NewMarketStore(s.db)
	
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
