# SpaceMolt Crafting Query MCP Server

> **Compatible with SpaceMolt gameserver v0.104.1**
> Last updated: 2026-02-19

An MCP (Model Context Protocol) server that provides intelligent crafting queries for SpaceMolt AI agents. Enables agents to efficiently discover what they can craft, plan crafting paths, and optimize skill progression without complex prompt engineering.

## Features

### 6 Powerful MCP Tools

1. **`craft_query`** - "What can I craft with my inventory?"
   - Returns fully craftable recipes, partial matches, and skill-blocked recipes
   - Supports multiple optimization strategies (profit, volume, inventory usage)
   - Fast component matching with inverted index

2. **`craft_path_to`** - "How do I craft this specific item?"
   - Backward chaining from target recipe
   - Shows material requirements with acquisition methods
   - Single-level expansion (agents control planning depth)

3. **`recipe_lookup`** - "Tell me about this recipe"
   - Direct lookup by ID or search by name
   - Skill gap analysis
   - Shows what recipes use this output (crafting chains)

4. **`skill_craft_paths`** - "Which skills unlock new recipes?"
   - Identifies high-value skill progression paths
   - Shows recipes unlocked at next level
   - Sorted by number of recipes unlocked

5. **`component_uses`** - "What can I do with this component?"
   - Find all recipes using a specific component
   - Useful when acquiring new materials
   - Supports profit optimization

6. **`bill_of_materials`** - "What raw materials do I need to craft this?"
   - Complete recursive dependency resolution (multi-level)
   - Returns total raw materials, intermediate items, and craft steps
   - Accounts for output quantities (e.g., recipes producing 2+ per craft)
   - **Deterministic recipe selection** when multiple recipes produce the same item:
     - Prefers shortest craft time
     - Then highest output quantity (better efficiency)
     - Then lexicographically first recipe_id (for consistency)
   - **Consistent diamond dependencies**: Same intermediate used on multiple paths always uses the same recipe throughout the tree

### Optimization Strategies

All query tools support strategic result sorting:
- `MAXIMIZE_PROFIT` - Sort by profit margin (requires market data)
- `MAXIMIZE_VOLUME` - Prefer recipes you can craft many times
- `OPTIMIZE_CRAFT_PATH` - Prefer simpler recipes
- `USE_INVENTORY_FIRST` - Minimize new acquisitions (default)
- `MINIMIZE_ACQUISITION` - Prefer recipes needing fewest missing components

### Deterministic Recipe Selection (Bill of Materials)

When multiple recipes produce the same output item (e.g., 4 different recipes produce `refined_circuits`), the `bill_of_materials` tool uses deterministic selection:

**Selection Criteria (in priority order):**
1. **Shortest craft time** - Faster crafting is preferred
2. **Highest output quantity** - More efficient for bulk production
3. **Lexicographically first recipe_id** - Consistent tie-breaker

**Diamond Dependency Consistency:**
When an intermediate item appears in multiple places in the dependency tree (e.g., `refined_crystal` needed by both the target recipe and a sub-component), the tool **always uses the same recipe** throughout the entire tree. This ensures:
- Predictable raw material totals
- No mixing of recipe variants within a single BOM
- Consistent crafting plans

**Example:** If `refined_circuits` is selected via `refine_circuits` recipe, all instances of `refined_circuits` in the dependency tree will use `refine_circuits`, not alternative recipes like `craft_fluorine_circuits` or `refine_circuits_silver`.

## Installation

The crafting server is part of the spacemolt-agent-server project:

```bash
# Build the server
go build -o bin/crafting-server ./cmd/crafting-server

# Build the data converters
go build -o bin/convert-recipes ./cmd/convert-recipes
go build -o bin/convert-skills ./cmd/convert-skills
```

## Data Import

Before using the server, import recipe and skill data:

### 1. Convert SpaceMolt Data Format

```bash
# Convert recipes
./bin/convert-recipes server_docs/recipes.20260216.json data/crafting/recipes-import.json

# Convert skills
./bin/convert-skills server_docs/skills.20260216.json data/crafting/skills-import.json
```

### 2. Import into Database

```bash
# Import recipes (239 recipes as of v0.87.1)
./bin/crafting-server -import-recipes data/crafting/recipes-import.json

# Import skills (139 skills)
./bin/crafting-server -import-skills data/crafting/skills-import.json

# Optional: Import market data for profit calculations
./bin/crafting-server -import-market data/crafting/market.json
```

## Usage

### As an MCP Server (for Claude Desktop/API)

Run the server to communicate via stdin/stdout JSON-RPC:

```bash
./bin/crafting-server -db data/crafting/crafting.db
```

### MCP Client Configuration

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "spacemolt-crafting": {
      "command": "/full/path/to/bin/crafting-server",
      "args": ["-db", "/full/path/to/data/crafting/crafting.db"]
    }
  }
}
```

### Command-Line Options

```
-db string
    Path to SQLite database (default "data/crafting/crafting.db")
-import-recipes string
    Import recipes from JSON file
-import-skills string
    Import skills from JSON file
-import-market string
    Import market data from JSON file
-verbose
    Enable verbose logging
```

## Example Queries

### Query Craftable Recipes

```json
{
  "method": "tools/call",
  "params": {
    "name": "craft_query",
    "arguments": {
      "components": [
        {"id": "ore_copper", "quantity": 50},
        {"id": "ore_iron", "quantity": 30}
      ],
      "skills": {
        "crafting_basic": 1,
        "mining_basic": 2
      },
      "limit": 10
    }
  }
}
```

**Response:**

```json
{
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "## Fully Craftable Recipes (3)\n\n### craft_copper_wire\n- **Output:** refined_circuits x2\n- **Time:** 5s\n- **Inputs:** ore_copper x10, ore_iron x5\n- **Skills:** crafting_basic:1\n- **Profit:** 15 credits (market: 25, cost: 10)\n\n### craft_iron_plate\n- **Output:** iron_plate x1\n- **Time:** 8s\n- **Inputs:** ore_iron x15\n- **Skills:** crafting_basic:1\n- **Profit:** 8 credits (market: 23, cost: 15)\n\n## Partial Matches (2)\n\n### craft_basic_mining_laser\n- **Missing:** crystal_lens x1\n- **Have:** ore_copper (10/20), ore_iron (5/15)\n- **Skill Gap:** mining_basic:2\n\n## Skill-Blocked (5)\n\n### craft_advanced_sensor\n- **Skill Gap:** crafting_advanced:3 (have: crafting_basic:1)\n- **Have:** All components\n"
      }
    ],
    "isError": false
  }
}
```

Returns craftable recipes, partial matches, and skill-blocked options.

### Plan Crafting Path

```json
{
  "method": "tools/call",
  "params": {
    "name": "craft_path_to",
    "arguments": {
      "target_recipe_id": "craft_sensor_array",
      "current_inventory": [
        {"id": "refined_circuits", "quantity": 5}
      ],
      "skills": {
        "crafting_basic": 4,
        "scanning": 2
      }
    }
  }
}
```

**Response:**

```json
{
  "id": 4,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "## Crafting Path: sensor_array\n\n### Target Recipe\n- **Output:** sensor_array x1\n- **Time:** 15s\n- **Skills Required:** crafting_basic:4, scanning:2 ✓\n\n### Materials Needed\n| Component | Required | Have | Acquire | Source |\n|-----------|----------|------|---------|--------|\n| refined_circuits | 10 | 5 | 5 | craft_refine_circuits (5s) |\n| crystal_lens | 2 | 0 | 2 | craft_grind_lens (8s) |\n| housing_unit | 1 | 0 | 1 | Market: 45 credits |\n\n### Total Craft Time: 23s\n### Total Market Cost: 45 credits\n"
      }
    ],
    "isError": false
  }
}
```

Shows materials needed, what you have, what to acquire, and crafting time.

### Find Skill Progression Paths

```json
{
  "method": "tools/call",
  "params": {
    "name": "skill_craft_paths",
    "arguments": {
      "skills": {
        "crafting_basic": {"level": 1, "current_xp": 50},
        "mining_basic": {"level": 2, "current_xp": 100}
      },
      "limit": 5
    }
  }
}
```

**Response:**

```json
{
  "id": 5,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "## Skill Progression Paths\n\n### crafting_basic → Level 2 (XP needed: 50)\n**Unlocks 12 new recipes:**\n- craft_advanced_mining_laser (profit: 45 credits)\n- craft_precision_drill (profit: 38 credits)\n- craft_industrial_smelter (profit: 120 credits)\n- ...and 9 more\n\n### mining_basic → Level 3 (XP needed: 200)\n**Unlocks 8 new recipes:**\n- craft_deep_core_drill (profit: 85 credits)\n- craft_automated_miner (profit: 150 credits)\n- craft_ore_processor (profit: 95 credits)\n- ...and 5 more\n\n### Top Recommendations:\n1. **crafting_basic** - Best ROI: 12 recipes for 50 XP\n2. **mining_basic** - High value: 8 recipes for 200 XP\n"
      }
    ],
    "isError": false
  }
}
```

Lists skills sorted by recipes unlocked at next level, with XP needed.

### Calculate Full Bill of Materials

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

**Response:**

```json
{
  "id": 6,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "## Bill of Materials: scanner_1 (x1)\n\n### Raw Materials\n- ore_copper: 9\n- ore_silicon: 6\n- ore_crystal: 11\n- ore_palladium: 4\n\n### Intermediate Components\n| Item | Quantity | Craft Runs | Recipe |\n|------|----------|------------|--------|\n| refined_circuits | 3 | 1 | refine_circuits (5s) |\n| crystal_lens | 2 | 1 | grind_lens (8s) |\n| sensor_unit | 1 | 1 | craft_sensor_unit (10s) |\n\n### Craft Steps (Ordered)\n1. Craft refined_circuits (5s) - need: ore_copper:6, ore_silicon:3\n2. Craft crystal_lens (8s) - need: ore_crystal:8, ore_palladium:2\n3. Craft sensor_unit (10s) - need: refined_circuits:1, crystal_lens:1, ore_copper:3\n4. Craft scanner_1 (12s) - need: sensor_unit:1, refined_circuits:2, ore_crystal:3\n\n### Summary\n- **Total Raw Materials:** 30 units across 4 types\n- **Total Craft Time:** 35 seconds\n- **Craft Steps:** 4 steps\n- **Recipe Selection:** refine_circuits (shortest time: 5s)\n"
      }
    ],
    "isError": false
  }
}
```

Returns complete breakdown:
- `raw_materials` - Total ore/gas needed
- `intermediates` - All intermediate items with craft runs and quantities
- `craft_steps` - Ordered steps from raw materials to final product (deepest dependencies first)
- `total_craft_time_sec` - Sum of all crafting time

**Recipe Selection:** When multiple recipes produce the same output (e.g., 4 recipes for `refined_circuits`), the tool deterministically selects based on:
1. Shortest craft time (faster is better)
2. Highest output quantity (more efficient)
3. Recipe ID alphabetically (consistent tie-breaker)

Once selected, the same recipe is used throughout the entire dependency tree for consistency.

### Recipe Lookup

```json
{
  "method": "tools/call",
  "params": {
    "name": "recipe_lookup",
    "arguments": {
      "query": "sensor_array"
    }
  }
}
```

**Response:**

```json
{
  "id": 7,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "## Recipe: craft_scanner_1\n\n### Output\n- **Item:** scanner_1\n- **Quantity:** 1\n- **Market Price:** 150 credits\n\n### Inputs\n| Component | Quantity |\n|-----------|----------|\n| sensor_unit | 1 |\n| refined_circuits | 2 |\n| ore_crystal | 3 |\n\n### Requirements\n- **Time:** 12 seconds\n- **Skills:** crafting_basic:4, scanning:2\n\n### Used By (2 recipes)\n- craft_advanced_scanner (profit: 85 credits)\n- craft_deep_space_scanner (profit: 120 credits)\n\n### Profitability\n- **Input Cost:** 65 credits\n- **Output Value:** 150 credits\n- **Profit Margin:** 85 credits (56%)\n"
      }
    ],
    "isError": false
  }
}
```

Direct lookup by ID or search by name, with skill gap analysis and reverse lookup.

### Component Uses

```json
{
  "method": "tools/call",
  "params": {
    "name": "component_uses",
    "arguments": {
      "component_id": "refined_circuits",
      "limit": 10
    }
  }
}
```

**Response:**

```json
{
  "id": 8,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "## Uses for: refined_circuits\n\n### Found 12 recipes using this component\n\n### Direct Uses (Highest Profit First)\n| Recipe | Uses | Profit | Time |\n|--------|-------|--------|------|\n| craft_advanced_scanner | 3 | 85 | 18s |\n| craft_sensor_array | 2 | 45 | 12s |\n| craft_computer_unit | 1 | 35 | 10s |\n| craft_power_core | 2 | 55 | 14s |\n| craft_shield_generator | 4 | 95 | 22s |\n\n### Top 5 by Profit\n1. **craft_shield_generator** - 95 profit, uses 4x\n2. **craft_advanced_scanner** - 85 profit, uses 3x\n3. **craft_power_core** - 55 profit, uses 2x\n4. **craft_sensor_array** - 45 profit, uses 2x\n5. **craft_computer_unit** - 35 profit, uses 1x\n"
      }
    ],
    "isError": false
  }
}
```

Find all recipes using a specific component, useful when acquiring new materials.

## Architecture

```
cmd/crafting-server/
├── main.go                    # Entry point and CLI handling

pkg/crafting/
└── types.go                   # Public domain types (340 lines)

internal/crafting/
├── db/                        # Database layer
│   ├── db.go                  # Core DB wrapper
│   ├── schema.go              # Schema initialization
│   ├── recipes.go             # Recipe queries
│   ├── skills.go              # Skill queries
│   └── market.go              # Market data queries
├── engine/                    # Query business logic
│   ├── engine.go              # Main engine
│   ├── craft_query.go         # Component matching
│   ├── craft_path.go          # Path planning (single-level)
│   ├── bill_of_materials.go   # Recursive BOM with deterministic recipe selection
│   ├── recipe_lookup.go       # Recipe search
│   ├── skill_paths.go         # Skill analysis
│   └── component_uses.go      # Reverse lookup
├── mcp/                       # MCP protocol
│   ├── server.go              # JSON-RPC server
│   └── tools.go               # Tool definitions
└── sync/                      # Data import
    └── sync.go                # Import from JSON
```

## Database Schema

SQLite database with the following tables:

- **recipes** - Recipe metadata
- **recipe_components** - Required inputs (inverted index on component_id)
- **recipe_skills** - Skill requirements
- **skills** - Skill definitions
- **skill_prerequisites** - Skill dependencies
- **skill_levels** - XP thresholds per level
- **market_prices** - Historical price data
- **market_price_summary** - Aggregated price stats
- **sync_metadata** - Import tracking

## Performance

- **Query Speed:** 1-5ms for typical craft_query (6-20 recipes checked)
- **Database Size:** ~500KB (239 recipes, 139 skills)
- **Binary Size:** 10MB (includes all dependencies)
- **Indexing:** Inverted component index for O(log n) lookups

## Data Sources

The server imports data from SpaceMolt game API snapshots:

- **Recipes:** `server_docs/recipes.20260216.json` (289 total, 239 imported)
- **Skills:** `server_docs/skills.20260216.json` (139 skills)
- **Market:** Agent-collected price data (optional)

**Note:** Not all recipes from the game API may be imported - some may be filtered or pending implementation.

## Testing

Test the server with sample queries:

```bash
# Initialize
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./bin/crafting-server -db data/crafting/crafting.db 2>/dev/null

# List tools
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | ./bin/crafting-server -db data/crafting/crafting.db 2>/dev/null

# Test craft_query
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"craft_query","arguments":{"components":[{"id":"ore_copper","quantity":50}],"skills":{"crafting_basic":1},"limit":5}}}' | ./bin/crafting-server -db data/crafting/crafting.db 2>/dev/null

# Test bill_of_materials (with pretty output)
cat <<'EOF' | ./bin/crafting-server -db data/crafting/crafting.db 2>/dev/null | jq -r 'select(.id == 2) | .result.content[0].text' | jq .
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"bill_of_materials","arguments":{"recipe_id":"craft_scanner_1","quantity":1}}}
EOF
```

## Design Principles

1. **Stateless** - No per-agent state stored
2. **All recipes visible** - Agents see all game content
3. **Single-level expansion** - Agents control planning depth
4. **Fast lookups** - Inverted indexes for component matching
5. **Market-aware** - Optional profit optimization with real prices

## Scope

### In Scope
- Recipe querying and matching
- Component gap analysis
- Skill requirement checking
- Optimization strategies
- Market-based profit calculations

### Out of Scope (Agent Layer)
- Multi-agent coordination
- Inventory synchronization with game
- Crafting execution (calling game API)
- Goal prioritization
- Per-agent state persistence

## Related Documentation

- [Design Specification](../../docs/spacemolt-crafting-server-spec-final.md)
- [SpaceMolt Agent Guide](../../docs/SPACEMOLT_AGENT_GUIDE.md)
- [MCP Protocol](https://modelcontextprotocol.io)

## Status

✅ **Production Ready**
- All 6 tools implemented and tested
- 239 recipes imported (as of gameserver v0.87.1)
- 139 skills imported
- Query performance: 1-5ms (craft_query), 5-15ms (bill_of_materials)
- Deterministic recipe selection for multi-level BOM
- Full MCP protocol compliance

## License

Part of the spacemolt-agent-server project.
