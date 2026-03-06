// Package engine contains the crafting query business logic.
package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/rsned/spacemolt-crafting-server/internal/crafting/db"
	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// Engine is the main query engine for crafting operations.
type Engine struct {
	db        *db.DB
	recipes   *db.RecipeStore
	skills    *db.SkillStore
	market    *db.MarketStore
	catPri    *db.CategoryPriorityStore
	illegalStore *db.IllegalRecipesStore

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
		db:                 database,
		recipes:            db.NewRecipeStore(database),
		skills:             db.NewSkillStore(database),
		market:             db.NewMarketStore(database),
		catPri:             database.CategoryPriorities(),
		illegalStore:       db.NewIllegalRecipesStore(database),
		categoryPriorities: priorities,
	}
}

// resolveStationID resolves a user-provided station identifier (which may be
// a station_id, poi_id, or name) to the canonical station_id used in market
// data. If no matching station is found, the original identifier is returned
// as-is so existing queries against market tables still work.
func (e *Engine) resolveStationID(ctx context.Context, identifier string) string {
	if identifier == "" {
		return ""
	}
	station, err := e.db.ResolveStation(ctx, identifier)
	if err != nil || station == nil {
		return identifier
	}
	return station.ID
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

	// Get primary output for stats
	var primaryOutput crafting.RecipeOutput
	if len(recipe.Outputs) > 0 {
		primaryOutput = recipe.Outputs[0]
	} else {
		return nil, nil // No outputs
	}

	// Get market price stats for output
	outputStats, err := e.market.GetPriceStats(ctx, primaryOutput.ItemID, stationID, "sell")
	if err != nil {
		return nil, err
	}

	// If no market data available, return nil
	if outputStats == nil {
		return nil, nil
	}

	// Calculate total output value from all outputs
	var totalOutputPrice int
	for _, output := range recipe.Outputs {
		var price int
		if output.ItemID == primaryOutput.ItemID {
			price = outputStats.RepresentativePrice
		} else {
			// For multi-output recipes, get stats for each output
			stats, err := e.market.GetPriceStats(ctx, output.ItemID, stationID, "sell")
			if err != nil {
				return nil, err
			}
			if stats == nil {
				// No market data for this output, can't calculate profit
				return nil, nil
			}
			price = stats.RepresentativePrice
		}
		totalOutputPrice += price * output.Quantity
	}

	// Calculate input cost using market stats
	var inputCost int
	for _, inp := range recipe.Inputs {
		inputStats, err := e.market.GetPriceStats(ctx, inp.ItemID, stationID, "buy")
		if err != nil {
			return nil, err
		}
		if inputStats == nil {
			// No market data for this input, use MSRP
			msrp, err := e.market.GetItemMSRP(ctx, inp.ItemID)
			if err != nil {
				return nil, err
			}
			inputCost += msrp * inp.Quantity
		} else {
			inputCost += inputStats.RepresentativePrice * inp.Quantity
		}
	}

	profitPerUnit := totalOutputPrice - inputCost

	var marginPct float64
	if inputCost > 0 {
		marginPct = float64(profitPerUnit) / float64(inputCost) * 100
	}

	// Get MSRP for primary output
	msrp, err := e.market.GetItemMSRP(ctx, primaryOutput.ItemID)
	if err != nil {
		return nil, err
	}

	// Determine market status from confidence score
	marketStatus := "no_market_data"
	if outputStats.ConfidenceScore >= 0.8 {
		marketStatus = "high_confidence"
	} else if outputStats.ConfidenceScore >= 0.5 {
		marketStatus = "medium_confidence"
	} else if outputStats.ConfidenceScore > 0 {
		marketStatus = "low_confidence"
	}

	// Handle nullable PriceTrend
	priceTrend := "unknown"
	if outputStats.PriceTrend != nil {
		priceTrend = *outputStats.PriceTrend
	}

	analysis := &crafting.ProfitAnalysis{
		OutputSellPrice: totalOutputPrice,
		InputCost:       inputCost,
		ProfitPerUnit:   profitPerUnit,
		ProfitMarginPct: marginPct,
		TotalVolume24h:  outputStats.TotalVolume,
		PriceTrend:      priceTrend,

		// NEW fields from Phase 3
		MSRP:          msrp,
		MarketStatus:  marketStatus,
		PricingMethod: outputStats.StatMethod,
		SampleCount:   outputStats.SampleCount,
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

// enrichRecipeWithIllegalStatus adds illegal status to recipe results
func (e *Engine) enrichRecipeWithIllegalStatus(
	ctx context.Context,
	recipe *crafting.Recipe,
) error {
	isIllegal, illegalInfo, err := e.illegalStore.IsIllegal(ctx, recipe.ID)
	if err != nil {
		return fmt.Errorf("checking illegal status: %w", err)
	}

	if isIllegal {
		recipe.IllegalStatus = &crafting.IllegalStatus{
			IsIllegal:     true,
			BanReason:     illegalInfo.BanReason,
			LegalLocation: illegalInfo.LegalLocation,
		}
	}

	return nil
}

// RecipeMarketProfitability calculates market profitability for all recipes.
// Returns recipes sorted by absolute profit (descending).
// components is an optional list of items the user currently has in inventory.
// For items in inventory, the input cost is set to 0 (since they already own them).
func (e *Engine) RecipeMarketProfitability(ctx context.Context, stationID, empireID string, components []crafting.Component) (*crafting.RecipeMarketProfitabilityResponse, error) {
	// Resolve station identifier
	stationID = e.resolveStationID(ctx, stationID)

	// Build inventory map from components for efficient lookup
	inventory := buildInventoryMap(components)

	// Get all recipes
	recipes, err := e.recipes.GetAllRecipes(ctx)
	if err != nil {
		return nil, err
	}

	var results []crafting.RecipeMarketProfit

	for _, recipe := range recipes {
		// Get primary output
		if len(recipe.Outputs) == 0 {
			continue
		}
		primaryOutput := recipe.Outputs[0]

		// Enrich with illegal status
		if err := e.enrichRecipeWithIllegalStatus(ctx, &recipe); err != nil {
			return nil, fmt.Errorf("enriching illegal status: %w", err)
		}

		// Calculate output price
		var outputSellPrice, outputMSRP int
		var outputUsesMSRP bool

		if stationID != "" {
			outputStats, err := e.market.GetPriceStats(ctx, primaryOutput.ItemID, stationID, "sell")
			if err != nil {
				return nil, err
			}

			outputMSRP, err = e.market.GetItemMSRP(ctx, primaryOutput.ItemID)
			if err != nil {
				return nil, err
			}

			if outputStats != nil {
				outputSellPrice = outputStats.RepresentativePrice * primaryOutput.Quantity
				outputUsesMSRP = false
			} else {
				outputSellPrice = outputMSRP * primaryOutput.Quantity
				outputUsesMSRP = true
			}
		} else {
			// No station, use MSRP for all
			msrp, err := e.market.GetItemMSRP(ctx, primaryOutput.ItemID)
			if err != nil {
				return nil, err
			}
			outputSellPrice = msrp * primaryOutput.Quantity
			outputMSRP = msrp
			outputUsesMSRP = true
		}

		// Calculate input cost
		var inputCost int
		var inputUsesMSRP bool

		for _, inp := range recipe.Inputs {
			// Check if user has this item in inventory
			available, hasItem := inventory[inp.ItemID]

			// Determine quantity to purchase
			quantityToBuy := inp.Quantity
			if hasItem && available >= inp.Quantity {
				// User has enough of this item, no need to buy
				continue
			} else if hasItem && available > 0 {
				// User has some but not enough, buy the shortfall
				quantityToBuy = inp.Quantity - available
			}
			// Otherwise (hasItem == false or available == 0), buy full quantity

			// Calculate cost for quantityToBuy
			if stationID != "" {
				inputStats, err := e.market.GetPriceStats(ctx, inp.ItemID, stationID, "buy")
				if err != nil {
					return nil, err
				}

				if inputStats != nil {
					inputCost += inputStats.RepresentativePrice * quantityToBuy
				} else {
					msrp, err := e.market.GetItemMSRP(ctx, inp.ItemID)
					if err != nil {
						return nil, err
					}
					inputCost += msrp * quantityToBuy
					inputUsesMSRP = true
				}
			} else {
				msrp, err := e.market.GetItemMSRP(ctx, inp.ItemID)
				if err != nil {
					return nil, err
				}
				inputCost += msrp * quantityToBuy
				inputUsesMSRP = true
			}
		}

		profit := outputSellPrice - inputCost

		var marginPct float64
		if inputCost > 0 {
			marginPct = float64(profit) / float64(inputCost) * 100
		}

		results = append(results, crafting.RecipeMarketProfit{
			RecipeID:       recipe.ID,
			RecipeName:     recipe.Name,
			Category:       recipe.Category,
			OutputItemID:   primaryOutput.ItemID,
			OutputQuantity: primaryOutput.Quantity,
			OutputSellPrice: outputSellPrice,
			OutputMSRP:     outputMSRP,
			OutputUsesMSRP:  outputUsesMSRP,
			InputCost:      inputCost,
			InputUsesMSRP:   inputUsesMSRP,
			Profit:         profit,
			ProfitMarginPct: marginPct,
			Illegal:        recipe.IllegalStatus != nil && recipe.IllegalStatus.IsIllegal,
		})
	}

	// Sort by absolute profit descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Profit > results[i].Profit {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	response := &crafting.RecipeMarketProfitabilityResponse{
		Recipes:      results,
		TotalRecipes: len(results),
		StationID:    stationID,
		EmpireID:     empireID,
	}

	return response, nil
}
