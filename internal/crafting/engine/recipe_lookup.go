package engine

import (
	"context"
	"fmt"
	"sort"

	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// RecipeLookup executes the recipe_lookup tool logic.
func (e *Engine) RecipeLookup(ctx context.Context, req crafting.RecipeLookupRequest) (*crafting.RecipeLookupResponse, error) {
	// Resolve station identifier
	req.StationID = e.resolveStationID(ctx, req.StationID)

	resp := &crafting.RecipeLookupResponse{}

	// If search term provided, search first
	if req.Search != "" {
		hits, err := e.recipes.SearchRecipes(ctx, req.Search, 10)
		if err != nil {
			return nil, err
		}
		resp.SearchResults = hits
		
		// Sort search results by category tier
		sort.Slice(resp.SearchResults, func(i, j int) bool {
			tierI := e.getCategoryTier(resp.SearchResults[i].Category)
			tierJ := e.getCategoryTier(resp.SearchResults[j].Category)
			if tierI != tierJ {
				return tierI < tierJ
			}
			// Within tier, sort by name
			return resp.SearchResults[i].Name < resp.SearchResults[j].Name
		})
		
		// If exactly one result and no recipe_id provided, use it
		if len(hits) == 1 && req.RecipeID == "" {
			req.RecipeID = hits[0].RecipeID
		}
	}
	
	// If no recipe ID, return just search results
	if req.RecipeID == "" {
		return resp, nil
	}
	
	// Get the recipe
	recipe, err := e.recipes.GetRecipe(ctx, req.RecipeID)
	if err != nil {
		return nil, err
	}
	if recipe == nil {
		return resp, nil
	}
	resp.Recipe = recipe
	
	// Check skill requirements if skills provided
	if len(req.Skills) > 0 {
		ready, gaps, err := e.checkSkillRequirements(ctx, recipe, req.Skills)
		if err != nil {
			return nil, err
		}
		resp.SkillReady = ready
		resp.SkillGaps = gaps
	}
	
	// Calculate profit analysis if station provided
	if req.StationID != "" {
		analysis, err := e.calculateProfitAnalysis(ctx, recipe, req.StationID, 1)
		if err != nil {
			return nil, err
		}
		resp.ProfitAnalysis = analysis
	}
	
	// Find recipes that use this recipe's outputs as inputs
	usedInMap := make(map[string]bool)
	for _, output := range recipe.Outputs {
		recipes, err := e.recipes.GetRecipesUsingOutput(ctx, output.ItemID)
		if err != nil {
			return nil, err
		}
		for _, r := range recipes {
			usedInMap[r] = true
		}
	}

	// Convert map to slice
	usedIn := make([]string, 0, len(usedInMap))
	for recipeID := range usedInMap {
		usedIn = append(usedIn, recipeID)
	}
	resp.UsedInRecipes = usedIn

	// Enrich with illegal status before returning
	if err := e.enrichRecipeWithIllegalStatus(ctx, resp.Recipe); err != nil {
		return nil, fmt.Errorf("checking illegal status: %w", err)
	}

	return resp, nil
}
