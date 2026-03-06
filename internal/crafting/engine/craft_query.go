package engine

import (
	"context"
	"fmt"
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

	// Resolve station identifier
	req.StationID = e.resolveStationID(ctx, req.StationID)

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

		// Filter out ammunition recipes unless explicitly included
		if !req.IncludeAmmunition && recipe.Category == "Ammunition" {
			continue
		}

		// Calculate input match
		have, missing, canCraft := e.calculateInputMatch(recipe, inventory)
		matchRatio := calculateMatchRatio(len(have), len(recipe.Inputs))

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
			result := crafting.CraftableMatch{
				Recipe:           *recipe,
				CanCraftQuantity: canCraft,
				ProfitAnalysis:   profitAnalysis,
			}

			// Enrich with illegal status
			if err := e.enrichRecipeWithIllegalStatus(ctx, &result.Recipe); err != nil {
				return nil, fmt.Errorf("enriching illegal status: %w", err)
			}

			craftable = append(craftable, result)
		} else if matchRatio == 1.0 && !skillsReady {
			// Have inputs but blocked by skills
			result := crafting.PartialComponentMatch{
				Recipe:         *recipe,
				InputsHave:     have,
				InputsMissing:  missing,
				MatchRatio:     matchRatio,
				SkillsReady:    false,
				SkillsMissing:  skillGaps,
				ProfitAnalysis: profitAnalysis,
			}

			// Enrich with illegal status
			if err := e.enrichRecipeWithIllegalStatus(ctx, &result.Recipe); err != nil {
				return nil, fmt.Errorf("enriching illegal status: %w", err)
			}

			blockedBySkills = append(blockedBySkills, result)
		} else if req.IncludePartial && matchRatio >= req.MinMatchRatio {
			// Partial input match
			result := crafting.PartialComponentMatch{
				Recipe:         *recipe,
				InputsHave:     have,
				InputsMissing:  missing,
				MatchRatio:     matchRatio,
				SkillsReady:    skillsReady,
				SkillsMissing:  skillGaps,
				ProfitAnalysis: profitAnalysis,
			}

			// Enrich with illegal status
			if err := e.enrichRecipeWithIllegalStatus(ctx, &result.Recipe); err != nil {
				return nil, fmt.Errorf("enriching illegal status: %w", err)
			}

			partialComponents = append(partialComponents, result)
		}
	}

	// Sort results based on strategy
	e.sortCraftable(craftable, req.Strategy)
	e.sortPartial(partialComponents, req.Strategy)
	e.sortPartial(blockedBySkills, req.Strategy)

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
// Primary sort: Category tier (1-6), Secondary sort: Strategy.
func (e *Engine) sortCraftable(matches []crafting.CraftableMatch, strategy crafting.OptimizationStrategy) {
	sort.Slice(matches, func(i, j int) bool {
		// Primary: sort by category tier
		tierI := e.getCategoryTier(matches[i].Recipe.Category)
		tierJ := e.getCategoryTier(matches[j].Recipe.Category)
		if tierI != tierJ {
			return tierI < tierJ
		}

		// Secondary: apply strategy within same tier
		switch strategy {
		case crafting.StrategyMaximizeProfit:
			pi := profitPerUnit(matches[i].ProfitAnalysis)
			pj := profitPerUnit(matches[j].ProfitAnalysis)
			return pi > pj

		case crafting.StrategyMaximizeVolume:
			return matches[i].CanCraftQuantity > matches[j].CanCraftQuantity

		case crafting.StrategyUseInventoryFirst:
			return matches[i].CanCraftQuantity > matches[j].CanCraftQuantity

		case crafting.StrategyMinimizeAcquisition:
			return matches[i].CanCraftQuantity > matches[j].CanCraftQuantity

		case crafting.StrategyOptimizeCraftPath:
			return len(matches[i].Recipe.Inputs) < len(matches[j].Recipe.Inputs)

		default:
			return matches[i].CanCraftQuantity > matches[j].CanCraftQuantity
		}
	})
}

// sortPartial sorts partial matches based on optimization strategy.
// Primary sort: Category tier (1-6), Secondary sort: Strategy.
func (e *Engine) sortPartial(matches []crafting.PartialComponentMatch, strategy crafting.OptimizationStrategy) {
	sort.Slice(matches, func(i, j int) bool {
		// Primary: sort by category tier
		tierI := e.getCategoryTier(matches[i].Recipe.Category)
		tierJ := e.getCategoryTier(matches[j].Recipe.Category)
		if tierI != tierJ {
			return tierI < tierJ
		}

		// Secondary: apply strategy within same tier
		switch strategy {
		case crafting.StrategyMaximizeProfit:
			pi := profitPerUnit(matches[i].ProfitAnalysis)
			pj := profitPerUnit(matches[j].ProfitAnalysis)
			return pi > pj

		case crafting.StrategyMaximizeVolume:
			return matches[i].MatchRatio > matches[j].MatchRatio

		case crafting.StrategyUseInventoryFirst:
			return matches[i].MatchRatio > matches[j].MatchRatio

		case crafting.StrategyMinimizeAcquisition:
			return len(matches[i].InputsMissing) < len(matches[j].InputsMissing)

		case crafting.StrategyOptimizeCraftPath:
			return len(matches[i].Recipe.Inputs) < len(matches[j].Recipe.Inputs)

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
