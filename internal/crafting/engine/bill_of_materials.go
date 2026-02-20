package engine

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// BillOfMaterials executes the bill_of_materials tool logic.
// It performs recursive dependency resolution, accounting for output quantities
// and returning a complete breakdown of raw materials, intermediates, and craft steps.
func (e *Engine) BillOfMaterials(ctx context.Context, req crafting.BillOfMaterialsRequest) (*crafting.BillOfMaterialsResponse, error) {
	// Apply defaults
	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	// Get the target recipe
	targetRecipe, err := e.recipes.GetRecipe(ctx, req.RecipeID)
	if err != nil {
		return nil, fmt.Errorf("getting target recipe: %w", err)
	}
	if targetRecipe == nil {
		return nil, fmt.Errorf("recipe not found: %s", req.RecipeID)
	}

	// Load all recipes to build reverse index
	allRecipes, err := e.recipes.GetAllRecipes(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading all recipes: %w", err)
	}

	// Build output -> recipe map with deterministic selection
	// When multiple recipes produce the same output, prefer:
	// 1. Shortest craft time
	// 2. Highest output quantity (better efficiency)
	// 3. Lexicographically first recipe_id (for determinism)
	//
	// IMPORTANT: This map is used consistently throughout the entire dependency tree,
	// so diamond dependencies (multiple paths to the same item) will always use the
	// same recipe. This ensures consistency - we don't use recipe A on one branch
	// and recipe B on another branch for the same intermediate item.
	outputToRecipe := make(map[string]*crafting.Recipe)
	for i := range allRecipes {
		existing, exists := outputToRecipe[allRecipes[i].Output.ItemID]
		if !exists {
			outputToRecipe[allRecipes[i].Output.ItemID] = &allRecipes[i]
			continue
		}

		// Compare and pick better recipe
		newRecipe := &allRecipes[i]
		replace := false

		// Prefer shorter craft time
		if newRecipe.CraftTimeSec < existing.CraftTimeSec {
			replace = true
		} else if newRecipe.CraftTimeSec == existing.CraftTimeSec {
			// If same time, prefer higher output quantity (more efficient)
			if newRecipe.Output.Quantity > existing.Output.Quantity {
				replace = true
			} else if newRecipe.Output.Quantity == existing.Output.Quantity {
				// If still tied, use recipe_id for determinism
				if newRecipe.ID < existing.ID {
					replace = true
				}
			}
		}

		if replace {
			outputToRecipe[allRecipes[i].Output.ItemID] = newRecipe
		}
	}

	// Discover craftable items via DFS starting from the target recipe
	// Note: Diamond dependencies (multiple paths to same item) are allowed
	craftableItems := make(map[string]*crafting.Recipe)
	visited := make(map[string]bool)
	pathStack := make(map[string]bool)

	var dfs func(itemID string) error
	dfs = func(itemID string) error {
		if visited[itemID] {
			return nil
		}

		if pathStack[itemID] {
			return fmt.Errorf("cycle detected: item %s has circular dependency", itemID)
		}

		visited[itemID] = true
		pathStack[itemID] = true

		recipe, exists := outputToRecipe[itemID]
		if !exists {
			// Not craftable (raw material)
			delete(pathStack, itemID)
			return nil
		}

		craftableItems[itemID] = recipe

		// Recursively visit dependencies
		for _, comp := range recipe.Components {
			if err := dfs(comp.ComponentID); err != nil {
				return err
			}
		}

		delete(pathStack, itemID)
		return nil
	}

	// Start DFS with the target recipe explicitly
	craftableItems[targetRecipe.Output.ItemID] = targetRecipe
	for _, comp := range targetRecipe.Components {
		if err := dfs(comp.ComponentID); err != nil {
			return nil, err
		}
	}

	// Topological sort (deepest dependencies first)
	sortedBottomUp, err := topologicalSort(craftableItems)
	if err != nil {
		return nil, fmt.Errorf("topological sort: %w", err)
	}

	// Calculate demand (top-down: process target first, then dependencies)
	// Create reversed order for demand propagation
	sortedTopDown := make([]string, len(sortedBottomUp))
	copy(sortedTopDown, sortedBottomUp)
	for i, j := 0, len(sortedTopDown)-1; i < j; i, j = i+1, j-1 {
		sortedTopDown[i], sortedTopDown[j] = sortedTopDown[j], sortedTopDown[i]
	}

	demand := make(map[string]int)
	demand[targetRecipe.Output.ItemID] = req.Quantity

	craftRuns := make(map[string]int)
	for _, itemID := range sortedTopDown {
		recipe := craftableItems[itemID]
		itemDemand := demand[itemID]
		if itemDemand == 0 {
			continue
		}

		// Calculate craft runs needed
		runsNeeded := int(math.Ceil(float64(itemDemand) / float64(recipe.Output.Quantity)))
		craftRuns[itemID] = runsNeeded

		// Propagate demand to components
		for _, comp := range recipe.Components {
			demand[comp.ComponentID] += runsNeeded * comp.Quantity
		}
	}

	// Separate raw materials (items with demand but no recipe)
	var rawMaterials []crafting.BOMItem
	for itemID, qty := range demand {
		if craftableItems[itemID] == nil && qty > 0 {
			rawMaterials = append(rawMaterials, crafting.BOMItem{
				ItemID:   itemID,
				Quantity: qty,
			})
		}
	}
	sort.Slice(rawMaterials, func(i, j int) bool {
		return rawMaterials[i].ItemID < rawMaterials[j].ItemID
	})

	// Build intermediates list
	var intermediates []crafting.BOMIntermediate
	for itemID, recipe := range craftableItems {
		runs := craftRuns[itemID]
		if runs == 0 {
			continue
		}
		// Exclude the target item from intermediates
		if itemID == targetRecipe.Output.ItemID {
			continue
		}

		intermediates = append(intermediates, crafting.BOMIntermediate{
			ItemID:        itemID,
			RecipeID:      recipe.ID,
			RecipeName:    recipe.Name,
			CraftRuns:     runs,
			TotalProduced: runs * recipe.Output.Quantity,
			TotalNeeded:   demand[itemID],
		})
	}
	sort.Slice(intermediates, func(i, j int) bool {
		return intermediates[i].ItemID < intermediates[j].ItemID
	})

	// Build craft steps (in bottom-up order: deepest dependencies first)
	var craftSteps []crafting.BOMCraftStep
	stepNum := 1
	for _, itemID := range sortedBottomUp {
		recipe := craftableItems[itemID]
		runs := craftRuns[itemID]
		if runs == 0 {
			continue
		}

		craftSteps = append(craftSteps, crafting.BOMCraftStep{
			StepNumber:   stepNum,
			RecipeID:     recipe.ID,
			RecipeName:   recipe.Name,
			CraftRuns:    runs,
			OutputItemID: recipe.Output.ItemID,
			OutputPerRun: recipe.Output.Quantity,
		})
		stepNum++
	}

	// Calculate total craft time
	totalTime := 0
	for itemID, runs := range craftRuns {
		recipe := craftableItems[itemID]
		totalTime += recipe.CraftTimeSec * runs
	}

	return &crafting.BillOfMaterialsResponse{
		RecipeID:       targetRecipe.ID,
		RecipeName:     targetRecipe.Name,
		OutputItemID:   targetRecipe.Output.ItemID,
		Quantity:       req.Quantity,
		RawMaterials:   rawMaterials,
		Intermediates:  intermediates,
		CraftSteps:     craftSteps,
		TotalCraftTime: totalTime,
	}, nil
}

// topologicalSort performs a topological sort on craftable items.
// Returns items in dependency order (deepest dependencies first).
func topologicalSort(craftable map[string]*crafting.Recipe) ([]string, error) {
	// Build in-degree map
	inDegree := make(map[string]int)
	adjacency := make(map[string][]string)

	for itemID, recipe := range craftable {
		if _, exists := inDegree[itemID]; !exists {
			inDegree[itemID] = 0
		}

		for _, comp := range recipe.Components {
			// Only consider craftable components as dependencies
			if craftable[comp.ComponentID] != nil {
				adjacency[comp.ComponentID] = append(adjacency[comp.ComponentID], itemID)
				inDegree[itemID]++
			}
		}
	}

	// Find nodes with no incoming edges
	var queue []string
	for itemID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, itemID)
		}
	}

	var sorted []string
	for len(queue) > 0 {
		// Dequeue
		current := queue[0]
		queue = queue[1:]
		sorted = append(sorted, current)

		// Reduce in-degree for dependents
		for _, dependent := range adjacency[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Check for cycles
	if len(sorted) != len(craftable) {
		return nil, fmt.Errorf("cycle detected in recipe dependencies")
	}

	return sorted, nil
}
