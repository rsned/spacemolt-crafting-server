package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// SkillStore handles skill data access.
type SkillStore struct {
	db *DB
}

// NewSkillStore creates a new SkillStore.
func NewSkillStore(db *DB) *SkillStore {
	return &SkillStore{db: db}
}

// GetSkill retrieves a single skill by ID.
func (s *SkillStore) GetSkill(ctx context.Context, id string) (*crafting.Skill, error) {
	skill := &crafting.Skill{ID: id}
	
	err := s.db.QueryRowContext(ctx, `
		SELECT name, category, description, max_level
		FROM skills WHERE id = ?
	`, id).Scan(
		&skill.Name,
		&skill.Category,
		&skill.Description,
		&skill.MaxLevel,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying skill: %w", err)
	}
	
	// Get prerequisites
	prereqs, err := s.getSkillPrerequisites(ctx, id)
	if err != nil {
		return nil, err
	}
	skill.Prerequisites = prereqs
	
	// Get XP thresholds
	thresholds, err := s.getXPThresholds(ctx, id)
	if err != nil {
		return nil, err
	}
	skill.XPThresholds = thresholds
	
	return skill, nil
}

// getSkillPrerequisites retrieves prerequisites for a skill.
func (s *SkillStore) getSkillPrerequisites(ctx context.Context, skillID string) ([]crafting.SkillRequirement, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT prereq_skill_id, level_required
		FROM skill_prerequisites
		WHERE skill_id = ?
	`, skillID)
	if err != nil {
		return nil, fmt.Errorf("querying skill prerequisites: %w", err)
	}
	defer func() { _ = rows.Close() }()
	
	var prereqs []crafting.SkillRequirement
	for rows.Next() {
		var sr crafting.SkillRequirement
		if err := rows.Scan(&sr.SkillID, &sr.LevelRequired); err != nil {
			return nil, fmt.Errorf("scanning prerequisite: %w", err)
		}
		prereqs = append(prereqs, sr)
	}
	
	return prereqs, rows.Err()
}

// getXPThresholds retrieves XP thresholds for a skill.
func (s *SkillStore) getXPThresholds(ctx context.Context, skillID string) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT xp_required
		FROM skill_levels
		WHERE skill_id = ?
		ORDER BY level ASC
	`, skillID)
	if err != nil {
		return nil, fmt.Errorf("querying XP thresholds: %w", err)
	}
	defer func() { _ = rows.Close() }()
	
	var thresholds []int
	for rows.Next() {
		var xp int
		if err := rows.Scan(&xp); err != nil {
			return nil, fmt.Errorf("scanning XP threshold: %w", err)
		}
		thresholds = append(thresholds, xp)
	}
	
	return thresholds, rows.Err()
}

// GetSkillName retrieves just the name of a skill (lightweight).
func (s *SkillStore) GetSkillName(ctx context.Context, id string) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `SELECT name FROM skills WHERE id = ?`, id).Scan(&name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("querying skill name: %w", err)
	}
	return name, nil
}

// GetXPForLevel retrieves the XP required to reach a specific level of a skill.
func (s *SkillStore) GetXPForLevel(ctx context.Context, skillID string, level int) (int, error) {
	var xp int
	err := s.db.QueryRowContext(ctx, `
		SELECT xp_required 
		FROM skill_levels 
		WHERE skill_id = ? AND level = ?
	`, skillID, level).Scan(&xp)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("querying XP for level: %w", err)
	}
	return xp, nil
}

// ListSkillsByCategory lists all skills in a category.
func (s *SkillStore) ListSkillsByCategory(ctx context.Context, category string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM skills WHERE category = ?
	`, category)
	if err != nil {
		return nil, fmt.Errorf("listing skills by category: %w", err)
	}
	defer func() { _ = rows.Close() }()
	
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning skill id: %w", err)
		}
		ids = append(ids, id)
	}
	
	return ids, rows.Err()
}

// GetAllSkillIDs returns all skill IDs.
func (s *SkillStore) GetAllSkillIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM skills`)
	if err != nil {
		return nil, fmt.Errorf("listing all skills: %w", err)
	}
	defer func() { _ = rows.Close() }()
	
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning skill id: %w", err)
		}
		ids = append(ids, id)
	}
	
	return ids, rows.Err()
}

// FindRecipesRequiringSkillLevel finds recipes that require a specific skill at a level
// greater than currentLevel but <= targetLevel.
// Used to find what recipes unlock at the next level.
func (s *SkillStore) FindRecipesUnlockedAtLevel(ctx context.Context, skillID string, level int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT recipe_id 
		FROM recipe_skills 
		WHERE skill_id = ? AND level_required = ?
	`, skillID, level)
	if err != nil {
		return nil, fmt.Errorf("finding recipes unlocked at level: %w", err)
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

// CountRecipesLockedBySkill counts how many recipes are locked by insufficient skill level.
func (s *SkillStore) CountRecipesLockedBySkill(ctx context.Context, skillID string, currentLevel int) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT recipe_id)
		FROM recipe_skills
		WHERE skill_id = ? AND level_required > ?
	`, skillID, currentLevel).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting locked recipes: %w", err)
	}
	return count, nil
}

// BulkInsertSkills inserts multiple skills in a transaction.
func (s *SkillStore) BulkInsertSkills(ctx context.Context, skills []crafting.Skill) error {
	return s.db.InTransaction(ctx, func(tx *sql.Tx) error {
		skillStmt, err := tx.PrepareContext(ctx, `
			INSERT OR REPLACE INTO skills (id, name, category, description, max_level)
			VALUES (?, ?, ?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("preparing skill statement: %w", err)
		}
		defer func() { _ = skillStmt.Close() }()
		
		prereqStmt, err := tx.PrepareContext(ctx, `
			INSERT OR REPLACE INTO skill_prerequisites (skill_id, prereq_skill_id, level_required)
			VALUES (?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("preparing prerequisite statement: %w", err)
		}
		defer func() { _ = prereqStmt.Close() }()
		
		levelStmt, err := tx.PrepareContext(ctx, `
			INSERT OR REPLACE INTO skill_levels (skill_id, level, xp_required)
			VALUES (?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("preparing level statement: %w", err)
		}
		defer func() { _ = levelStmt.Close() }()
		
		for _, sk := range skills {
			_, err := skillStmt.ExecContext(ctx,
				sk.ID, sk.Name, sk.Category, sk.Description, sk.MaxLevel,
			)
			if err != nil {
				return fmt.Errorf("inserting skill %s: %w", sk.ID, err)
			}
			
			for _, prereq := range sk.Prerequisites {
				_, err := prereqStmt.ExecContext(ctx, sk.ID, prereq.SkillID, prereq.LevelRequired)
				if err != nil {
					return fmt.Errorf("inserting prerequisite for %s: %w", sk.ID, err)
				}
			}
			
			for level, xp := range sk.XPThresholds {
				_, err := levelStmt.ExecContext(ctx, sk.ID, level+1, xp) // levels are 1-indexed
				if err != nil {
					return fmt.Errorf("inserting level for %s: %w", sk.ID, err)
				}
			}
		}
		
		return nil
	})
}

// ClearSkills removes all skill data.
func (s *SkillStore) ClearSkills(ctx context.Context) error {
	return s.db.InTransaction(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM skills`)
		return err
	})
}
