-- Migration 007: Add illegal_recipes table for Federation-banned crafting
-- Tracks recipes that are illegal to craft privately due to game patch changes

CREATE TABLE IF NOT EXISTS illegal_recipes (
  recipe_id TEXT PRIMARY KEY,
  ban_reason TEXT NOT NULL,
  legal_location TEXT NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (recipe_id) REFERENCES recipes(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_illegal_recipes_recipe_id
  ON illegal_recipes(recipe_id);
