# SpaceMolt Crafting Server - Usage Examples

This document provides practical examples for all MCP tools and HTTP API endpoints, showing real requests and responses.

## Table of Contents

- [MCP Tools](#mcp-tools)
  - [craft_query](#1-craft_query)
  - [craft_path_to](#2-craft_path_to)
  - [recipe_lookup](#3-recipe_lookup)
  - [skill_craft_paths](#4-skill_craft_paths)
  - [component_uses](#5-component_uses)
  - [bill_of_materials](#6-bill_of_materials)
  - [recipe_market_profitability](#7-recipe_market_profitability)
- [HTTP API Endpoints](#http-api-endpoints)
  - [POST /api/v1/market/submit](#post-apiv1marketsubmit)
  - [GET /api/v1/market/price/{item_id}](#get-apiv1marketpriceitem_id)
  - [POST /api/v1/admin/market/recalc/{item_id}](#post-apiv1adminmarketrecalcitem_id)

---

## MCP Tools

### 1. craft_query

**Purpose:** Find what you can craft with your current inventory and skills.

#### Example 1.1: Basic Inventory Query

**Request:**
```json
{
  "name": "craft_query",
  "arguments": {
    "components": [
      {"id": "ore_iron", "quantity": 50},
      {"id": "ore_copper", "quantity": 30}
    ],
    "skills": {"crafting_basic": 1},
    "limit": 3
  }
}
```

**Response:**
```json
{
  "craftable": [
    {
      "recipe": {
        "id": "basic_smelt_iron",
        "name": "Basic Iron Smelting",
        "description": "A crude but effective way to smelt iron ore.",
        "category": "Refining",
        "crafting_time": 3,
        "inputs": [{"item_id": "ore_iron", "quantity": 10}],
        "outputs": [{"item_id": "refined_steel", "quantity": 1}]
      },
      "can_craft_quantity": 5
    },
    {
      "recipe": {
        "id": "basic_copper_processing",
        "name": "Basic Copper Processing",
        "inputs": [{"item_id": "ore_copper", "quantity": 8}],
        "outputs": [{"item_id": "refined_copper_wire", "quantity": 1}]
      },
      "can_craft_quantity": 3
    }
  ],
  "blocked_by_skills": [
    {
      "recipe": {
        "id": "refine_steel",
        "name": "Refine Steel"
      },
      "skills_missing": [
        {
          "skill_id": "refinement",
          "skill_name": "Ore Refinement",
          "current_level": 0,
          "required_level": 1,
          "xp_to_next": 100
        }
      ]
    }
  ],
  "query_stats": {
    "total_recipes_checked": 8,
    "components_provided": 2
  }
}
```

#### Example 1.2: With Profit Maximization Strategy

**Request:**
```json
{
  "name": "craft_query",
  "arguments": {
    "components": [{"id": "ore_iron", "quantity": 100}],
    "skills": {"crafting_basic": 1},
    "strategy": "MAXIMIZE_PROFIT",
    "limit": 5
  }
}
```

**Response:**
```json
{
  "craftable": [
    {
      "recipe": {
        "id": "basic_smelt_iron",
        "name": "Basic Iron Smelting",
        "profit_analysis": {
          "profit_per_unit": 90,
          "total_potential_profit": 900
        }
      },
      "can_craft_quantity": 10
    }
  ],
  "query_stats": {
    "strategy_used": "MAXIMIZE_PROFIT"
  }
}
```

---

### 2. craft_path_to

**Purpose:** Get the complete crafting path for a specific item, showing what materials you need.

#### Example 2.1: Basic Crafting Path

**Request:**
```json
{
  "name": "craft_path_to",
  "arguments": {
    "target_recipe_id": "craft_engine_core",
    "target_quantity": 1,
    "current_inventory": [
      {"id": "comp_steel", "quantity": 2}
    ],
    "skills": {"crafting_basic": 1}
  }
}
```

**Response:**
```json
{
  "target": {
    "recipe_id": "craft_engine_core",
    "recipe_name": "Assemble Engine Core",
    "quantity": 1
  },
  "feasible": false,
  "skill_ready": false,
  "skills_missing": [
    {
      "skill_id": "crafting_advanced",
      "skill_name": "Advanced Crafting",
      "current_level": 0,
      "required_level": 2,
      "xp_to_next": 500
    }
  ],
  "materials_needed": [
    {
      "item_id": "comp_power_cell",
      "quantity_needed": 1,
      "quantity_have": 0,
      "quantity_to_acquire": 1,
      "acquisition_methods": ["craft:craft_power_cell"],
      "is_craftable": true,
      "craft_recipe_id": "craft_power_cell"
    },
    {
      "item_id": "ore_cobalt",
      "quantity_needed": 4,
      "quantity_have": 0,
      "quantity_to_acquire": 4,
      "is_craftable": false
    }
  ],
  "crafting_time": 10,
  "summary": {
    "total_components": 3,
    "components_have": 0,
    "components_to_acquire": 3,
    "components_craftable": 2
  }
}
```

---

### 3. recipe_lookup

**Purpose:** Look up details about a specific recipe, including skill requirements and what it's used for.

#### Example 3.1: Lookup by Recipe ID

**Request:**
```json
{
  "name": "recipe_lookup",
  "arguments": {
    "recipe_id": "craft_engine_core",
    "skills": {"crafting_basic": 1}
  }
}
```

**Response:**
```json
{
  "recipe": {
    "id": "craft_engine_core",
    "name": "Assemble Engine Core",
    "description": "Build propulsion system cores.",
    "category": "Components",
    "crafting_time": 10,
    "base_quality": 40,
    "inputs": [
      {"item_id": "comp_power_cell", "quantity": 1},
      {"item_id": "ore_cobalt", "quantity": 4},
      {"item_id": "refined_alloy", "quantity": 3}
    ],
    "outputs": [
      {"item_id": "comp_engine_core", "quantity": 1}
    ],
    "skills_required": [
      {
        "skill_id": "crafting_advanced",
        "level_required": 2
      }
    ]
  },
  "skill_ready": false,
  "skill_gaps": [
    {
      "skill_id": "crafting_advanced",
      "skill_name": "Advanced Crafting",
      "current_level": 0,
      "required_level": 2,
      "xp_to_next": 500
    }
  ],
  "used_in_recipes": [
    "craft_thruster_assembly",
    "craft_voidborn_phase_drive",
    "craft_afterburner_1",
    "craft_warfare_link_speed",
    "craft_weapon_cannon_2"
  ]
}
```

#### Example 3.2: Search by Name

**Request:**
```json
{
  "name": "recipe_lookup",
  "arguments": {
    "search": "laser",
    "skills": {"crafting_basic": 1}
  }
}
```

**Response:**
```json
{
  "results": [
    {
      "recipe": {
        "id": "craft_laser_diode_1",
        "name": "Basic Laser Diode Assembly",
        "category": "Components"
      }
    },
    {
      "recipe": {
        "id": "craft_void_laser",
        "name": "Build Void Laser",
        "category": "Legendary"
      }
    }
  ]
}
```

---

### 4. skill_craft_paths

**Purpose:** Discover which skills unlock new crafting recipes and see your progression path.

#### Example 4.1: Current Skill Progression

**Request:**
```json
{
  "name": "skill_craft_paths",
  "arguments": {
    "skills": {
      "crafting_basic": {"level": 1, "current_xp": 100},
      "armor": {"level": 2}
    },
    "limit": 3
  }
}
```

**Response:**
```json
{
  "skill_paths": [
    {
      "skill": {
        "id": "crafting_basic",
        "name": "Basic Crafting",
        "description": "Fundamental manufacturing. Craft basic items.",
        "category": "Crafting",
        "max_level": 10
      },
      "current_level": 1,
      "xp_to_next_level": 200,
      "recipes_unlocked_at_next": [
        "craft_power_grid",
        "craft_sensor_cluster",
        "craft_afterburner_1",
        "craft_cargo_container"
      ]
    },
    {
      "skill": {
        "id": "refinement",
        "name": "Ore Refinement",
        "description": "Processing expertise. Increases refining output."
      },
      "current_level": 0,
      "xp_to_next_level": 100,
      "recipes_unlocked_at_next": [
        "refine_water_ice",
        "refine_silver_wire",
        "refine_glass",
        "refine_copper_wire"
      ]
    }
  ],
  "summary": {
    "total_recipes": 394,
    "recipes_unlocked": 47,
    "recipes_locked": 347
  }
}
```

---

### 5. component_uses

**Purpose:** Find all uses for a specific item across all recipes.

#### Example 5.1: Find Component Uses

**Request:**
```json
{
  "name": "component_uses",
  "arguments": {
    "item_id": "comp_steel",
    "skills": {"crafting_basic": 1}
  }
}
```

**Response:**
```json
{
  "item_id": "comp_steel",
  "used_in": [
    {
      "recipe_id": "craft_armor_plate_1",
      "recipe_name": "Basic Armor Plating",
      "category": "Components",
      "quantity_required": 3,
      "can_craft": true
    },
    {
      "recipe_id": "craft_hull_plate",
      "recipe_name": "Hull Plate Fabrication",
      "category": "Components",
      "quantity_required": 5,
      "can_craft": false,
      "blocked_by_skill": "engineering"
    }
  ],
  "total_uses": 12
}
```

---

### 6. bill_of_materials

**Purpose:** Calculate total raw materials needed for a recipe, including intermediate components.

#### Example 6.1: Multi-tier Recipe BOM

**Request:**
```json
{
  "name": "bill_of_materials",
  "arguments": {
    "recipe_id": "craft_engine_core",
    "quantity": 1
  }
}
```

**Response:**
```json
{
  "recipe_id": "craft_engine_core",
  "recipe_name": "Assemble Engine Core",
  "output_item_id": "comp_engine_core",
  "quantity": 1,
  "raw_materials": [
    {"item_id": "ore_cobalt", "quantity": 4},
    {"item_id": "ore_copper", "quantity": 7},
    {"item_id": "ore_crystal", "quantity": 4},
    {"item_id": "ore_iron", "quantity": 6},
    {"item_id": "ore_silicon", "quantity": 2},
    {"item_id": "ore_titanium", "quantity": 9}
  ],
  "intermediates": [
    {
      "item_id": "comp_power_cell",
      "recipe_id": "craft_power_cell",
      "recipe_name": "Build Power Cell",
      "craft_runs": 1,
      "total_produced": 1,
      "total_needed": 1
    },
    {
      "item_id": "refined_alloy",
      "recipe_id": "autoprocess_alloy",
      "recipe_name": "Onboard Alloy Synthesis",
      "craft_runs": 3,
      "total_produced": 3,
      "total_needed": 3
    }
  ],
  "craft_steps": [
    {
      "step_number": 1,
      "recipe_id": "refine_circuits",
      "recipe_name": "Fabricate Circuit Boards",
      "craft_runs": 1,
      "output_item_id": "refined_circuits",
      "output_per_run": 2
    },
    {
      "step_number": 2,
      "recipe_id": "refine_copper_wire",
      "recipe_name": "Process Copper Wiring",
      "craft_runs": 1,
      "output_item_id": "refined_copper_wire",
      "output_per_run": 2
    }
  ],
  "total_craft_time_sec": 21
}
```

---

### 7. recipe_market_profitability ⭐

**Purpose:** Get market profitability for all recipes, sorted by profit.

#### Example 7.1: MSRP-Only Mode (No Station)

**Request:**
```json
{
  "name": "recipe_market_profitability",
  "arguments": {}
}
```

**Response:**
```json
{
  "recipes": [
    {
      "recipe_id": "craft_quantum_entanglement_shield",
      "recipe_name": "Build Quantum Entanglement Shield",
      "category": "Legendary",
      "output_sell_price": 750000,
      "output_msrp": 750000,
      "output_uses_msrp": true,
      "input_cost": 182000,
      "input_uses_msrp": true,
      "profit": 568000,
      "profit_margin_pct": 312.09
    },
    {
      "recipe_id": "craft_void_laser",
      "recipe_name": "Build Void Laser",
      "category": "Legendary",
      "output_sell_price": 500000,
      "output_msrp": 500000,
      "output_uses_msrp": true,
      "input_cost": 239500,
      "input_uses_msrp": true,
      "profit": 260500,
      "profit_margin_pct": 108.77
    }
  ],
  "total_recipes": 394
}
```

#### Example 7.2: With Inventory Support

**Request:**
```json
{
  "name": "recipe_market_profitability",
  "arguments": {
    "components": [
      {"id": "tritanium", "quantity": 1000},
      {"id": "pyerite", "quantity": 500}
    ]
  }
}
```

**Response:**
```json
{
  "recipes": [
    {
      "recipe_id": "craft_heavy_combat_drone",
      "recipe_name": "Build Heavy Combat Drone",
      "category": "Drones",
      "output_sell_price": 12000,
      "output_msrp": 12000,
      "output_uses_msrp": true,
      "input_cost": 0,
      "input_uses_msrp": true,
      "profit": 12000,
      "profit_margin_pct": 0
    }
  ],
  "total_recipes": 394
}
```

**Note:** Input cost is 0 because the user has all required materials in inventory.

#### Example 7.3: Partial Inventory

**Request:**
```json
{
  "name": "recipe_market_profitability",
  "arguments": {
    "components": [
      {"id": "tritanium", "quantity": 100}
    ]
  }
}
```

**Response:**
```json
{
  "recipes": [
    {
      "recipe_id": "craft_capital_armor_plate",
      "recipe_name": "Build Capital Armor Plate",
      "category": "Components",
      "output_sell_price": 15000,
      "input_cost": 7500,
      "profit": 7500,
      "profit_margin_pct": 50.0
    }
  ]
}
```

**Note:** Input cost is reduced because user has some materials (only pays for shortfall).

---

## HTTP API Endpoints

### POST /api/v1/market/submit

**Purpose:** Submit market order data for price calculations.

#### Example 1: Submit Sell Orders

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/market/submit \
  -H "Content-Type: application/json" \
  -d '{
    "station_id": "Jita IV",
    "source": "market_scraper_v1",
    "orders": [
      {
        "item_id": "ore_iron",
        "order_type": "sell",
        "price_per_unit": 30,
        "volume_available": 128700,
        "player_stall_name": "BulkMiner Inc"
      },
      {
        "item_id": "ore_iron",
        "order_type": "sell",
        "price_per_unit": 2,
        "volume_available": 5000
      }
    ],
    "submitted_at": "2025-02-28T10:30:00Z"
  }'
```

**Response:**
```json
{
  "status": "success",
  "batch_id": "batch_20250228_103000_abc123",
  "orders_received": 2,
  "orders_accepted": 2,
  "orders_rejected": 0,
  "recalculated_items": ["ore_iron"],
  "timestamp": "2025-02-28T10:30:01Z"
}
```

#### Example 2: Submit with Invalid Item

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/market/submit \
  -H "Content-Type: application/json" \
  -d '{
    "station_id": "Jita IV",
    "orders": [
      {
        "item_id": "invalid_item_that_does_not_exist",
        "order_type": "sell",
        "price_per_unit": 10,
        "volume_available": 100
      }
    ]
  }'
```

**Response:**
```json
{
  "status": "partial_failure",
  "batch_id": "batch_20250228_103100_def456",
  "orders_received": 1,
  "orders_accepted": 0,
  "orders_rejected": 1,
  "rejections": [
    {
      "item_id": "invalid_item_that_does_not_exist",
      "reason": "item not found in database"
    }
  ]
}
```

---

### GET /api/v1/market/price/{item_id}

**Purpose:** Query current market price for an item.

#### Example 1: Item with Market Data

**Request:**
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
  "method_name": "volume_weighted",
  "sample_count": 22,
  "confidence_score": 0.95,
  "market_status": "high_confidence",
  "total_volume": 134133,
  "price_trend": "stable"
}
```

#### Example 2: Item with No Market Data (MSRP Fallback)

**Request:**
```bash
curl http://localhost:8080/api/v1/market/price/rare_artifact
```

**Response:**
```json
{
  "item_id": "rare_artifact",
  "sell_price": 5000,
  "buy_price": 5000,
  "msrp": 5000,
  "method_name": "msrp_only",
  "sample_count": 0,
  "confidence_score": 0.0,
  "market_status": "no_market_data"
}
```

---

### POST /api/v1/admin/market/recalc/{item_id}

**Purpose:** Manually trigger price recalculation for an item.

#### Example 1: Manual Recalculation

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/admin/market/recalc/comp_steel
```

**Response:**
```json
{
  "status": "success",
  "item_id": "comp_steel",
  "station": "Jita IV",
  "orders_processed": 45,
  "previous_price": 120,
  "new_price": 125,
  "previous_method": "second_price",
  "new_method": "volume_weighted",
  "recalculated_at": "2025-02-28T10:35:00Z"
}
```

---

## Quick Reference

### MCP Tools Quick Reference

| Tool | Best For | Key Parameters |
|------|----------|----------------|
| `craft_query` | What can I make now? | components, skills, limit |
| `craft_path_to` | How do I make X? | target_recipe_id, current_inventory |
| `recipe_lookup` | Tell me about recipe X | recipe_id or search |
| `skill_craft_paths` | What should I train? | skills (with levels) |
| `component_uses` | What can I do with X? | item_id |
| `bill_of_materials` | What do I need to make X? | recipe_id, quantity |
| `recipe_market_profitability` | What's most profitable? | station_id, components |

### HTTP API Quick Reference

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v1/market/submit` | POST | Submit market orders |
| `/api/v1/market/price/{item_id}` | GET | Query item prices |
| `/api/v1/admin/market/recalc/{item_id}` | POST | Recalculate prices |

---

## Common Patterns

### Pattern 1: Find Profitable Items with Current Inventory

```json
{
  "name": "recipe_market_profitability",
  "arguments": {
    "components": [
      {"id": "tritanium", "quantity": 1000},
      {"id": "pyerite", "quantity": 500}
    ]
  }
}
```

### Pattern 2: Plan Multi-Step Crafting

```json
{
  "name": "craft_path_to",
  "arguments": {
    "target_recipe_id": "craft_capital_ship",
    "target_quantity": 1,
    "skills": {"capital_construction": 5}
  }
}
```

### Pattern 3: Discover Skill Training Paths

```json
{
  "name": "skill_craft_paths",
  "arguments": {
    "skills": {
      "crafting_basic": {"level": 1}
    }
  }
}
```

---

For more information, see the main [README.md](README.md).
