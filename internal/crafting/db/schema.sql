-- SpaceMolt Crafting Server Schema
-- SQLite 3

-- ============================================
-- RECIPE DATA
-- ============================================

CREATE TABLE IF NOT EXISTS recipes (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT,
    category        TEXT,
    craft_time_sec  INTEGER DEFAULT 0,
    output_item_id  TEXT NOT NULL,
    output_quantity INTEGER DEFAULT 1
);

CREATE TABLE IF NOT EXISTS recipe_components (
    recipe_id       TEXT NOT NULL,
    component_id    TEXT NOT NULL,
    quantity        INTEGER NOT NULL,
    PRIMARY KEY (recipe_id, component_id),
    FOREIGN KEY (recipe_id) REFERENCES recipes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS recipe_skills (
    recipe_id       TEXT NOT NULL,
    skill_id        TEXT NOT NULL,
    level_required  INTEGER NOT NULL,
    PRIMARY KEY (recipe_id, skill_id),
    FOREIGN KEY (recipe_id) REFERENCES recipes(id) ON DELETE CASCADE
);

-- Inverted index for fast component lookups
CREATE INDEX IF NOT EXISTS idx_recipe_components_component ON recipe_components(component_id);
CREATE INDEX IF NOT EXISTS idx_recipes_category ON recipes(category);
CREATE INDEX IF NOT EXISTS idx_recipes_output ON recipes(output_item_id);

-- ============================================
-- SKILL DATA
-- ============================================

CREATE TABLE IF NOT EXISTS skills (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    category        TEXT,
    description     TEXT,
    max_level       INTEGER DEFAULT 10
);

CREATE TABLE IF NOT EXISTS skill_prerequisites (
    skill_id        TEXT NOT NULL,
    prereq_skill_id TEXT NOT NULL,
    level_required  INTEGER NOT NULL,
    PRIMARY KEY (skill_id, prereq_skill_id),
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE,
    FOREIGN KEY (prereq_skill_id) REFERENCES skills(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS skill_levels (
    skill_id        TEXT NOT NULL,
    level           INTEGER NOT NULL,
    xp_required     INTEGER NOT NULL,
    PRIMARY KEY (skill_id, level),
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_skill_levels_skill ON skill_levels(skill_id);

-- ============================================
-- MARKET DATA
-- ============================================

CREATE TABLE IF NOT EXISTS market_prices (
    component_id    TEXT NOT NULL,
    station_id      TEXT NOT NULL,
    price_type      TEXT NOT NULL CHECK (price_type IN ('buy', 'sell')),
    price           INTEGER NOT NULL,
    volume_24h      INTEGER,
    recorded_at     TEXT NOT NULL,
    PRIMARY KEY (component_id, station_id, price_type, recorded_at)
);

CREATE TABLE IF NOT EXISTS market_price_summary (
    component_id    TEXT NOT NULL,
    station_id      TEXT NOT NULL,
    price_type      TEXT NOT NULL CHECK (price_type IN ('buy', 'sell')),
    avg_price_7d    REAL,
    min_price_7d    INTEGER,
    max_price_7d    INTEGER,
    price_trend     TEXT CHECK (price_trend IN ('rising', 'falling', 'stable')),
    last_updated    TEXT,
    PRIMARY KEY (component_id, station_id, price_type)
);

CREATE INDEX IF NOT EXISTS idx_market_prices_component ON market_prices(component_id);
CREATE INDEX IF NOT EXISTS idx_market_prices_recorded ON market_prices(recorded_at);
CREATE INDEX IF NOT EXISTS idx_market_summary_component ON market_price_summary(component_id);

-- ============================================
-- COMPONENT METADATA (for names, etc.)
-- ============================================

CREATE TABLE IF NOT EXISTS components (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    category        TEXT,
    description     TEXT
);

-- ============================================
-- METADATA
-- ============================================

CREATE TABLE IF NOT EXISTS sync_metadata (
    key             TEXT PRIMARY KEY,
    value           TEXT,
    updated_at      TEXT DEFAULT (datetime('now'))
);
