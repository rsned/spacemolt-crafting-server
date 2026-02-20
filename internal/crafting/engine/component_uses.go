package engine

import (
	"context"
	"sort"

	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// ComponentUses executes the component_uses tool logic.
func (e *Engine) ComponentUses(ctx context.Context, req crafting.ComponentUsesRequest) (*crafting.ComponentUsesResponse, error) {
	// Apply defaults
	if !req.Strategy.IsValid() {
		req.Strategy = crafting.StrategyUseInventoryFirst
	}
	
	resp := &crafting.ComponentUsesResponse{
		ComponentID: req.ComponentID,
	}
	
	// Find all recipes that use this component
	recipeIDs, err := e.recipes.FindRecipesByComponents(ctx, []string{req.ComponentID})
	if err != nil {
		return nil, err
	}
	
	var uses []crafting.ComponentUseInfo
	
	for _, recipeID := range recipeIDs {
		recipe, err := e.recipes.GetRecipe(ctx, recipeID)
		if err != nil {
			return nil, err
		}
		if recipe == nil {
			continue
		}
		
		// Find how much of this component is needed
		var quantityNeeded int
		for _, comp := range recipe.Components {
			if comp.ComponentID == req.ComponentID {
				quantityNeeded = comp.Quantity
				break
			}
		}
		
		// Check skill requirements
		var skillReady bool
		var skillGaps []crafting.SkillGap
		if len(req.Skills) > 0 {
			var err error
			skillReady, skillGaps, err = e.checkSkillRequirements(ctx, recipe, req.Skills)
			if err != nil {
				return nil, err
			}
			
			// Skip if not including skill-locked recipes
			if !req.IncludeSkillLocked && !skillReady {
				continue
			}
		} else {
			skillReady = true // Assume ready if no skills provided
		}
		
		// Calculate profit if station provided
		var profitAnalysis *crafting.ProfitAnalysis
		if req.StationID != "" {
			profitAnalysis, err = e.calculateProfitAnalysis(ctx, recipe, req.StationID, 1)
			if err != nil {
				return nil, err
			}
		}
		
		uses = append(uses, crafting.ComponentUseInfo{
			Recipe:           *recipe,
			QuantityPerCraft: quantityNeeded,
			SkillReady:       skillReady,
			SkillGaps:        skillGaps,
			ProfitAnalysis:   profitAnalysis,
		})
	}
	
	// Sort based on strategy
	sortComponentUses(uses, req.Strategy)
	
	resp.UsedIn = uses
	resp.TotalUses = len(uses)
	
	// Get market sell price as alternative
	if req.StationID != "" {
		sellPrice, err := e.market.GetSellPrice(ctx, req.ComponentID, req.StationID)
		if err != nil {
			return nil, err
		}
		resp.MarketSellPrice = sellPrice
	}
	
	return resp, nil
}

// sortComponentUses sorts component uses based on optimization strategy.
func sortComponentUses(uses []crafting.ComponentUseInfo, strategy crafting.OptimizationStrategy) {
	sort.Slice(uses, func(i, j int) bool {
		switch strategy {
		case crafting.StrategyMaximizeProfit:
			pi := 0
			pj := 0
			if uses[i].ProfitAnalysis != nil {
				pi = uses[i].ProfitAnalysis.ProfitPerUnit
			}
			if uses[j].ProfitAnalysis != nil {
				pj = uses[j].ProfitAnalysis.ProfitPerUnit
			}
			return pi > pj
			
		case crafting.StrategyMaximizeVolume:
			// Prefer recipes that use less of the component (more recipes possible)
			return uses[i].QuantityPerCraft < uses[j].QuantityPerCraft
			
		case crafting.StrategyUseInventoryFirst:
			// Prefer simpler recipes
			return len(uses[i].Recipe.Components) < len(uses[j].Recipe.Components)
			
		default:
			// Default: prefer recipes we can actually craft
			if uses[i].SkillReady != uses[j].SkillReady {
				return uses[i].SkillReady
			}
			return len(uses[i].Recipe.Components) < len(uses[j].Recipe.Components)
		}
	})
}
