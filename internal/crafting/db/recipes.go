package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// RecipeStore handles recipe data access.
type RecipeStore struct {
	db *DB
}

// NewRecipeStore creates a new RecipeStore.
func NewRecipeStore(db *DB) *RecipeStore {
	return &RecipeStore{db: db}
}

// GetRecipe retrieves a single recipe by ID with all its components and skill requirements.
func (s *RecipeStore) GetRecipe(ctx context.Context, id string) (*crafting.Recipe, error) {
	recipe := &crafting.Recipe{ID: id}
	
	// Get base recipe info
	err := s.db.QueryRowContext(ctx, `
		SELECT name, description, category, craft_time_sec, output_item_id, output_quantity
		FROM recipes WHERE id = ?
	`, id).Scan(
		&recipe.Name,
		&recipe.Description,
		&recipe.Category,
		&recipe.CraftTimeSec,
		&recipe.Output.ItemID,
		&recipe.Output.Quantity,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying recipe: %w", err)
	}
	
	// Get components
	components, err := s.getRecipeComponents(ctx, id)
	if err != nil {
		return nil, err
	}
	recipe.Components = components
	
	// Get skill requirements
	skills, err := s.getRecipeSkills(ctx, id)
	if err != nil {
		return nil, err
	}
	recipe.SkillsRequired = skills
	
	return recipe, nil
}

// getRecipeComponents retrieves components for a recipe.
func (s *RecipeStore) getRecipeComponents(ctx context.Context, recipeID string) ([]crafting.RecipeComponent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT component_id, quantity
		FROM recipe_components
		WHERE recipe_id = ?
	`, recipeID)
	if err != nil {
		return nil, fmt.Errorf("querying recipe components: %w", err)
	}
	defer func() { _ = rows.Close() }()
	
	var components []crafting.RecipeComponent
	for rows.Next() {
		var c crafting.RecipeComponent
		if err := rows.Scan(&c.ComponentID, &c.Quantity); err != nil {
			return nil, fmt.Errorf("scanning component: %w", err)
		}
		components = append(components, c)
	}
	
	return components, rows.Err()
}

// getRecipeSkills retrieves skill requirements for a recipe.
func (s *RecipeStore) getRecipeSkills(ctx context.Context, recipeID string) ([]crafting.SkillRequirement, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT skill_id, level_required
		FROM recipe_skills
		WHERE recipe_id = ?
	`, recipeID)
	if err != nil {
		return nil, fmt.Errorf("querying recipe skills: %w", err)
	}
	defer func() { _ = rows.Close() }()
	
	var skills []crafting.SkillRequirement
	for rows.Next() {
		var sr crafting.SkillRequirement
		if err := rows.Scan(&sr.SkillID, &sr.LevelRequired); err != nil {
			return nil, fmt.Errorf("scanning skill requirement: %w", err)
		}
		skills = append(skills, sr)
	}
	
	return skills, rows.Err()
}

// FindRecipesByComponents finds recipes that use any of the given components.
// Returns recipe IDs for further processing.
func (s *RecipeStore) FindRecipesByComponents(ctx context.Context, componentIDs []string) ([]string, error) {
	if len(componentIDs) == 0 {
		return nil, nil
	}
	
	// Build placeholders
	placeholders := make([]string, len(componentIDs))
	args := make([]interface{}, len(componentIDs))
	for i, id := range componentIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	
	query := fmt.Sprintf(`
		SELECT DISTINCT recipe_id 
		FROM recipe_components 
		WHERE component_id IN (%s)
	`, strings.Join(placeholders, ","))
	
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("finding recipes by components: %w", err)
	}
	defer func() { _ = rows.Close() }()
	
	var recipeIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning recipe id: %w", err)
		}
		recipeIDs = append(recipeIDs, id)
	}
	
	return recipeIDs, rows.Err()
}

// FindRecipesByOutput finds recipes that produce a given item.
func (s *RecipeStore) FindRecipesByOutput(ctx context.Context, itemID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM recipes WHERE output_item_id = ?
	`, itemID)
	if err != nil {
		return nil, fmt.Errorf("finding recipes by output: %w", err)
	}
	defer func() { _ = rows.Close() }()
	
	var recipeIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning recipe id: %w", err)
		}
		recipeIDs = append(recipeIDs, id)
	}
	
	return recipeIDs, rows.Err()
}

// SearchRecipes searches recipes by name (case-insensitive partial match).
func (s *RecipeStore) SearchRecipes(ctx context.Context, term string, limit int) ([]crafting.RecipeSearchHit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, category
		FROM recipes
		WHERE name LIKE ?
		LIMIT ?
	`, "%"+term+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("searching recipes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	
	var results []crafting.RecipeSearchHit
	for rows.Next() {
		var hit crafting.RecipeSearchHit
		if err := rows.Scan(&hit.RecipeID, &hit.Name, &hit.Category); err != nil {
			return nil, fmt.Errorf("scanning search hit: %w", err)
		}
		results = append(results, hit)
	}
	
	return results, rows.Err()
}

// ListRecipesByCategory lists all recipes in a category.
func (s *RecipeStore) ListRecipesByCategory(ctx context.Context, category string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM recipes WHERE category = ?
	`, category)
	if err != nil {
		return nil, fmt.Errorf("listing recipes by category: %w", err)
	}
	defer func() { _ = rows.Close() }()
	
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning recipe id: %w", err)
		}
		ids = append(ids, id)
	}
	
	return ids, rows.Err()
}

// GetAllRecipeIDs returns all recipe IDs in the database.
func (s *RecipeStore) GetAllRecipeIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM recipes`)
	if err != nil {
		return nil, fmt.Errorf("listing all recipes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning recipe id: %w", err)
		}
		ids = append(ids, id)
	}
	
	return ids, rows.Err()
}

// CountRecipes returns the total number of recipes.
func (s *RecipeStore) CountRecipes(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM recipes`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting recipes: %w", err)
	}
	return count, nil
}

// GetAllRecipes retrieves all recipes with their components and skill requirements.
func (s *RecipeStore) GetAllRecipes(ctx context.Context) ([]crafting.Recipe, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, category, craft_time_sec, output_item_id, output_quantity
		FROM recipes
	`)
	if err != nil {
		return nil, fmt.Errorf("querying all recipes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var recipes []crafting.Recipe
	for rows.Next() {
		var r crafting.Recipe
		if err := rows.Scan(
			&r.ID,
			&r.Name,
			&r.Description,
			&r.Category,
			&r.CraftTimeSec,
			&r.Output.ItemID,
			&r.Output.Quantity,
		); err != nil {
			return nil, fmt.Errorf("scanning recipe: %w", err)
		}
		recipes = append(recipes, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load components and skills for all recipes
	for i := range recipes {
		components, err := s.getRecipeComponents(ctx, recipes[i].ID)
		if err != nil {
			return nil, fmt.Errorf("loading components for %s: %w", recipes[i].ID, err)
		}
		recipes[i].Components = components

		skills, err := s.getRecipeSkills(ctx, recipes[i].ID)
		if err != nil {
			return nil, fmt.Errorf("loading skills for %s: %w", recipes[i].ID, err)
		}
		recipes[i].SkillsRequired = skills
	}

	return recipes, nil
}

// GetRecipesUsingOutput finds recipes that use a given item as an input component.
func (s *RecipeStore) GetRecipesUsingOutput(ctx context.Context, itemID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT recipe_id 
		FROM recipe_components 
		WHERE component_id = ?
	`, itemID)
	if err != nil {
		return nil, fmt.Errorf("finding recipes using item: %w", err)
	}
	defer func() { _ = rows.Close() }()
	
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning recipe id: %w", err)
		}
		ids = append(ids, id)
	}
	
	return ids, rows.Err()
}

// BulkInsertRecipes inserts multiple recipes in a transaction.
func (s *RecipeStore) BulkInsertRecipes(ctx context.Context, recipes []crafting.Recipe) error {
	return s.db.InTransaction(ctx, func(tx *sql.Tx) error {
		// Prepare statements
		recipeStmt, err := tx.PrepareContext(ctx, `
			INSERT OR REPLACE INTO recipes 
			(id, name, description, category, craft_time_sec, output_item_id, output_quantity)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("preparing recipe statement: %w", err)
		}
		defer func() { _ = recipeStmt.Close() }()
		
		compStmt, err := tx.PrepareContext(ctx, `
			INSERT OR REPLACE INTO recipe_components (recipe_id, component_id, quantity)
			VALUES (?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("preparing component statement: %w", err)
		}
		defer func() { _ = compStmt.Close() }()
		
		skillStmt, err := tx.PrepareContext(ctx, `
			INSERT OR REPLACE INTO recipe_skills (recipe_id, skill_id, level_required)
			VALUES (?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("preparing skill statement: %w", err)
		}
		defer func() { _ = skillStmt.Close() }()
		
		for _, r := range recipes {
			_, err := recipeStmt.ExecContext(ctx,
				r.ID, r.Name, r.Description, r.Category,
				r.CraftTimeSec, r.Output.ItemID, r.Output.Quantity,
			)
			if err != nil {
				return fmt.Errorf("inserting recipe %s: %w", r.ID, err)
			}
			
			for _, c := range r.Components {
				_, err := compStmt.ExecContext(ctx, r.ID, c.ComponentID, c.Quantity)
				if err != nil {
					return fmt.Errorf("inserting component for %s: %w", r.ID, err)
				}
			}
			
			for _, sk := range r.SkillsRequired {
				_, err := skillStmt.ExecContext(ctx, r.ID, sk.SkillID, sk.LevelRequired)
				if err != nil {
					return fmt.Errorf("inserting skill for %s: %w", r.ID, err)
				}
			}
		}
		
		return nil
	})
}

// ClearRecipes removes all recipe data (for re-sync).
func (s *RecipeStore) ClearRecipes(ctx context.Context) error {
	return s.db.InTransaction(ctx, func(tx *sql.Tx) error {
		// Foreign keys will cascade delete components and skills
		_, err := tx.ExecContext(ctx, `DELETE FROM recipes`)
		return err
	})
}
