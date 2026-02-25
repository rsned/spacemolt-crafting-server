# Crafting Server Test Tools

A comprehensive testing tool for the SpaceMolt Crafting Server MCP tools.

## Overview

This tool tests all 6 MCP tools exposed by the crafting server:
- `craft_query` - What can I craft with my inventory?
- `craft_path_to` - How do I craft this specific item?
- `recipe_lookup` - Tell me about this recipe
- `skill_craft_paths` - Which skills unlock new recipes?
- `component_uses` - What can I do with this item?
- `bill_of_materials` - What raw materials do I need?

## Test Categories

Each tool is tested with three categories of tests:

### 1. Invalid/Non-existent Inputs
Tests that verify proper error handling for:
- Non-existent recipe/item IDs (e.g., "chicken_pot_pie", "dragon_scales")
- Empty or malformed parameters
- Negative quantities
- Invalid skill IDs

**Goal**: Ensure bad inputs don't crash the server and return appropriate errors.

### 2. Simple Queries
Basic functional tests with valid inputs:
- Single component queries (e.g., "ore_iron: 10")
- Direct recipe lookups by ID
- Simple search operations
- Single-level material requirements

**Goal**: Verify the basic happy path works correctly.

### 3. Complex Queries
Advanced tests covering edge cases and deeper functionality:
- Multi-tier recipes (e.g., "quantum_matrix" with 3+ levels of dependencies)
- All 5 optimization strategies (MAXIMIZE_PROFIT, MAXIMIZE_VOLUME, etc.)
- Category filtering
- Large quantity calculations
- Skill gap analysis
- Recursive bill of materials

**Goal**: Ensure deep results and edge cases don't cause problems.

## Usage

### Quick Start

```bash
# Build the test tool
go build -o bin/test-tools ./cmd/test-tools

# Run tests with default database (elided output)
./bin/test-tools

# Run tests with full output (no eliding)
./bin/test-tools -v

# Run tests with custom database
CRAFTING_DB=/path/to/crafting.db ./bin/test-tools
```

### Database Location

The tool searches for the database in this order:
1. `$CRAFTING_DB` environment variable (if set)
2. `database/crafting.db` (default pre-built location)
3. `data/crafting/crafting.db` (runtime default)

## Output Format

The tool provides:
- ✓ PASS or ✗ FAIL status for each test
- Test category: [invalid], [simple], or [complex]
- Execution time per test
- JSON preview of results for simple/complex tests
- Detailed summary with pass/fail counts
- Timing statistics

### Command-Line Flags

- `-v` - Show full results instead of eliding them. By default, JSON output is truncated to 10 lines for readability. With `-v`, the complete JSON response is shown.

Example output (default - elided):
```
  ✓ PASS [invalid] craft_query with non-existent items (0s)
  ✓ PASS [simple] craft_query with ore_iron: 10 (1ms)
  ✓ PASS [complex] craft_query with strategy: MAXIMIZE_PROFIT (2ms)
```

## Test Coverage

The tool runs **43 tests** across all 6 MCP tools:

| Tool          | Invalid | Simple | Complex | Total |
|---------------|---------|--------|---------|-------|
| craft_query   | 3       | 2      | 6       | 11    |
| craft_path_to | 2       | 2      | 2       | 6     |
| recipe_lookup | 2       | 2      | 2       | 6     |
| skill_craft_paths | 2    | 2      | 2       | 6     |
| component_uses | 2      | 2      | 3       | 7     |
| bill_of_materials | 2    | 2      | 3       | 7     |

## Benefits

1. **Regression Testing**: Quickly verify changes don't break existing functionality
2. **Schema Validation**: Catches database schema mismatches (as seen in the migration from v1 to v2)
3. **Error Handling**: Ensures graceful handling of invalid inputs
4. **Performance Monitoring**: Track query execution times
5. **Documentation**: Serves as executable documentation of expected behavior

## Exit Codes

- `0` - All tests passed
- `1` - One or more tests failed

## Integration with CI/CD

Add to your CI pipeline:

```yaml
# Example GitHub Actions step
- name: Run crafting server tests
  run: |
    go build -o bin/test-tools ./cmd/test-tools
    CRAFTING_DB=database/crafting.db ./bin/test-tools
```

## Troubleshooting

### Database Schema Mismatch

If you see errors like "no such column: crafting_time", your database needs migration:

```bash
sqlite3 database/crafting.db < scripts/migrate-v1-to-v2.sql
```

### Missing Database

Ensure you've imported data:

```bash
crafting-server -import-items catalog_items.json
crafting-server -import-recipes catalog_recipes.json
crafting-server -import-skills catalog_skills.json
```

## Maintenance

When adding new MCP tools:
1. Add a test function `testNewTool(ctx, eng, log)`
2. Add test cases in the three categories (invalid, simple, complex)
3. Call the function from `runAllTests()`
4. Update this README with the new tool's coverage
