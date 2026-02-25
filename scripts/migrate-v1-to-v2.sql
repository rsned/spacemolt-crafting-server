-- Migration script to update database from v1 to v2 schema
-- This migrates craft_time_sec to crafting_time and adds missing skill columns

-- Migrate recipes table: rename craft_time_sec to crafting_time
-- SQLite doesn't support ALTER TABLE RENAME COLUMN directly, so we need to recreate
BEGIN TRANSACTION;

-- Create new recipes table with v2 schema
CREATE TABLE IF NOT EXISTS recipes_v2 (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT,
    category        TEXT,
    crafting_time   INTEGER DEFAULT 0,
    base_quality    INTEGER DEFAULT 0,
    skill_quality_mod INTEGER DEFAULT 0,
    required_skills TEXT DEFAULT '{}',
    last_updated_tick INTEGER DEFAULT 0
);

-- Copy data from old table to new table
INSERT INTO recipes_v2 (id, name, description, category, crafting_time, base_quality, skill_quality_mod, required_skills, last_updated_tick)
SELECT id, name, description, category, craft_time_sec, base_quality, skill_quality_mod, required_skills, last_updated_tick
FROM recipes;

-- Drop old table and rename new table
DROP TABLE recipes;
ALTER TABLE recipes_v2 RENAME TO recipes;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_recipe_inputs_item ON recipe_inputs(item_id);
CREATE INDEX IF NOT EXISTS idx_recipe_outputs_item ON recipe_outputs(item_id);
CREATE INDEX IF NOT EXISTS idx_recipes_category ON recipes(category);

COMMIT;

-- Migrate skills table: add missing columns
BEGIN TRANSACTION;

-- Create new skills table with v2 schema
CREATE TABLE IF NOT EXISTS skills_v2 (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT,
    category        TEXT,
    max_level       INTEGER DEFAULT 10,
    training_source TEXT,
    xp_per_level    TEXT DEFAULT '[]',
    bonus_per_level TEXT DEFAULT '{}',
    required_skills TEXT DEFAULT '{}',
    last_updated_tick INTEGER DEFAULT 0
);

-- Copy data from old table to new table (new columns will use defaults)
INSERT INTO skills_v2 (id, name, description, category, max_level)
SELECT id, name, description, category, max_level
FROM skills;

-- Drop old table and rename new table
DROP TABLE skills;
ALTER TABLE skills_v2 RENAME TO skills;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_skill_levels_skill ON skill_levels(skill_id);

COMMIT;
