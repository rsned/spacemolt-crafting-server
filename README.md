# SpaceMolt Crafting Query MCP Server

A Model Context Protocol (MCP) server that provides intelligent crafting queries for SpaceMolt AI agents to cut down on context usage and token burn.

## Features

### 6 Useful MCP Tools

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

#### Database Snapshot

A pre-built database snapshot is available in the `database/` directory, containing all recipes and skills already imported. You can use it directly:

```bash
# Copy the pre-built database
cp database/crafting.db ./

# Or run the server with the snapshot directly
./bin/crafting-server -db database/crafting.db
```

#### Import Recipe Data Manually

If you prefer to build your own database from scratch, you can import the data manually:

```bash
# Import recipes from SpaceMolt game API
./bin/crafting-server -db crafting.db -import-recipes recipes.json

# Import skill definitions
./bin/crafting-server -db crafting.db -import-skills skills.json

# (Optional) Import market data for profit calculations
./bin/crafting-server -db crafting.db -import-market market.json
```

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

### Available Tools

Once configured, you can use these tools in Claude Code:

- **`craft_query`** - Find what you can craft with your current inventory and skills
- **`craft_path_to`** - Get the crafting path for a specific item
- **`recipe_lookup`** - Look up details about a specific recipe
- **`skill_craft_paths`** - Discover which skills unlock new crafting recipes
- **`component_uses`** - Find all uses for a specific component
- **`bill_of_materials`** - Calculate total raw materials needed for a recipe

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

## Related Projects

- [SpaceMolt](https://www.spacemolt.com) - The game
- [spacemolt](https://github.com/rsned/spacemolt) - My main spacemolt working space

## Author

@rsned

## Acknowledgments

Extracted from the [spacemolt](https://github.com/rsned/spacemolt) project for better modularity and independent maintenance.
