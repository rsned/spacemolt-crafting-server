# SpaceMolt Crafting Query Server

A comprehensive server for SpaceMolt crafting queries with **market data integration** and **intelligent pricing**. Supports both MCP (Model Context Protocol) and HTTP API modes for AI agents and web services.

- Last rebuilt and repopulated the database against Server version: **v0.142.7**
- **NEW:** Market data submission and sophisticated pricing calculations
- **NEW:** HTTP API for real-time market data integration

## Features

### 7 Useful MCP Tools

1. **`craft_query`** - "What can I craft with my inventory?" (optional market pricing with station_id)
2. **`craft_path_to`** - "How do I craft this specific item?"
3. **`recipe_lookup`** - "Tell me about this recipe" (optional market pricing with station_id)
4. **`skill_craft_paths`** - "Which skills unlock new recipes?"
5. **`component_uses`** - "What can I do with this item?" (optional market pricing with station_id)
6. **`bill_of_materials`** - "What raw materials do I need?"
7. **`recipe_market_profitability`** ⭐ - "Show profitability for all recipes" (with inventory support)

### Market Data Integration ⭐ NEW

- **Real-time Market Data API**: Submit and query market prices via HTTP
- **Sophisticated Pricing Algorithms**: Volume-weighted, second-price auction, median
- **Automatic Outlier Handling**: Ignores price manipulation and extreme outliers
- **Market Confidence Scoring**: High/medium/low confidence based on sample size
- **Auto-recalculation**: Statistics updated automatically when new orders submitted
- **7-Day Order Retention**: Keeps order book bounded and fresh

### HTTP API Endpoints ⭐ NEW

```bash
# Submit market data
POST /api/v1/market/submit
Content-Type: application/json

{
  "station_id": "Grand Exchange Station",
  "empire_id": "empire_123",
  "orders": [
    {
      "item_id": "ore_iron",
      "order_type": "sell",
      "price_per_unit": 30,
      "volume_available": 128700
    }
  ]
}

# Query current market price
GET /api/v1/market/price/ore_iron

# Admin: manually recalculate stats
POST /api/v1/admin/market/recalc/ore_iron
```


The crafting server uses a rough priority ordering when returning recipes to try to ensure the best things can get crafted first rather than burning all your cycles and inventory making Std Ammo rounds and smelting iron.

### Recipe Distribution by Priority Tier

| Tier | Categories | Recipe Count |
|------|------------|--------------|
| 1 (Highest) | Shipbuilding, Legendary | 25 |
| 2 | Components, Weapons, Equipment | 125 |
| 3 | Consumables | 79 |
| 4 | Refining | 71 |
| 5 | (various) | ~50 |
| 6 (Lowest) | Defense, Utility, Drones, etc. | ~44 |

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/rsned/spacemolt-crafting-server.git
cd spacemolt-crafting-server

# Build the server
go build -o bin/crafting-server ./cmd/crafting-server

# (Optional) Install to PATH
cp bin/crafting-server ~/go/bin/
```

### Usage

#### As an MCP Server (Default)

```bash
# Run the server (communicates via stdin/stdout)
./bin/crafting-server -db crafting.db
```

#### As an HTTP Server ⭐ NEW

```bash
# Run in HTTP API mode on port 8080
./bin/crafting-server -db crafting.db -http :8080

# Server is now available at:
# - http://localhost:8080/api/v1/health
# - http://localhost:8080/api/v1/market/submit
# - http://localhost:8080/api/v1/market/price/:item_id
# - http://localhost:8080/api/v1/admin/market/recalc/:item_id
```

#### Database Snapshot

A pre-built database snapshot is available in the `database/` directory, containing all items, recipes, and skills already imported. You can use it directly:

```bash
# Copy the pre-built database
cp database/crafting.db ./

# Or run the server with the snapshot directly
./bin/crafting-server -db database/crafting.db
```

#### Starting From Scratch

To create and populate a fresh database from the game catalog JSON files:

```bash
# Build the server
go build -o bin/crafting-server ./cmd/crafting-server

# Import all data into a new database (the DB file is created automatically)
./bin/crafting-server -db crafting.db \
  -import-items /path/to/catalog_items.json \
  -import-recipes /path/to/catalog_recipes.json \
  -import-skills /path/to/catalog_skills.json \
  -verbose
```

The catalog JSON files use a `{"items": [...]}` envelope format, which the importer handles automatically. You can also import each file separately:

```bash
# Import items first (provides item metadata for names, categories, etc.)
./bin/crafting-server -db crafting.db -import-items catalog_items.json

# Import recipes (with inputs, outputs, and skill requirements)
./bin/crafting-server -db crafting.db -import-recipes catalog_recipes.json

# Import skills (with prerequisites and XP thresholds)
./bin/crafting-server -db crafting.db -import-skills catalog_skills.json

# Set game version (optional, tracks which server version the data came from)
./bin/crafting-server -db crafting.db -game-version v0.142.7 -import-items catalog_items.json

# (Optional) Import market data for profit calculations
./bin/crafting-server -db crafting.db -import-market market.json
```

### Checking Database Version

To see which game server version the database was built from:

```bash
./bin/crafting-server -db crafting.db -version
```

Output:
```
Game Version: v0.142.7
Imported At: 2026-02-20 20:09:00 PST
Updated At:  2026-02-24 19:06:35 PST
```

#### Verifying the Import

After importing, you can verify the data with SQLite:

```bash
sqlite3 crafting.db "
  SELECT 'items', COUNT(*) FROM items
  UNION ALL SELECT 'recipes', COUNT(*) FROM recipes
  UNION ALL SELECT 'skills', COUNT(*) FROM skills
  UNION ALL SELECT 'recipe_inputs', COUNT(*) FROM recipe_inputs
  UNION ALL SELECT 'recipe_outputs', COUNT(*) FROM recipe_outputs
  UNION ALL SELECT 'recipe_skills', COUNT(*) FROM recipe_skills
  UNION ALL SELECT 'skill_levels', COUNT(*) FROM skill_levels
  UNION ALL SELECT 'skill_prerequisites', COUNT(*) FROM skill_prerequisites;
"
```

Expected counts: ~688 items, 394 recipes, 138 skills, plus populated junction tables.

## Claude Code Integration

To use this MCP server with Claude Code, add it to your Claude Code configuration file.

### Configuration

Edit your Claude Code config file (typically `~/.config/claude/claude_desktop_config.json` on Linux/macOS or `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "spacemolt-crafting": {
      "command": "/path/to/spacemolt-crafting-server/bin/crafting-server",
      "args": [
        "-db",
        "/path/to/spacemolt-crafting-server/database/crafting.db"
      ]
    }
  }
}
```

**Important:** Update the paths to match your actual installation directory.

### Restart Claude Code

After updating the configuration, restart Claude Code to load the MCP server. The server will then be available to assist with crafting queries.

## Database

The server uses SQLite for fast, efficient recipe and skill queries:

- **Items:** 688 item definitions from the game catalog
- **Recipes:** 394 recipes from SpaceMolt
- **Skills:** 138 skill definitions
- **Database Size:** ~500KB (base) + variable for market data
- **Query Performance:** 1-5ms typical

### Schema

#### Core Tables
- `items` - Item metadata (name, category, rarity, base_value)
- `recipes` - Recipe metadata
- `recipe_inputs` - Required input items (inverted index)
- `recipe_outputs` - Recipe output items (supports multiple outputs)
- `recipe_skills` - Skill requirements per recipe
- `skills` - Skill definitions
- `skill_prerequisites` - Skill dependencies
- `skill_levels` - XP thresholds per level

#### Market Data Tables ⭐ NEW
- `market_order_book` - Individual buy/sell orders (7-day retention)
  - Stores raw order data with price, volume, station, timestamp
  - Used for sophisticated price calculations
  - Automatically pruned after 7 days

- `market_price_stats` - Pre-computed market statistics
  - `representative_price` - Calculated using hybrid pricing method
  - `stat_method` - volume_weighted, second_price, median, or msrp_only
  - `sample_count` - Number of orders used in calculation
  - `total_volume` - Total trading volume
  - `confidence_score` - Data quality indicator (0.0-1.0)
  - `min_price`, `max_price`, `stddev` - Price distribution metrics

- `market_prices` - Legacy price data (backward compatibility)
- `market_price_summary` - Legacy aggregated summaries

### Pricing Methodology ⭐ NEW

The server automatically selects the best pricing algorithm based on market characteristics:

| Method | Conditions | Use Case | Confidence |
|--------|------------|----------|------------|
| **Volume-Weighted** | 10+ orders AND 50K+ volume OR 100K+ volume | Flooded markets (ores) | 0.95 (high) |
| **Second-Price Auction** | 3+ orders | Normal liquidity | 0.75 (medium) |
| **Median** | 1+ order | Sparse data, single orders | 0.50 (low-medium) |
| **MSRP Only** | 0 orders | No market data | 0.0 (none) |

**Example:** Iron ore with 22 orders totaling 134,133 units uses volume-weighted average, giving massive orders (128,700 @ 30cr) appropriate weight while ignoring outliers (999cr).

## Example Queries

### What Can I Craft? (With Market-Aware Profit Analysis)

```json
{
  "method": "tools/call",
  "params": {
    "name": "craft_query",
    "arguments": {
      "components": [
        {"id": "ore_copper", "quantity": 50}
      ],
      "skills": {
        "crafting_basic": 1
      },
      "limit": 10
    }
  }
}
```

**Enhanced Response with Market Data:**

```json
{
  "profit_analysis": {
    "output_sell_price": 5000,
    "input_cost": 3200,
    "profit_per_unit": 1800,
    "profit_margin_pct": 36.0,
    "msrp": 4500,
    "market_status": "high_confidence",
    "pricing_method": "volume_weighted",
    "sample_count": 247,
    "total_volume_24h": 125000
  }
}
```

### HTTP API Usage ⭐ NEW

#### Submit Market Data

```bash
curl -X POST http://localhost:8080/api/v1/market/submit \
  -H "Content-Type: application/json" \
  -d '{
    "station_id": "Grand Exchange Station",
    "empire_id": "empire_123",
    "source": "manual_scan",
    "orders": [
      {
        "item_id": "ore_iron",
        "order_type": "sell",
        "price_per_unit": 30,
        "volume_available": 128700,
        "player_stall_name": "IronMiner42"
      }
    ]
  }'
```

**Response:**

```json
{
  "batch_id": "batch_20260228_150405",
  "orders_received": 1,
  "orders_accepted": 1,
  "orders_rejected": 0
}
```

#### Query Market Price

```bash
curl http://localhost:8080/api/v1/market/price/ore_iron
```

**Response:**

```json
{
  "item_id": "ore_iron",
  "sell_price": 29,
  "buy_price": 25,
  "msrp": 1,
  "method_name": "volume_weighted"
}
```

#### Admin: Manual Recalculation

```bash
curl -X POST http://localhost:8080/api/v1/admin/market/recalc/ore_iron
```

**Response:**

```json
{
  "status": "success",
  "item_id": "ore_iron",
  "station": "Grand Exchange Station"
}
```

### Bill of Materials

```json
{
  "method": "tools/call",
  "params": {
    "name": "bill_of_materials",
    "arguments": {
      "recipe_id": "craft_scanner_1",
      "quantity": 1
    }
  }
}
```

### Recipe Market Profitability ⭐ NEW

Get market profitability for all recipes, sorted by profit. Shows which items are most profitable to craft based on current market data or MSRP.

#### Basic Usage (MSRP Only)

```json
{
  "method": "tools/call",
  "params": {
    "name": "recipe_market_profitability",
    "arguments": {}
  }
}
```

**Response:**

```json
{
  "recipes": [
    {
      "recipe_id": "build_quantum_shield",
      "recipe_name": "Build Quantum Entanglement Shield",
      "output_sell_price": 750000,
      "output_msrp": 750000,
      "output_uses_msrp": true,
      "input_cost": 182000,
      "input_uses_msrp": true,
      "profit": 568000,
      "profit_margin_pct": 312.1
    }
  ],
  "total_recipes": 394
}
```

#### With Market Data

```json
{
  "method": "tools/call",
  "params": {
    "name": "recipe_market_profitability",
    "arguments": {
      "station_id": "jita_iv"
    }
  }
}
```

#### With Inventory Support ⭐ NEW

Specify items you already have in inventory. The tool will set input costs to 0 for items you own, showing true profit based on what you need to buy.

```json
{
  "method": "tools/call",
  "params": {
    "name": "recipe_market_profitability",
    "arguments": {
      "station_id": "jita_iv",
      "components": [
        {"id": "tritanium", "quantity": 1000},
        {"id": "pyerite", "quantity": 500}
      ]
    }
  }
}
```

**How inventory affects profit calculation:**

- **Full inventory:** If you have ≥ required quantity, cost = 0 (you already own it)
- **Partial inventory:** If you have some but not enough, cost = price × shortfall only
- **No inventory:** Full price for required quantity

**Example:** A recipe needs 10 tritanium at 5cr each:
- With 0 in inventory: input_cost = 50cr
- With 5 in inventory: input_cost = 25cr (only pay for 5 more)
- With 10+ in inventory: input_cost = 0cr (you have enough)

This enables accurate profitability calculations based on your actual inventory, not just theoretical market costs.


## Architecture

```
cmd/crafting-server/    # Main entry point
pkg/crafting/           # Public domain types
internal/
  ├── api/             # HTTP API server and handlers ⭐ NEW
  ├── db/              # Database layer (SQLite)
  │   ├── migrations/  # Database schema migrations
  │   └── market*.go   # Market data and statistics
  ├── engine/          # Query business logic
  ├── mcp/             # MCP protocol
  └── sync/            # Data import from catalog JSON
```

### Key Components

- **`internal/api/`** ⭐ NEW - HTTP server for market data API
  - Market data submission endpoint
  - Price query endpoint
  - Admin recalculation endpoint
  - Graceful shutdown and timeout handling

- **`internal/db/market*.go`** ⭐ NEW - Market data management
  - MarketStore for order book and price stats
  - StatsCalculator with hybrid pricing algorithms
  - Auto-recalculation after order submission
  - Old order pruning (7-day retention)

- **`internal/db/migrations/`** ⭐ NEW - Database versioning
  - Migration 005: Enhanced market tables
  - Automatic migration application
  - Schema version tracking

## Dependencies

- **Go:** 1.24 or later
- **External:** `modernc.org/sqlite` (pure Go SQLite driver)
- **Standard Library:** context, database/sql, encoding/json, log/slog

## Development

### Build

```bash
go build ./cmd/crafting-server
```

### Test

```bash
go test ./...
```

### Lint

```bash
golangci-lint run
```

## Configuration

Command-line options:

```
-db string
    Path to SQLite database (default "data/crafting/crafting.db")
-http string
    Start HTTP server on specified address (e.g., ":8080")
    When set, server runs in HTTP mode instead of MCP mode
-import-items string
    Import items from JSON file
-import-recipes string
    Import recipes from JSON file
-import-skills string
    Import skills from JSON file
-import-market string
    Import market data from JSON file
-game-version string
    Set game server version (e.g., "v0.142.7")
-version
    Show database version information and exit
-verbose
    Enable verbose logging
```

### HTTP Server Configuration ⭐ NEW

When running in HTTP mode (`-http :8080`), the server uses these timeouts:

- **Read Timeout:** 10 seconds
- **Write Timeout:** 10 seconds
- **Shutdown Timeout:** 5 seconds (graceful shutdown)

## Data Format

The importer accepts both flat JSON arrays and catalog envelope format (`{"items": [...], "total": N}`).

### Item JSON (Catalog Format)

```json
{
  "items": [
    {
      "id": "ore_copper",
      "name": "Copper Ore",
      "description": "Common metallic ore.",
      "category": "ore",
      "rarity": "common",
      "size": 1,
      "base_value": 10,
      "stackable": true,
      "tradeable": true
    }
  ]
}
```

### Recipe JSON (Catalog Format)

```json
{
  "items": [
    {
      "id": "craft_engine_core",
      "name": "Assemble Engine Core",
      "description": "Build propulsion system cores.",
      "category": "Components",
      "crafting_time": 10,
      "base_quality": 40,
      "skill_quality_mod": 6,
      "inputs": [
        {"item_id": "refined_alloy", "quantity": 3},
        {"item_id": "ore_cobalt", "quantity": 4}
      ],
      "outputs": [
        {"item_id": "comp_engine_core", "quantity": 1, "quality_mod": true}
      ],
      "required_skills": {
        "crafting_advanced": 2
      }
    }
  ]
}
```

### Skill JSON (Catalog Format)

```json
{
  "items": [
    {
      "id": "armor_advanced",
      "name": "Advanced Armor",
      "description": "Expert armor plating.",
      "category": "Combat",
      "max_level": 10,
      "training_source": "Take hull damage in combat.",
      "required_skills": {"armor": 5},
      "xp_per_level": [500, 1500, 3000, 5000, 8000, 12000, 17000, 23000, 30000, 40000],
      "bonus_per_level": {"armorEffectiveness": 2, "hullHP": 3}
    }
  ]
}
```

## Performance

- **Query Speed:** 1-5ms for typical queries
- **Database Size:** ~500KB (base) + variable for market data
  - Base: 688 items, 394 recipes, 138 skills
  - Market data: ~1KB per order (7-day retention)
- **Binary Size:** ~10MB
- **Memory Usage:** ~5MB typical (MCP mode), ~10MB (HTTP mode)
- **HTTP API:** Handles 1000+ concurrent submissions
- **Market Stats Calculation:** <50ms for typical items

## Development

### Build

```bash
go build ./cmd/crafting-server
```

### Test

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/api/...
go test ./internal/crafting/db/...
go test ./internal/crafting/engine/...

# Run with verbose output
go test -v ./...
```

### Test Market Data Features

```bash
# Test HTTP server with market data
go test -v ./internal/api/...

# Test pricing algorithms with real data
go test -v ./internal/crafting/db/... -run TestStatsCalculator

# Test profit calculations with market stats
go test -v ./internal/crafting/engine/... -run TestCalculateProfitAnalysis
```

### Lint

```bash
golangci-lint run
```

## Market Data Integration Guide ⭐ NEW

### Database Migrations ⭐ NEW

The server uses automatic database migrations to manage schema updates:

- **Migration 005:** Enhanced market tables (order book, price stats)
- Migrations run automatically on server startup
- Migration status tracked in `schema_migrations` table
- Backward compatible with existing databases

**View Applied Migrations:**

```bash
sqlite3 crafting.db "SELECT * FROM schema_migrations ORDER BY applied_at;"
```

**Manual Migration Control:**

The server handles migrations automatically, but you can verify the schema version:

```bash
./bin/crafting-server -db crafting.db -version
```

### Setting Up Market Data Collection

1. **Start the HTTP Server:**

```bash
./bin/crafting-server -db crafting.db -http :8080
```

2. **Submit Market Data:**

Use the game's `view_market` command output or manually scan station markets:

```python
import requests
import json

# From view_market.json
market_data = {
    "station_id": "Grand Exchange Station",
    "empire_id": "your_empire_id",
    "orders": [
        {
            "item_id": item["item_id"],
            "order_type": "sell",
            "price_per_unit": order["price_each"],
            "volume_available": order["quantity"],
            "player_stall_name": order.get("source", "")
        }
        for item in view_market_data["items"]
        for order in item["sell_orders"]
    ]
}

response = requests.post(
    "http://localhost:8080/api/v1/market/submit",
    json=market_data
)
print(response.json())
```

3. **Query Prices for Crafting:**

```bash
# Get current price before crafting
curl http://localhost:8080/api/v1/market/price/comp_steel
```

4. **MCP Tools Automatically Use Market Data:**

All MCP tools that return `profit_analysis` now include:
- Real market prices (not just MSRP)
- Market confidence level
- Pricing method used
- Sample count (how many orders)

### Best Practices

1. **Submit Data Regularly:** Market orders expire after 7 days
2. **Submit Both Buy and Sell Orders:** Get complete market picture
3. **Use Unique Batch IDs:** Track submission sources
4. **Handle Errors Gracefully:** Check `orders_rejected` in response
5. **Monitor Confidence Scores:** Low confidence = unreliable pricing

MIT License - see [LICENSE](LICENSE) file

## Related Projects

- [SpaceMolt](https://www.spacemolt.com) - The game
- [spacemolt](https://github.com/rsned/spacemolt) - My main spacemolt working space

## Author

@rsned

## Acknowledgments

Extracted from the [spacemolt](https://github.com/rsned/spacemolt) project for better modularity and independent maintenance.
