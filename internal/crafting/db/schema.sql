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
    crafting_time   INTEGER DEFAULT 0,
    facility_only   BOOLEAN DEFAULT 0,
    no_recycle      BOOLEAN DEFAULT 0,
    fuel_output     INTEGER DEFAULT 0,
    last_updated_tick INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS recipe_inputs (
    recipe_id       TEXT NOT NULL,
    item_id         TEXT NOT NULL,
    quantity        INTEGER NOT NULL,
    PRIMARY KEY (recipe_id, item_id),
    FOREIGN KEY (recipe_id) REFERENCES recipes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS recipe_outputs (
    recipe_id       TEXT NOT NULL,
    item_id         TEXT NOT NULL,
    quantity        INTEGER NOT NULL,
    PRIMARY KEY (recipe_id, item_id),
    FOREIGN KEY (recipe_id) REFERENCES recipes(id) ON DELETE CASCADE
);

-- Inverted indexes for fast lookups
CREATE INDEX IF NOT EXISTS idx_recipe_inputs_item ON recipe_inputs(item_id);
CREATE INDEX IF NOT EXISTS idx_recipe_outputs_item ON recipe_outputs(item_id);
CREATE INDEX IF NOT EXISTS idx_recipes_category ON recipes(category);

-- ============================================
-- SKILL DATA
-- ============================================

CREATE TABLE IF NOT EXISTS skills (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT,
    category        TEXT,
    max_level       INTEGER DEFAULT 10,
    training_source TEXT,
    xp_per_level    TEXT DEFAULT '[]',
    bonus_per_level TEXT DEFAULT '{}',
    required_skills TEXT DEFAULT '{}',
    empire_restriction TEXT,
    last_updated_tick INTEGER DEFAULT 0
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
    item_id         TEXT NOT NULL,
    station_id      TEXT NOT NULL,
    empire_id       TEXT,
    price_type      TEXT NOT NULL CHECK (price_type IN ('buy', 'sell')),
    price           INTEGER NOT NULL,
    volume_24h      INTEGER,
    recorded_at     TEXT NOT NULL,
    PRIMARY KEY (item_id, station_id, price_type, recorded_at)
);

CREATE TABLE IF NOT EXISTS market_price_summary (
    item_id         TEXT NOT NULL,
    station_id      TEXT NOT NULL,
    price_type      TEXT NOT NULL CHECK (price_type IN ('buy', 'sell')),
    avg_price_7d    REAL,
    min_price_7d    INTEGER,
    max_price_7d    INTEGER,
    price_trend     TEXT CHECK (price_trend IN ('rising', 'falling', 'stable')),
    last_updated    TEXT,
    PRIMARY KEY (item_id, station_id, price_type)
);

CREATE TABLE IF NOT EXISTS market_order_book (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    batch_id        TEXT NOT NULL,
    item_id         TEXT NOT NULL,
    station_id      TEXT NOT NULL,
    empire_id       TEXT,
    order_type      TEXT NOT NULL CHECK (order_type IN ('buy', 'sell')),
    price_per_unit  INTEGER NOT NULL,
    volume_available INTEGER NOT NULL,
    player_stall_name TEXT,
    recorded_at     TEXT NOT NULL,
    submitter_id    TEXT,
    created_at      TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (item_id) REFERENCES items(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS market_price_stats (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id             TEXT NOT NULL,
    station_id          TEXT NOT NULL,
    empire_id           TEXT,
    order_type          TEXT NOT NULL CHECK (order_type IN ('buy', 'sell')),
    stat_method         TEXT NOT NULL,
    representative_price INTEGER NOT NULL,
    sample_count        INTEGER NOT NULL,
    total_volume        INTEGER NOT NULL,
    min_price           INTEGER NOT NULL,
    max_price           INTEGER NOT NULL,
    stddev              REAL,
    confidence_score    REAL NOT NULL,
    price_trend         TEXT CHECK (price_trend IN ('rising', 'falling', 'stable')),
    last_updated        TEXT NOT NULL,
    UNIQUE(item_id, station_id, empire_id, order_type),
    FOREIGN KEY (item_id) REFERENCES items(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_market_prices_item ON market_prices(item_id);
CREATE INDEX IF NOT EXISTS idx_market_prices_recorded ON market_prices(recorded_at);
CREATE INDEX IF NOT EXISTS idx_market_summary_item ON market_price_summary(item_id);
CREATE INDEX IF NOT EXISTS idx_order_book_lookup ON market_order_book(item_id, station_id, order_type, recorded_at);
CREATE INDEX IF NOT EXISTS idx_order_book_batch ON market_order_book(batch_id);
CREATE INDEX IF NOT EXISTS idx_order_book_stale ON market_order_book(recorded_at);
CREATE INDEX IF NOT EXISTS idx_price_stats_lookup ON market_price_stats(item_id, station_id, order_type);

-- ============================================
-- ITEM METADATA (for names, etc.)
-- ============================================

CREATE TABLE IF NOT EXISTS items (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT,
    category        TEXT,
    rarity          TEXT,
    size            INTEGER DEFAULT 1,
    base_value      INTEGER DEFAULT 0,
    stackable       BOOLEAN DEFAULT 0,
    tradeable       BOOLEAN DEFAULT 0,
    attributes      TEXT DEFAULT '{}',
    last_updated_tick INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_items_category ON items(category);

-- ============================================
-- STATION DATA
-- ============================================

CREATE TABLE IF NOT EXISTS stations (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    poi_id          TEXT,
    empire          TEXT NOT NULL
);

-- ============================================
-- METADATA
-- ============================================

CREATE TABLE IF NOT EXISTS schema_migrations (
    migration_id TEXT PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sync_metadata (
    key             TEXT PRIMARY KEY,
    value           TEXT,
    updated_at      TEXT DEFAULT (datetime('now'))
);

-- Database version tracking
CREATE TABLE IF NOT EXISTS version (
    id              INTEGER PRIMARY KEY CHECK (id = 1),
    game_version    TEXT NOT NULL,
    imported_at     TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Ensure only one row in version table
CREATE TRIGGER IF NOT EXISTS version_single_row INSERT ON version WHEN NEW.id != 1
BEGIN
    SELECT RAISE(ABORT, 'Only one row allowed in version table with id=1');
END;


-- ============================================
-- ILLEGAL RECIPES
-- ============================================

CREATE TABLE IF NOT EXISTS illegal_recipes (
    recipe_id       TEXT PRIMARY KEY,
    ban_reason      TEXT NOT NULL,
    legal_location  TEXT NOT NULL,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (recipe_id) REFERENCES recipes(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_illegal_recipes_recipe_id ON illegal_recipes(recipe_id);

-- ============================================
-- CATEGORY PRIORITY DATA
-- ============================================

CREATE TABLE IF NOT EXISTS category_priorities (
    category TEXT PRIMARY KEY,
    priority_tier INTEGER NOT NULL CHECK (priority_tier BETWEEN 1 AND 6),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_category_priorities_tier ON category_priorities(priority_tier);
