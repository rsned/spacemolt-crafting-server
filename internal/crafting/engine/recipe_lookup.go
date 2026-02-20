package engine

import (
	"context"

	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// RecipeLookup executes the recipe_lookup tool logic.
func (e *Engine) RecipeLookup(ctx context.Context, req crafting.RecipeLookupRequest) (*crafting.RecipeLookupResponse, error) {
	resp := &crafting.RecipeLookupResponse{}
	
	// If search term provided, search first
	if req.Search != "" {
		hits, err := e.recipes.SearchRecipes(ctx, req.Search, 10)
		if err != nil {
			return nil, err
		}
		resp.SearchResults = hits
		
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
	
	// Find recipes that use this recipe's output
	usedIn, err := e.recipes.GetRecipesUsingOutput(ctx, recipe.Output.ItemID)
	if err != nil {
		return nil, err
	}
	resp.UsedInRecipes = usedIn
	
	return resp, nil
}
