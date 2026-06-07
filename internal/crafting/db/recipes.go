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

// GetRecipe retrieves a single recipe by ID with all its inputs and outputs.
func (s *RecipeStore) GetRecipe(ctx context.Context, id string) (*crafting.Recipe, error) {
	recipe := &crafting.Recipe{ID: id}

	err := s.db.QueryRowContext(ctx, `
		SELECT name, description, category, crafting_time, facility_only, no_recycle, fuel_output
		FROM recipes WHERE id = ?
	`, id).Scan(
		&recipe.Name,
		&recipe.Description,
		&recipe.Category,
		&recipe.CraftingTime,
		&recipe.FacilityOnly,
		&recipe.NoRecycle,
		&recipe.FuelOutput,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying recipe: %w", err)
	}

	// Get inputs
	inputs, err := s.getRecipeInputs(ctx, id)
	if err != nil {
		return nil, err
	}
	recipe.Inputs = inputs

	// Get outputs
	outputs, err := s.getRecipeOutputs(ctx, id)
	if err != nil {
		return nil, err
	}
	recipe.Outputs = outputs

	return recipe, nil
}

// getRecipeInputs retrieves inputs for a recipe.
func (s *RecipeStore) getRecipeInputs(ctx context.Context, recipeID string) ([]crafting.RecipeInput, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT item_id, quantity
		FROM recipe_inputs
		WHERE recipe_id = ?
	`, recipeID)
	if err != nil {
		return nil, fmt.Errorf("querying recipe inputs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var inputs []crafting.RecipeInput
	for rows.Next() {
		var inp crafting.RecipeInput
		if err := rows.Scan(&inp.ItemID, &inp.Quantity); err != nil {
			return nil, fmt.Errorf("scanning input: %w", err)
		}
		inputs = append(inputs, inp)
	}

	return inputs, rows.Err()
}

// getRecipeOutputs retrieves outputs for a recipe.
func (s *RecipeStore) getRecipeOutputs(ctx context.Context, recipeID string) ([]crafting.RecipeOutput, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT item_id, quantity
		FROM recipe_outputs
		WHERE recipe_id = ?
	`, recipeID)
	if err != nil {
		return nil, fmt.Errorf("querying recipe outputs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var outputs []crafting.RecipeOutput
	for rows.Next() {
		var out crafting.RecipeOutput
		if err := rows.Scan(&out.ItemID, &out.Quantity); err != nil {
			return nil, fmt.Errorf("scanning output: %w", err)
		}
		outputs = append(outputs, out)
	}

	return outputs, rows.Err()
}

// FindRecipesByComponents finds recipes that use any of the given items as inputs.
// Returns recipe IDs for further processing.
func (s *RecipeStore) FindRecipesByComponents(ctx context.Context, itemIDs []string) ([]string, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}

	// Build placeholders
	placeholders := make([]string, len(itemIDs))
	args := make([]interface{}, len(itemIDs))
	for i, id := range itemIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT recipe_id
		FROM recipe_inputs
		WHERE item_id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("finding recipes by inputs: %w", err)
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
		SELECT DISTINCT recipe_id FROM recipe_outputs WHERE item_id = ?
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

// GetAllRecipes retrieves all recipes with their inputs and outputs.
func (s *RecipeStore) GetAllRecipes(ctx context.Context) ([]crafting.Recipe, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, category, crafting_time, facility_only, no_recycle, fuel_output
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
			&r.CraftingTime,
			&r.FacilityOnly,
			&r.NoRecycle,
			&r.FuelOutput,
		); err != nil {
			return nil, fmt.Errorf("scanning recipe: %w", err)
		}
		recipes = append(recipes, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load inputs and outputs for all recipes
	for i := range recipes {
		inputs, err := s.getRecipeInputs(ctx, recipes[i].ID)
		if err != nil {
			return nil, fmt.Errorf("loading inputs for %s: %w", recipes[i].ID, err)
		}
		recipes[i].Inputs = inputs

		outputs, err := s.getRecipeOutputs(ctx, recipes[i].ID)
		if err != nil {
			return nil, fmt.Errorf("loading outputs for %s: %w", recipes[i].ID, err)
		}
		recipes[i].Outputs = outputs
	}

	return recipes, nil
}

// GetRecipesUsingOutput finds recipes that use a given item as an input.
func (s *RecipeStore) GetRecipesUsingOutput(ctx context.Context, itemID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT recipe_id
		FROM recipe_inputs
		WHERE item_id = ?
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
		// Remove recipes that are no longer in the import set.
		importedIDs := make(map[string]struct{}, len(recipes))
		for _, r := range recipes {
			importedIDs[r.ID] = struct{}{}
		}

		// Fetch current recipe IDs to find ones to delete.
		rows, err := tx.QueryContext(ctx, `SELECT id FROM recipes`)
		if err != nil {
			return fmt.Errorf("querying existing recipes: %w", err)
		}
		var staleIDs []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scanning recipe id: %w", err)
			}
			if _, ok := importedIDs[id]; !ok {
				staleIDs = append(staleIDs, id)
			}
		}
		_ = rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterating existing recipes: %w", err)
		}

		// Delete stale recipes and their child rows explicitly
		// (CASCADE may not fire if foreign_keys pragma is off).
		if len(staleIDs) > 0 {
			delRecipeStmt, err := tx.PrepareContext(ctx, `DELETE FROM recipes WHERE id = ?`)
			if err != nil {
				return fmt.Errorf("preparing delete statement: %w", err)
			}
			defer func() { _ = delRecipeStmt.Close() }()

			delStaleInputs, err := tx.PrepareContext(ctx, `DELETE FROM recipe_inputs WHERE recipe_id = ?`)
			if err != nil {
				return fmt.Errorf("preparing delete stale inputs: %w", err)
			}
			defer func() { _ = delStaleInputs.Close() }()

			delStaleOutputs, err := tx.PrepareContext(ctx, `DELETE FROM recipe_outputs WHERE recipe_id = ?`)
			if err != nil {
				return fmt.Errorf("preparing delete stale outputs: %w", err)
			}
			defer func() { _ = delStaleOutputs.Close() }()

			for _, id := range staleIDs {
				if _, err := delStaleInputs.ExecContext(ctx, id); err != nil {
					return fmt.Errorf("deleting stale inputs for %s: %w", id, err)
				}
				if _, err := delStaleOutputs.ExecContext(ctx, id); err != nil {
					return fmt.Errorf("deleting stale outputs for %s: %w", id, err)
				}
				if _, err := delRecipeStmt.ExecContext(ctx, id); err != nil {
					return fmt.Errorf("deleting stale recipe %s: %w", id, err)
				}
			}
		}

		// Prepare statements
		recipeStmt, err := tx.PrepareContext(ctx, `
			INSERT OR REPLACE INTO recipes
			(id, name, description, category, crafting_time, facility_only, no_recycle, fuel_output, last_updated_tick)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("preparing recipe statement: %w", err)
		}
		defer func() { _ = recipeStmt.Close() }()

		// Prepare delete statements to clear old child rows before re-inserting.
		delInputsStmt, err := tx.PrepareContext(ctx, `DELETE FROM recipe_inputs WHERE recipe_id = ?`)
		if err != nil {
			return fmt.Errorf("preparing delete inputs statement: %w", err)
		}
		defer func() { _ = delInputsStmt.Close() }()

		delOutputsStmt, err := tx.PrepareContext(ctx, `DELETE FROM recipe_outputs WHERE recipe_id = ?`)
		if err != nil {
			return fmt.Errorf("preparing delete outputs statement: %w", err)
		}
		defer func() { _ = delOutputsStmt.Close() }()

		inputStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO recipe_inputs (recipe_id, item_id, quantity)
			VALUES (?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("preparing input statement: %w", err)
		}
		defer func() { _ = inputStmt.Close() }()

		outputStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO recipe_outputs (recipe_id, item_id, quantity)
			VALUES (?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("preparing output statement: %w", err)
		}
		defer func() { _ = outputStmt.Close() }()

		for _, r := range recipes {
			_, err := recipeStmt.ExecContext(ctx,
				r.ID, r.Name, r.Description, r.Category,
				r.CraftingTime, r.FacilityOnly, r.NoRecycle, r.FuelOutput,
				0, // last_updated_tick defaults to 0
			)
			if err != nil {
				return fmt.Errorf("inserting recipe %s: %w", r.ID, err)
			}

			// Clear old child rows before inserting current ones.
			if _, err := delInputsStmt.ExecContext(ctx, r.ID); err != nil {
				return fmt.Errorf("clearing inputs for %s: %w", r.ID, err)
			}
			if _, err := delOutputsStmt.ExecContext(ctx, r.ID); err != nil {
				return fmt.Errorf("clearing outputs for %s: %w", r.ID, err)
			}

			for _, inp := range r.Inputs {
				_, err := inputStmt.ExecContext(ctx, r.ID, inp.ItemID, inp.Quantity)
				if err != nil {
					return fmt.Errorf("inserting input for %s: %w", r.ID, err)
				}
			}

			for _, out := range r.Outputs {
				_, err := outputStmt.ExecContext(ctx, r.ID, out.ItemID, out.Quantity)
				if err != nil {
					return fmt.Errorf("inserting output for %s: %w", r.ID, err)
				}
			}
		}

		return nil
	})
}

// ClearRecipes removes all recipe data (for re-sync).
func (s *RecipeStore) ClearRecipes(ctx context.Context) error {
	return s.db.InTransaction(ctx, func(tx *sql.Tx) error {
		// Foreign keys will cascade delete inputs and outputs
		_, err := tx.ExecContext(ctx, `DELETE FROM recipes`)
		return err
	})
}
