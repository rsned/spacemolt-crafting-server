// Package engine contains the crafting query business logic.
package engine

import (
	"context"
	"log"

	"github.com/rsned/spacemolt-crafting-server/internal/crafting/db"
	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// Engine is the main query engine for crafting operations.
type Engine struct {
	recipes   *db.RecipeStore
	skills    *db.SkillStore
	market    *db.MarketStore
	catPri    *db.CategoryPriorityStore

	// Cached priority map for fast lookups
	categoryPriorities map[string]int
}

// New creates a new Engine with the given database stores.
func New(database *db.DB) *Engine {
	// Load category priorities into memory for fast access
	priorities, err := database.CategoryPriorities().GetAllCategories(context.Background())
	if err != nil {
		// Log warning but continue - will use tier 6 (default) for all
		log.Printf("WARNING: Failed to load category priorities: %v", err)
		priorities = make(map[string]int)
	}

	return &Engine{
		recipes:            db.NewRecipeStore(database),
		skills:             db.NewSkillStore(database),
		market:             db.NewMarketStore(database),
		catPri:             database.CategoryPriorities(),
		categoryPriorities: priorities,
	}
}

// getCategoryTier returns the priority tier for a category.
// Returns 6 (lowest) for unlisted categories.
func (e *Engine) getCategoryTier(category string) int {
	if tier, ok := e.categoryPriorities[category]; ok {
		return tier
	}
	return 6 // Default to lowest priority
}

// checkSkillRequirements checks if the given skills meet recipe requirements.
// Returns whether all requirements are met and any gaps.
func (e *Engine) checkSkillRequirements(
	ctx context.Context,
	recipe *crafting.Recipe,
	agentSkills map[string]int,
) (bool, []crafting.SkillGap, error) {
	ready := true
	var gaps []crafting.SkillGap

	for _, req := range recipe.SkillsRequired {
		currentLevel := agentSkills[req.SkillID] // defaults to 0 if not present

		if currentLevel < req.LevelRequired {
			ready = false

			// Get skill name for better output
			skillName, err := e.skills.GetSkillName(ctx, req.SkillID)
			if err != nil {
				return false, nil, err
			}
			if skillName == "" {
				skillName = req.SkillID
			}

			// Calculate XP to next level
			xpToNext, err := e.skills.GetXPForLevel(ctx, req.SkillID, currentLevel+1)
			if err != nil {
				return false, nil, err
			}

			gaps = append(gaps, crafting.SkillGap{
				SkillID:       req.SkillID,
				SkillName:     skillName,
				CurrentLevel:  currentLevel,
				RequiredLevel: req.LevelRequired,
				XPToNext:      xpToNext,
			})
		}
	}

	return ready, gaps, nil
}

// calculateInputMatch calculates how well the agent's inventory matches recipe input requirements.
func (e *Engine) calculateInputMatch(
	recipe *crafting.Recipe,
	inventory map[string]int,
) (have []crafting.RecipeInput, missing []crafting.RecipeInput, canCraft int) {
	if len(recipe.Inputs) == 0 {
		return nil, nil, 0
	}

	canCraft = -1 // will be set to minimum craftable quantity

	for _, req := range recipe.Inputs {
		available := inventory[req.ItemID]

		if available >= req.Quantity {
			// Have enough for at least one craft
			have = append(have, crafting.RecipeInput{
				ItemID:   req.ItemID,
				Quantity: req.Quantity,
			})

			// How many times can we craft with this input?
			thisCanCraft := available / req.Quantity
			if canCraft < 0 || thisCanCraft < canCraft {
				canCraft = thisCanCraft
			}
		} else if available > 0 {
			// Have some but not enough
			have = append(have, crafting.RecipeInput{
				ItemID:   req.ItemID,
				Quantity: available,
			})
			missing = append(missing, crafting.RecipeInput{
				ItemID:   req.ItemID,
				Quantity: req.Quantity - available,
			})
			canCraft = 0
		} else {
			// Have none
			missing = append(missing, crafting.RecipeInput{
				ItemID:   req.ItemID,
				Quantity: req.Quantity,
			})
			canCraft = 0
		}
	}

	if canCraft < 0 {
		canCraft = 0
	}

	return have, missing, canCraft
}

// calculateMatchRatio returns the ratio of matched inputs to total inputs.
func calculateMatchRatio(have, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(have) / float64(total)
}

// calculateProfitAnalysis calculates profit metrics for a recipe at a station.
func (e *Engine) calculateProfitAnalysis(
	ctx context.Context,
	recipe *crafting.Recipe,
	stationID string,
	canCraftQuantity int,
) (*crafting.ProfitAnalysis, error) {
	if stationID == "" {
		return nil, nil
	}

	// Calculate total output value from all outputs
	var totalOutputPrice int
	for _, output := range recipe.Outputs {
		outputPrice, err := e.market.GetSellPrice(ctx, output.ItemID, stationID)
		if err != nil {
			return nil, err
		}
		totalOutputPrice += outputPrice * output.Quantity
	}

	if totalOutputPrice == 0 {
		return nil, nil // No market data
	}

	// Calculate input cost
	var inputCost int
	for _, inp := range recipe.Inputs {
		buyPrice, err := e.market.GetBuyPrice(ctx, inp.ItemID, stationID)
		if err != nil {
			return nil, err
		}
		inputCost += buyPrice * inp.Quantity
	}

	profitPerUnit := totalOutputPrice - inputCost

	var marginPct float64
	if inputCost > 0 {
		marginPct = float64(profitPerUnit) / float64(inputCost) * 100
	}

	// For multi-output recipes, use the first output as primary for volume/trend
	var primaryOutput crafting.RecipeOutput
	if len(recipe.Outputs) > 0 {
		primaryOutput = recipe.Outputs[0]
	}

	volume, err := e.market.GetVolume24h(ctx, primaryOutput.ItemID, stationID)
	if err != nil {
		return nil, err
	}

	trend, err := e.market.GetPriceTrend(ctx, primaryOutput.ItemID, stationID)
	if err != nil {
		return nil, err
	}

	analysis := &crafting.ProfitAnalysis{
		OutputSellPrice: totalOutputPrice,
		InputCost:       inputCost,
		ProfitPerUnit:   profitPerUnit,
		ProfitMarginPct: marginPct,
		MarketVolume24h: volume,
		PriceTrend:      trend,
	}

	if canCraftQuantity > 0 {
		analysis.TotalPotentialProfit = profitPerUnit * canCraftQuantity
	}

	return analysis, nil
}

// buildInventoryMap converts a component slice to a map for efficient lookup.
func buildInventoryMap(components []crafting.Component) map[string]int {
	m := make(map[string]int, len(components))
	for _, c := range components {
		m[c.ID] = c.Quantity
	}
	return m
}
