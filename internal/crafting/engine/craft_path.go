package engine

import (
	"context"

	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// CraftPathTo executes the craft_path_to tool logic.
// It performs single-level expansion - showing direct components needed.
func (e *Engine) CraftPathTo(ctx context.Context, req crafting.CraftPathRequest) (*crafting.CraftPathResponse, error) {
	// Apply defaults
	if req.TargetQuantity <= 0 {
		req.TargetQuantity = 1
	}
	
	// Get the target recipe
	recipe, err := e.recipes.GetRecipe(ctx, req.TargetRecipeID)
	if err != nil {
		return nil, err
	}
	if recipe == nil {
		return &crafting.CraftPathResponse{
			Target: crafting.CraftPathTarget{
				RecipeID: req.TargetRecipeID,
				Quantity: req.TargetQuantity,
			},
			Feasible: false,
		}, nil
	}
	
	// Build inventory map
	inventory := buildInventoryMap(req.CurrentInventory)
	
	// Check skill requirements
	skillsReady, skillGaps, err := e.checkSkillRequirements(ctx, recipe, req.Skills)
	if err != nil {
		return nil, err
	}
	
	// Calculate materials needed (single level)
	materials, err := e.calculateMaterialsNeeded(ctx, recipe, req.TargetQuantity, inventory, req.StationID)
	if err != nil {
		return nil, err
	}
	
	// Calculate summary
	summary := calculatePathSummary(materials)
	
	// Determine feasibility (can acquire all materials)
	feasible := true
	for _, mat := range materials {
		if mat.QuantityToAcquire > 0 && len(mat.AcquisitionMethods) == 0 && !mat.IsCraftable {
			feasible = false
			break
		}
	}
	
	return &crafting.CraftPathResponse{
		Target: crafting.CraftPathTarget{
			RecipeID:   recipe.ID,
			RecipeName: recipe.Name,
			Quantity:   req.TargetQuantity,
		},
		Feasible:        feasible,
		SkillReady:      skillsReady,
		SkillsMissing:   skillGaps,
		MaterialsNeeded: materials,
		CraftTimeSec:    recipe.CraftTimeSec * req.TargetQuantity,
		Summary:         summary,
	}, nil
}

// calculateMaterialsNeeded calculates what materials are needed for a recipe.
func (e *Engine) calculateMaterialsNeeded(
	ctx context.Context,
	recipe *crafting.Recipe,
	quantity int,
	inventory map[string]int,
	stationID string,
) ([]crafting.MaterialRequirement, error) {
	var materials []crafting.MaterialRequirement
	
	for _, comp := range recipe.Components {
		needed := comp.Quantity * quantity
		have := inventory[comp.ComponentID]
		toAcquire := needed - have
		if toAcquire < 0 {
			toAcquire = 0
		}
		
		mat := crafting.MaterialRequirement{
			ComponentID:       comp.ComponentID,
			QuantityNeeded:    needed,
			QuantityHave:      have,
			QuantityToAcquire: toAcquire,
		}
		
		// Check if this component can be crafted
		craftRecipes, err := e.recipes.FindRecipesByOutput(ctx, comp.ComponentID)
		if err != nil {
			return nil, err
		}
		if len(craftRecipes) > 0 {
			mat.IsCraftable = true
			mat.CraftRecipeID = craftRecipes[0] // Use first recipe
		}
		
		// Add acquisition methods
		if toAcquire > 0 {
			// TODO: Look up where this component can be acquired
			// For now, indicate it can be bought if we have market data
			if stationID != "" {
				price, err := e.market.GetBuyPrice(ctx, comp.ComponentID, stationID)
				if err != nil {
					return nil, err
				}
				if price > 0 {
					mat.AcquisitionMethods = append(mat.AcquisitionMethods, "buy:"+stationID)
				}
			}
			
			// If craftable, that's also an acquisition method
			if mat.IsCraftable {
				mat.AcquisitionMethods = append(mat.AcquisitionMethods, "craft:"+mat.CraftRecipeID)
			}
		}
		
		materials = append(materials, mat)
	}
	
	return materials, nil
}

// calculatePathSummary aggregates material requirements into a summary.
func calculatePathSummary(materials []crafting.MaterialRequirement) crafting.CraftPathSummary {
	summary := crafting.CraftPathSummary{
		TotalComponents: len(materials),
	}
	
	for _, mat := range materials {
		if mat.QuantityHave >= mat.QuantityNeeded {
			summary.ComponentsHave++
		}
		if mat.QuantityToAcquire > 0 {
			summary.ComponentsToAcquire++
			if mat.IsCraftable {
				summary.ComponentsCraftable++
			}
		}
	}
	
	return summary
}
