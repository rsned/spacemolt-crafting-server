package engine

import (
	"context"
	"fmt"
	"sort"

	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// SkillCraftPaths executes the skill_craft_paths tool logic.
func (e *Engine) SkillCraftPaths(ctx context.Context, req crafting.SkillCraftPathsRequest) (*crafting.SkillCraftPathsResponse, error) {
	// Apply defaults
	if req.Limit <= 0 {
		req.Limit = 10
	}

	// Get all skills (optionally filtered by category)
	var skillIDs []string
	var err error
	if req.CategoryFilter != "" {
		skillIDs, err = e.skills.ListSkillsByCategory(ctx, req.CategoryFilter)
	} else {
		skillIDs, err = e.skills.GetAllSkillIDs(ctx)
	}
	if err != nil {
		return nil, err
	}

	// Get total recipe count
	totalRecipes, err := e.recipes.CountRecipes(ctx)
	if err != nil {
		return nil, err
	}

	var paths []crafting.SkillUnlockPath
	var totalUnlocked, totalLocked int
	var closestSkill string
	closestXP := -1

	for _, skillID := range skillIDs {
		skill, err := e.skills.GetSkill(ctx, skillID)
		if err != nil {
			return nil, err
		}
		if skill == nil {
			continue
		}

		// Get current level from request
		progress := req.Skills[skillID]
		currentLevel := progress.Level
		currentXP := progress.CurrentXP

		// Count locked recipes for this skill
		lockedCount, err := e.skills.CountRecipesLockedBySkill(ctx, skillID, currentLevel)
		if err != nil {
			return nil, err
		}
		totalLocked += lockedCount

		// Find recipes unlocked at next level
		nextLevel := currentLevel + 1
		if nextLevel > skill.MaxLevel {
			continue // Already maxed
		}

		recipeIDsAtNext, err := e.skills.FindRecipesUnlockedAtLevel(ctx, skillID, nextLevel)
		if err != nil {
			return nil, err
		}

		if len(recipeIDsAtNext) == 0 {
			continue // No recipes unlock at next level
		}

		// Fetch full recipe details and enrich with illegal status
		var recipesAtNext []crafting.Recipe
		for _, recipeID := range recipeIDsAtNext {
			recipe, err := e.recipes.GetRecipe(ctx, recipeID)
			if err != nil {
				return nil, fmt.Errorf("getting recipe %s: %w", recipeID, err)
			}
			if recipe == nil {
				continue
			}

			// Enrich with illegal status
			if err := e.enrichRecipeWithIllegalStatus(ctx, recipe); err != nil {
				return nil, fmt.Errorf("enriching illegal status: %w", err)
			}

			recipesAtNext = append(recipesAtNext, *recipe)
		}

		// Calculate XP to next level
		xpForNext, err := e.skills.GetXPForLevel(ctx, skillID, nextLevel)
		if err != nil {
			return nil, err
		}
		xpNeeded := xpForNext - currentXP
		if xpNeeded < 0 {
			xpNeeded = 0
		}

		paths = append(paths, crafting.SkillUnlockPath{
			Skill:           *skill,
			CurrentLevel:    currentLevel,
			XPToNextLevel:   xpNeeded,
			RecipesUnlocked: recipesAtNext,
		})

		// Track closest unlock
		if closestXP < 0 || xpNeeded < closestXP {
			closestXP = xpNeeded
			closestSkill = skillID
		}
	}

	// Calculate unlocked recipes
	totalUnlocked = totalRecipes - totalLocked

	// Sort by number of recipes unlocked (descending)
	sort.Slice(paths, func(i, j int) bool {
		return len(paths[i].RecipesUnlocked) > len(paths[j].RecipesUnlocked)
	})

	// Sort each skill's unlocked recipes by category tier
	for i := range paths {
		sort.Slice(paths[i].RecipesUnlocked, func(j, k int) bool {
			recipeJ := &paths[i].RecipesUnlocked[j]
			recipeK := &paths[i].RecipesUnlocked[k]

			tierJ := e.getCategoryTier(recipeJ.Category)
			tierK := e.getCategoryTier(recipeK.Category)
			if tierJ != tierK {
				return tierJ < tierK
			}

			// Within tier, sort by recipe name
			return recipeJ.Name < recipeK.Name
		})
	}

	// Apply limit
	if len(paths) > req.Limit {
		paths = paths[:req.Limit]
	}

	return &crafting.SkillCraftPathsResponse{
		SkillPaths: paths,
		Summary: crafting.SkillCraftPathsSummary{
			TotalRecipes:       totalRecipes,
			RecipesUnlocked:    totalUnlocked,
			RecipesLocked:      totalLocked,
			ClosestUnlockSkill: closestSkill,
			ClosestUnlockXP:    closestXP,
		},
	}, nil
}
