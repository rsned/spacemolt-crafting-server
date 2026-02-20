// Package engine contains the crafting query business logic.
package engine

import (
	"context"

	"github.com/rsned/spacemolt-crafting-server/internal/crafting/db"
	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// Engine is the main query engine for crafting operations.
type Engine struct {
	recipes *db.RecipeStore
	skills  *db.SkillStore
	market  *db.MarketStore
}

// New creates a new Engine with the given database stores.
func New(database *db.DB) *Engine {
	return &Engine{
		recipes: db.NewRecipeStore(database),
		skills:  db.NewSkillStore(database),
		market:  db.NewMarketStore(database),
	}
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

// calculateComponentMatch calculates how well the agent's inventory matches recipe requirements.
func (e *Engine) calculateComponentMatch(
	recipe *crafting.Recipe,
	inventory map[string]int,
) (have []crafting.RecipeComponent, missing []crafting.RecipeComponent, canCraft int) {
	if len(recipe.Components) == 0 {
		return nil, nil, 0
	}
	
	canCraft = -1 // will be set to minimum craftable quantity
	
	for _, req := range recipe.Components {
		available := inventory[req.ComponentID]
		
		if available >= req.Quantity {
			// Have enough for at least one craft
			have = append(have, crafting.RecipeComponent{
				ComponentID: req.ComponentID,
				Quantity:    req.Quantity,
			})
			
			// How many times can we craft with this component?
			thisCanCraft := available / req.Quantity
			if canCraft < 0 || thisCanCraft < canCraft {
				canCraft = thisCanCraft
			}
		} else if available > 0 {
			// Have some but not enough
			have = append(have, crafting.RecipeComponent{
				ComponentID: req.ComponentID,
				Quantity:    available,
			})
			missing = append(missing, crafting.RecipeComponent{
				ComponentID: req.ComponentID,
				Quantity:    req.Quantity - available,
			})
			canCraft = 0
		} else {
			// Have none
			missing = append(missing, crafting.RecipeComponent{
				ComponentID: req.ComponentID,
				Quantity:    req.Quantity,
			})
			canCraft = 0
		}
	}
	
	if canCraft < 0 {
		canCraft = 0
	}
	
	return have, missing, canCraft
}

// calculateMatchRatio returns the ratio of matched components to total components.
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
	
	// Get output sell price
	outputPrice, err := e.market.GetSellPrice(ctx, recipe.Output.ItemID, stationID)
	if err != nil {
		return nil, err
	}
	if outputPrice == 0 {
		return nil, nil // No market data
	}
	
	// Calculate input cost
	var inputCost int
	for _, comp := range recipe.Components {
		buyPrice, err := e.market.GetBuyPrice(ctx, comp.ComponentID, stationID)
		if err != nil {
			return nil, err
		}
		inputCost += buyPrice * comp.Quantity
	}
	
	profitPerUnit := (outputPrice * recipe.Output.Quantity) - inputCost
	
	var marginPct float64
	if inputCost > 0 {
		marginPct = float64(profitPerUnit) / float64(inputCost) * 100
	}
	
	// Get volume and trend
	volume, err := e.market.GetVolume24h(ctx, recipe.Output.ItemID, stationID)
	if err != nil {
		return nil, err
	}
	
	trend, err := e.market.GetPriceTrend(ctx, recipe.Output.ItemID, stationID)
	if err != nil {
		return nil, err
	}
	
	analysis := &crafting.ProfitAnalysis{
		OutputSellPrice: outputPrice,
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
