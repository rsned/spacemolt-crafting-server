package mcp

import (
	"context"
	"encoding/json"

	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// ToolDefinition describes an MCP tool.
type ToolDefinition struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema JSONSchema `json:"inputSchema"`
}

// JSONSchema is a simplified JSON Schema representation.
type JSONSchema struct {
	Type       string                `json:"type"`
	Properties map[string]Property   `json:"properties,omitempty"`
	Required   []string              `json:"required,omitempty"`
}

// Property describes a schema property.
type Property struct {
	Type        string              `json:"type,omitempty"`
	Description string              `json:"description,omitempty"`
	Default     any                 `json:"default,omitempty"`
	Enum        []string            `json:"enum,omitempty"`
	Minimum     *float64            `json:"minimum,omitempty"`
	Maximum     *float64            `json:"maximum,omitempty"`
	Items       *Property           `json:"items,omitempty"`
	Properties  map[string]Property `json:"properties,omitempty"`
	Required    []string            `json:"required,omitempty"`
	AdditionalProperties *Property  `json:"additionalProperties,omitempty"`
}

// GetToolDefinitions returns all tool definitions.
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		craftQueryTool(),
		craftPathToTool(),
		recipeLookupTool(),
		skillCraftPathsTool(),
		componentUsesTool(),
		billOfMaterialsTool(),
	}
}

func craftQueryTool() ToolDefinition {
	minMatch := 0.0
	maxMatch := 1.0
	minLimit := 1.0
	maxLimit := 100.0
	
	return ToolDefinition{
		Name:        "craft_query",
		Description: "Query what recipes can be crafted with given components and skills. Returns fully craftable recipes, partial matches, and skill-blocked recipes.",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]Property{
				"components": {
					Type:        "array",
					Description: "Components the agent currently has",
					Items: &Property{
						Type: "object",
						Properties: map[string]Property{
							"id":       {Type: "string", Description: "Component ID"},
							"quantity": {Type: "integer", Description: "Quantity available"},
						},
						Required: []string{"id", "quantity"},
					},
				},
				"skills": {
					Type:        "object",
					Description: "Agent's current skill levels (skill_id -> level)",
					AdditionalProperties: &Property{Type: "integer"},
				},
				"include_partial": {
					Type:        "boolean",
					Description: "Include recipes where agent has some but not all components",
					Default:     true,
				},
				"min_match_ratio": {
					Type:        "number",
					Description: "Minimum component match ratio for partial results (0.0-1.0)",
					Default:     0.25,
					Minimum:     &minMatch,
					Maximum:     &maxMatch,
				},
				"optimization_strategy": {
					Type:        "string",
					Description: "How to sort/optimize results",
					Enum:        []string{"MAXIMIZE_PROFIT", "MAXIMIZE_VOLUME", "OPTIMIZE_CRAFT_PATH", "USE_INVENTORY_FIRST", "MINIMIZE_ACQUISITION"},
					Default:     "USE_INVENTORY_FIRST",
				},
				"station_id": {
					Type:        "string",
					Description: "Station ID for market price lookups (required for MAXIMIZE_PROFIT)",
				},
				"category_filter": {
					Type:        "string",
					Description: "Filter to specific recipe category",
				},
				"limit": {
					Type:        "integer",
					Description: "Max results per section",
					Default:     20,
					Minimum:     &minLimit,
					Maximum:     &maxLimit,
				},
			},
			Required: []string{"components", "skills"},
		},
	}
}

func craftPathToTool() ToolDefinition {
	minQty := 1.0
	
	return ToolDefinition{
		Name:        "craft_path_to",
		Description: "Calculate what materials are needed to craft a specific recipe. Returns single-level component expansion with acquisition methods.",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]Property{
				"target_recipe_id": {
					Type:        "string",
					Description: "Recipe ID to craft",
				},
				"target_quantity": {
					Type:        "integer",
					Description: "How many to craft",
					Default:     1,
					Minimum:     &minQty,
				},
				"current_inventory": {
					Type:        "array",
					Description: "Components agent currently has",
					Items: &Property{
						Type: "object",
						Properties: map[string]Property{
							"id":       {Type: "string"},
							"quantity": {Type: "integer"},
						},
						Required: []string{"id", "quantity"},
					},
				},
				"skills": {
					Type:        "object",
					Description: "Agent's current skill levels",
					AdditionalProperties: &Property{Type: "integer"},
				},
				"station_id": {
					Type:        "string",
					Description: "Station ID for acquisition method lookups",
				},
			},
			Required: []string{"target_recipe_id", "skills"},
		},
	}
}

func recipeLookupTool() ToolDefinition {
	return ToolDefinition{
		Name:        "recipe_lookup",
		Description: "Look up details for a specific recipe by ID or search term. Returns recipe details, skill gaps, profit analysis, and what recipes use the output.",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]Property{
				"recipe_id": {
					Type:        "string",
					Description: "Exact recipe ID to look up",
				},
				"search": {
					Type:        "string",
					Description: "Search term for recipe name (alternative to recipe_id)",
				},
				"skills": {
					Type:        "object",
					Description: "Agent's skills for gap analysis",
					AdditionalProperties: &Property{Type: "integer"},
				},
				"station_id": {
					Type:        "string",
					Description: "Station for market data",
				},
			},
		},
	}
}

func skillCraftPathsTool() ToolDefinition {
	minLimit := 1.0
	
	return ToolDefinition{
		Name:        "skill_craft_paths",
		Description: "Find which skills would unlock new crafting recipes if leveled. Returns skills sorted by recipes unlocked at next level.",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]Property{
				"skills": {
					Type:        "object",
					Description: "Agent's current skill levels with optional XP",
					AdditionalProperties: &Property{
						Type: "object",
						Properties: map[string]Property{
							"level":      {Type: "integer"},
							"current_xp": {Type: "integer"},
						},
						Required: []string{"level"},
					},
				},
				"category_filter": {
					Type:        "string",
					Description: "Filter to skills in specific category",
				},
				"limit": {
					Type:        "integer",
					Description: "Max skills to return",
					Default:     10,
					Minimum:     &minLimit,
				},
			},
			Required: []string{"skills"},
		},
	}
}

func componentUsesTool() ToolDefinition {
	return ToolDefinition{
		Name:        "component_uses",
		Description: "Find all recipes that use a specific component. Useful when acquiring a new item to see crafting options.",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]Property{
				"component_id": {
					Type:        "string",
					Description: "Component to look up uses for",
				},
				"skills": {
					Type:        "object",
					Description: "Agent's skills for filtering",
					AdditionalProperties: &Property{Type: "integer"},
				},
				"include_skill_locked": {
					Type:        "boolean",
					Description: "Include recipes agent can't craft yet",
					Default:     true,
				},
				"station_id": {
					Type:        "string",
					Description: "Station for market data",
				},
				"optimization_strategy": {
					Type:        "string",
					Description: "How to sort results",
					Enum:        []string{"MAXIMIZE_PROFIT", "MAXIMIZE_VOLUME", "USE_INVENTORY_FIRST"},
					Default:     "USE_INVENTORY_FIRST",
				},
			},
			Required: []string{"component_id"},
		},
	}
}

// Tool handlers

func (s *Server) toolCraftQuery(ctx context.Context, args json.RawMessage) (any, error) {
	var req crafting.CraftQueryRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}
	return s.engine.CraftQuery(ctx, req)
}

func (s *Server) toolCraftPathTo(ctx context.Context, args json.RawMessage) (any, error) {
	var req crafting.CraftPathRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}
	return s.engine.CraftPathTo(ctx, req)
}

func (s *Server) toolRecipeLookup(ctx context.Context, args json.RawMessage) (any, error) {
	var req crafting.RecipeLookupRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}
	return s.engine.RecipeLookup(ctx, req)
}

func (s *Server) toolSkillCraftPaths(ctx context.Context, args json.RawMessage) (any, error) {
	var req crafting.SkillCraftPathsRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}
	return s.engine.SkillCraftPaths(ctx, req)
}

func (s *Server) toolComponentUses(ctx context.Context, args json.RawMessage) (any, error) {
	var req crafting.ComponentUsesRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}
	return s.engine.ComponentUses(ctx, req)
}

func billOfMaterialsTool() ToolDefinition {
	minQty := 1.0

	return ToolDefinition{
		Name:        "bill_of_materials",
		Description: "Calculate the complete recursive bill of materials for a recipe. Returns all raw materials, intermediate items, and crafting steps needed in dependency order.",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]Property{
				"recipe_id": {
					Type:        "string",
					Description: "Recipe ID to calculate BOM for",
				},
				"quantity": {
					Type:        "integer",
					Description: "How many to craft",
					Default:     1,
					Minimum:     &minQty,
				},
			},
			Required: []string{"recipe_id"},
		},
	}
}

func (s *Server) toolBillOfMaterials(ctx context.Context, args json.RawMessage) (any, error) {
	var req crafting.BillOfMaterialsRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}
	return s.engine.BillOfMaterials(ctx, req)
}
