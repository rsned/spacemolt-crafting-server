package db

import (
	"context"
	"database/sql"
	"encoding/json"
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

	var trainingSource sql.NullString
	var xpPerLevel, bonusPerLevel, requiredSkills sql.NullString
	var empireRestriction sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT name, description, category, max_level, training_source, xp_per_level, bonus_per_level, required_skills, empire_restriction
		FROM skills WHERE id = ?
	`, id).Scan(
		&skill.Name,
		&skill.Description,
		&skill.Category,
		&skill.MaxLevel,
		&trainingSource,
		&xpPerLevel,
		&bonusPerLevel,
		&requiredSkills,
		&empireRestriction,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying skill: %w", err)
	}

	if trainingSource.Valid {
		skill.TrainingSource = trainingSource.String
	}
	if xpPerLevel.Valid && xpPerLevel.String != "[]" {
		skill.XPPerLevel = json.RawMessage(xpPerLevel.String)
	}
	if bonusPerLevel.Valid && bonusPerLevel.String != "{}" {
		skill.BonusPerLevel = json.RawMessage(bonusPerLevel.String)
	}
	if requiredSkills.Valid && requiredSkills.String != "{}" {
		skill.RequiredSkills = json.RawMessage(requiredSkills.String)
	}
	if empireRestriction.Valid {
		skill.EmpireRestriction = empireRestriction.String
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

// BulkInsertSkills inserts multiple skills in a transaction.
func (s *SkillStore) BulkInsertSkills(ctx context.Context, skills []crafting.Skill) error {
	return s.db.InTransaction(ctx, func(tx *sql.Tx) error {
		skillStmt, err := tx.PrepareContext(ctx, `
			INSERT OR REPLACE INTO skills (id, name, description, category, max_level, training_source, xp_per_level, bonus_per_level, required_skills, empire_restriction)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			xpPerLevel := "[]"
			if len(sk.XPPerLevel) > 0 {
				xpPerLevel = string(sk.XPPerLevel)
			}
			bonusPerLevel := "{}"
			if len(sk.BonusPerLevel) > 0 {
				bonusPerLevel = string(sk.BonusPerLevel)
			}
			requiredSkills := "{}"
			if len(sk.RequiredSkills) > 0 {
				requiredSkills = string(sk.RequiredSkills)
			}
			var empireRestriction any
			if sk.EmpireRestriction != "" {
				empireRestriction = sk.EmpireRestriction
			}

			_, err := skillStmt.ExecContext(ctx,
				sk.ID, sk.Name, sk.Description, sk.Category, sk.MaxLevel,
				sk.TrainingSource, xpPerLevel, bonusPerLevel, requiredSkills, empireRestriction,
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
