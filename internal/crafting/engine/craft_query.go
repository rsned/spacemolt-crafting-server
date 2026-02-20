package engine

import (
	"context"
	"sort"
	"time"

	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// CraftQuery executes the craft_query tool logic.
func (e *Engine) CraftQuery(ctx context.Context, req crafting.CraftQueryRequest) (*crafting.CraftQueryResponse, error) {
	startTime := time.Now()
	
	// Apply defaults
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.MinMatchRatio <= 0 {
		req.MinMatchRatio = 0.25
	}
	if !req.Strategy.IsValid() {
		req.Strategy = crafting.StrategyUseInventoryFirst
	}
	
	// Build inventory lookup map
	inventory := buildInventoryMap(req.Components)
	componentIDs := make([]string, 0, len(req.Components))
	for _, c := range req.Components {
		componentIDs = append(componentIDs, c.ID)
	}
	
	// Find candidate recipes using inverted index
	candidateIDs, err := e.recipes.FindRecipesByComponents(ctx, componentIDs)
	if err != nil {
		return nil, err
	}
	
	// If category filter is set, also include all recipes from that category
	if req.CategoryFilter != "" {
		categoryIDs, err := e.recipes.ListRecipesByCategory(ctx, req.CategoryFilter)
		if err != nil {
			return nil, err
		}
		// Merge without duplicates
		seen := make(map[string]bool)
		for _, id := range candidateIDs {
			seen[id] = true
		}
		for _, id := range categoryIDs {
			if !seen[id] {
				candidateIDs = append(candidateIDs, id)
				seen[id] = true
			}
		}
	}
	
	var craftable []crafting.CraftableMatch
	var partialComponents []crafting.PartialComponentMatch
	var blockedBySkills []crafting.PartialComponentMatch
	
	for _, recipeID := range candidateIDs {
		recipe, err := e.recipes.GetRecipe(ctx, recipeID)
		if err != nil {
			return nil, err
		}
		if recipe == nil {
			continue
		}
		
		// Apply category filter
		if req.CategoryFilter != "" && recipe.Category != req.CategoryFilter {
			continue
		}
		
		// Calculate component match
		have, missing, canCraft := e.calculateComponentMatch(recipe, inventory)
		matchRatio := calculateMatchRatio(len(have), len(recipe.Components))
		
		// Check skill requirements
		skillsReady, skillGaps, err := e.checkSkillRequirements(ctx, recipe, req.Skills)
		if err != nil {
			return nil, err
		}
		
		// Calculate profit if station provided
		var profitAnalysis *crafting.ProfitAnalysis
		if req.StationID != "" {
			profitAnalysis, err = e.calculateProfitAnalysis(ctx, recipe, req.StationID, canCraft)
			if err != nil {
				return nil, err
			}
		}
		
		// Categorize result
		if matchRatio == 1.0 && skillsReady {
			// Fully craftable
			craftable = append(craftable, crafting.CraftableMatch{
				Recipe:           *recipe,
				CanCraftQuantity: canCraft,
				ProfitAnalysis:   profitAnalysis,
			})
		} else if matchRatio == 1.0 && !skillsReady {
			// Have components but blocked by skills
			blockedBySkills = append(blockedBySkills, crafting.PartialComponentMatch{
				Recipe:            *recipe,
				ComponentsHave:    have,
				ComponentsMissing: missing,
				MatchRatio:        matchRatio,
				SkillsReady:       false,
				SkillsMissing:     skillGaps,
				ProfitAnalysis:    profitAnalysis,
			})
		} else if req.IncludePartial && matchRatio >= req.MinMatchRatio {
			// Partial component match
			partialComponents = append(partialComponents, crafting.PartialComponentMatch{
				Recipe:            *recipe,
				ComponentsHave:    have,
				ComponentsMissing: missing,
				MatchRatio:        matchRatio,
				SkillsReady:       skillsReady,
				SkillsMissing:     skillGaps,
				ProfitAnalysis:    profitAnalysis,
			})
		}
	}
	
	// Sort results based on strategy
	sortCraftable(craftable, req.Strategy)
	sortPartial(partialComponents, req.Strategy)
	sortPartial(blockedBySkills, req.Strategy)
	
	// Apply limits
	if len(craftable) > req.Limit {
		craftable = craftable[:req.Limit]
	}
	if len(partialComponents) > req.Limit {
		partialComponents = partialComponents[:req.Limit]
	}
	if len(blockedBySkills) > req.Limit {
		blockedBySkills = blockedBySkills[:req.Limit]
	}
	
	return &crafting.CraftQueryResponse{
		Craftable:         craftable,
		PartialComponents: partialComponents,
		BlockedBySkills:   blockedBySkills,
		QueryStats: crafting.QueryStats{
			TotalRecipesChecked: len(candidateIDs),
			ComponentsProvided:  len(req.Components),
			StrategyUsed:        string(req.Strategy),
			ProcessingTimeMs:    time.Since(startTime).Milliseconds(),
		},
	}, nil
}

// sortCraftable sorts craftable matches based on optimization strategy.
func sortCraftable(matches []crafting.CraftableMatch, strategy crafting.OptimizationStrategy) {
	sort.Slice(matches, func(i, j int) bool {
		switch strategy {
		case crafting.StrategyMaximizeProfit:
			pi := profitPerUnit(matches[i].ProfitAnalysis)
			pj := profitPerUnit(matches[j].ProfitAnalysis)
			return pi > pj
			
		case crafting.StrategyMaximizeVolume:
			return matches[i].CanCraftQuantity > matches[j].CanCraftQuantity
			
		case crafting.StrategyUseInventoryFirst:
			// Already sorted by having all components; sort by can_craft as tiebreaker
			return matches[i].CanCraftQuantity > matches[j].CanCraftQuantity
			
		case crafting.StrategyMinimizeAcquisition:
			// All craftable items need 0 acquisition, sort by craft quantity
			return matches[i].CanCraftQuantity > matches[j].CanCraftQuantity
			
		case crafting.StrategyOptimizeCraftPath:
			// Prefer simpler recipes (fewer components)
			return len(matches[i].Recipe.Components) < len(matches[j].Recipe.Components)
			
		default:
			return matches[i].CanCraftQuantity > matches[j].CanCraftQuantity
		}
	})
}

// sortPartial sorts partial matches based on optimization strategy.
func sortPartial(matches []crafting.PartialComponentMatch, strategy crafting.OptimizationStrategy) {
	sort.Slice(matches, func(i, j int) bool {
		switch strategy {
		case crafting.StrategyMaximizeProfit:
			pi := profitPerUnit(matches[i].ProfitAnalysis)
			pj := profitPerUnit(matches[j].ProfitAnalysis)
			return pi > pj
			
		case crafting.StrategyMaximizeVolume:
			// Sort by match ratio (closer to craftable)
			return matches[i].MatchRatio > matches[j].MatchRatio
			
		case crafting.StrategyUseInventoryFirst:
			// Sort by match ratio (use more of what we have)
			return matches[i].MatchRatio > matches[j].MatchRatio
			
		case crafting.StrategyMinimizeAcquisition:
			// Sort by fewest missing components
			return len(matches[i].ComponentsMissing) < len(matches[j].ComponentsMissing)
			
		case crafting.StrategyOptimizeCraftPath:
			// Prefer simpler recipes
			return len(matches[i].Recipe.Components) < len(matches[j].Recipe.Components)
			
		default:
			return matches[i].MatchRatio > matches[j].MatchRatio
		}
	})
}

// profitPerUnit safely extracts profit from analysis.
func profitPerUnit(analysis *crafting.ProfitAnalysis) int {
	if analysis == nil {
		return 0
	}
	return analysis.ProfitPerUnit
}
