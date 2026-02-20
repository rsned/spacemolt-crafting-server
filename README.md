# SpaceMolt Crafting Query MCP Server

A Model Context Protocol (MCP) server that provides intelligent crafting queries for SpaceMolt AI agents.

## Features

### 6 Powerful MCP Tools

1. **`craft_query`** - "What can I craft with my inventory?"
2. **`craft_path_to`** - "How do I craft this specific item?"
3. **`recipe_lookup`** - "Tell me about this recipe"
4. **`skill_craft_paths`** - "Which skills unlock new recipes?"
5. **`component_uses`** - "What can I do with this component?"
6. **`bill_of_materials`** - "What raw materials do I need?"

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

#### As an MCP Server

```bash
# Run the server (communicates via stdin/stdout)
./bin/crafting-server -db crafting.db
```

#### Import Recipe Data

```bash
# Import recipes from SpaceMolt game API
./bin/crafting-server -db crafting.db -import-recipes recipes.json

# Import skill definitions
./bin/crafting-server -db crafting.db -import-skills skills.json

# (Optional) Import market data for profit calculations
./bin/crafting-server -db crafting.db -import-market market.json
```

## Database

The server uses SQLite for fast, efficient recipe and skill queries:

- **Recipes:** 239+ recipes from SpaceMolt
- **Skills:** 139 skill definitions
- **Database Size:** ~500KB
- **Query Performance:** 1-5ms typical

### Schema

- `recipes` - Recipe metadata
- `recipe_components` - Required inputs (inverted index)
- `recipe_skills` - Skill requirements
- `skills` - Skill definitions
- `skill_prerequisites` - Skill dependencies
- `skill_levels` - XP thresholds per level
- `market_prices` - Historical price data

## Example Queries

### What Can I Craft?

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

## Architecture

```
cmd/crafting-server/    # Main entry point
pkg/crafting/           # Public domain types
internal/
  ├── db/              # Database layer
  ├── engine/          # Query business logic
  ├── mcp/             # MCP protocol
  └── sync/            # Data import
```

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
-import-recipes string
    Import recipes from JSON file
-import-skills string
    Import skills from JSON file
-import-market string
    Import market data from JSON file
-verbose
    Enable verbose logging
```

## Data Format

### Recipe JSON

```json
[
  {
    "id": "craft_basic_mining_laser",
    "name": "Basic Mining Laser",
    "description": "A simple mining laser for asteroid extraction.",
    "category": "Mining",
    "craft_time_sec": 10,
    "components": [
      {"id": "ore_copper", "quantity": 5},
      {"id": "ore_crystal", "quantity": 2}
    ],
    "skills_required": [
      {"skill_id": "crafting_basic", "level": 1}
    ],
    "output": {
      "item_id": "mining_laser_1",
      "quantity": 1
    }
  }
]
```

### Skill JSON

```json
[
  {
    "id": "crafting_basic",
    "name": "Basic Crafting",
    "description": "Foundation of crafting knowledge.",
    "category": "Crafting",
    "levels": [
      {"level": 1, "xp_required": 0},
      {"level": 2, "xp_required": 100},
      {"level": 3, "xp_required": 300}
    ]
  }
]
```

## Performance

- **Query Speed:** 1-5ms for typical queries
- **Database Size:** ~500KB (239 recipes, 139 skills)
- **Binary Size:** ~10MB
- **Memory Usage:** ~5MB typical

## License

MIT License - see [LICENSE](LICENSE) file

## Contributing

Contributions welcome! Please feel free to submit a Pull Request.

## Related Projects

- [SpaceMolt](https://www.spacemolt.com) - The game
- [spacemolt-agent-server](https://github.com/rsned/spacemolt-agent-server) - Main agent framework

## Author

Robert Sneddon

## Acknowledgments

Extracted from the [spacemolt-agent-server](https://github.com/rsned/spacemolt-agent-server) project for better modularity and independent maintenance.
