// Package crafting contains the core types for the crafting query server.
package crafting

// ============================================
// INPUT TYPES
// ============================================

// Component represents an item with quantity (used in queries).
type Component struct {
	ID       string `json:"id"`
	Quantity int    `json:"quantity"`
}

// SkillProgress represents an agent's progress in a skill.
type SkillProgress struct {
	Level     int `json:"level"`
	CurrentXP int `json:"current_xp,omitempty"`
}

// AgentSkillState maps skill IDs to progress.
type AgentSkillState map[string]SkillProgress

// OptimizationStrategy controls result sorting/filtering.
type OptimizationStrategy string

const (
	StrategyMaximizeProfit      OptimizationStrategy = "MAXIMIZE_PROFIT"
	StrategyMaximizeVolume      OptimizationStrategy = "MAXIMIZE_VOLUME"
	StrategyOptimizeCraftPath   OptimizationStrategy = "OPTIMIZE_CRAFT_PATH"
	StrategyUseInventoryFirst   OptimizationStrategy = "USE_INVENTORY_FIRST"
	StrategyMinimizeAcquisition OptimizationStrategy = "MINIMIZE_ACQUISITION"
)

// ValidStrategies returns all valid optimization strategies.
func ValidStrategies() []OptimizationStrategy {
	return []OptimizationStrategy{
		StrategyMaximizeProfit,
		StrategyMaximizeVolume,
		StrategyOptimizeCraftPath,
		StrategyUseInventoryFirst,
		StrategyMinimizeAcquisition,
	}
}

// IsValid checks if the strategy is a known valid strategy.
func (s OptimizationStrategy) IsValid() bool {
	for _, valid := range ValidStrategies() {
		if s == valid {
			return true
		}
	}
	return false
}

// ============================================
// RECIPE TYPES
// ============================================

// Recipe represents a craftable item with its requirements.
type Recipe struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Description    string             `json:"description,omitempty"`
	Category       string             `json:"category,omitempty"`
	CraftTimeSec   int                `json:"craft_time_sec,omitempty"`
	Components     []RecipeComponent  `json:"components"`
	SkillsRequired []SkillRequirement `json:"skills_required"`
	Output         RecipeOutput       `json:"output"`
}

// RecipeComponent represents a required input component for a recipe.
type RecipeComponent struct {
	ComponentID string `json:"component_id"`
	Quantity    int    `json:"quantity"`
}

// SkillRequirement represents a skill level needed for a recipe.
type SkillRequirement struct {
	SkillID       string `json:"skill_id"`
	LevelRequired int    `json:"level_required"`
}

// RecipeOutput represents what a recipe produces.
type RecipeOutput struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

// ============================================
// SKILL TYPES
// ============================================

// Skill represents a skill in the progression tree.
type Skill struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Category      string             `json:"category"`
	Description   string             `json:"description,omitempty"`
	MaxLevel      int                `json:"max_level"`
	Prerequisites []SkillRequirement `json:"prerequisites,omitempty"`
	XPThresholds  []int              `json:"xp_thresholds"` // XP needed for each level
}

// SkillGap represents the difference between current and required skill levels.
type SkillGap struct {
	SkillID       string `json:"skill_id"`
	SkillName     string `json:"skill_name"`
	CurrentLevel  int    `json:"current_level"`
	RequiredLevel int    `json:"required_level"`
	XPToNext      int    `json:"xp_to_next,omitempty"`
}

// ============================================
// MARKET TYPES
// ============================================

// ProfitAnalysis contains market-based profit calculations for a recipe.
type ProfitAnalysis struct {
	OutputSellPrice      int     `json:"output_sell_price"`
	InputCost            int     `json:"input_cost"`
	ProfitPerUnit        int     `json:"profit_per_unit"`
	ProfitMarginPct      float64 `json:"profit_margin_pct"`
	TotalPotentialProfit int     `json:"total_potential_profit,omitempty"`
	MarketVolume24h      int     `json:"market_volume_24h,omitempty"`
	PriceTrend           string  `json:"price_trend,omitempty"`
}

// MarketPriceSummary contains aggregated price data for a component.
type MarketPriceSummary struct {
	ComponentID string  `json:"component_id"`
	StationID   string  `json:"station_id"`
	PriceType   string  `json:"price_type"` // "buy" or "sell"
	AvgPrice7d  float64 `json:"avg_price_7d"`
	MinPrice7d  int     `json:"min_price_7d"`
	MaxPrice7d  int     `json:"max_price_7d"`
	PriceTrend  string  `json:"price_trend"`
}

// ============================================
// QUERY RESULT TYPES
// ============================================

// CraftableMatch represents a recipe the agent can craft right now.
type CraftableMatch struct {
	Recipe           Recipe          `json:"recipe"`
	CanCraftQuantity int             `json:"can_craft_quantity"`
	ProfitAnalysis   *ProfitAnalysis `json:"profit_analysis,omitempty"`
}

// PartialComponentMatch represents a recipe where the agent has some components.
type PartialComponentMatch struct {
	Recipe            Recipe             `json:"recipe"`
	ComponentsHave    []RecipeComponent  `json:"components_have"`
	ComponentsMissing []RecipeComponent  `json:"components_missing"`
	MatchRatio        float64            `json:"match_ratio"`
	SkillsReady       bool               `json:"skills_ready"`
	SkillsMissing     []SkillGap         `json:"skills_missing,omitempty"`
	ProfitAnalysis    *ProfitAnalysis    `json:"profit_analysis,omitempty"`
}

// CraftStep represents a single step in a crafting path.
type CraftStep struct {
	StepNumber      int              `json:"step_number"`
	RecipeID        string           `json:"recipe_id"`
	RecipeName      string           `json:"recipe_name"`
	QuantityToCraft int              `json:"quantity_to_craft"`
	Inputs          []CraftStepInput `json:"inputs"`
	Output          RecipeOutput     `json:"output"`
	SkillReady      bool             `json:"skill_ready"`
	SkillGap        *SkillGap        `json:"skill_gap,omitempty"`
}

// CraftStepInput represents an input component for a craft step.
type CraftStepInput struct {
	ComponentID string `json:"component_id"`
	Quantity    int    `json:"quantity"`
	Source      string `json:"source"` // "inventory", "previous_step", "acquire"
	SourceStep  int    `json:"source_step,omitempty"`
}

// MaterialRequirement represents a component needed for crafting.
type MaterialRequirement struct {
	ComponentID        string   `json:"component_id"`
	QuantityNeeded     int      `json:"quantity_needed"`
	QuantityHave       int      `json:"quantity_have"`
	QuantityToAcquire  int      `json:"quantity_to_acquire"`
	AcquisitionMethods []string `json:"acquisition_methods,omitempty"`
	IsCraftable        bool     `json:"is_craftable"`
	CraftRecipeID      string   `json:"craft_recipe_id,omitempty"`
}

// SkillUnlockPath represents a skill that would unlock recipes if leveled.
type SkillUnlockPath struct {
	Skill           Skill    `json:"skill"`
	CurrentLevel    int      `json:"current_level"`
	XPToNextLevel   int      `json:"xp_to_next_level"`
	RecipesUnlocked []string `json:"recipes_unlocked_at_next"`
}

// ============================================
// TOOL REQUEST/RESPONSE TYPES
// ============================================

// CraftQueryRequest is the input for the craft_query tool.
type CraftQueryRequest struct {
	Components     []Component          `json:"components"`
	Skills         map[string]int       `json:"skills"`
	IncludePartial bool                 `json:"include_partial"`
	MinMatchRatio  float64              `json:"min_match_ratio"`
	Strategy       OptimizationStrategy `json:"optimization_strategy"`
	StationID      string               `json:"station_id,omitempty"`
	CategoryFilter string               `json:"category_filter,omitempty"`
	Limit          int                  `json:"limit"`
}

// CraftQueryResponse is the output for the craft_query tool.
type CraftQueryResponse struct {
	Craftable         []CraftableMatch        `json:"craftable"`
	PartialComponents []PartialComponentMatch `json:"partial_components"`
	BlockedBySkills   []PartialComponentMatch `json:"blocked_by_skills"`
	QueryStats        QueryStats              `json:"query_stats"`
}

// QueryStats contains metadata about a query execution.
type QueryStats struct {
	TotalRecipesChecked int    `json:"total_recipes_checked"`
	ComponentsProvided  int    `json:"components_provided"`
	StrategyUsed        string `json:"strategy_used"`
	ProcessingTimeMs    int64  `json:"processing_time_ms"`
}

// CraftPathRequest is the input for the craft_path_to tool.
type CraftPathRequest struct {
	TargetRecipeID   string         `json:"target_recipe_id"`
	TargetQuantity   int            `json:"target_quantity"`
	CurrentInventory []Component    `json:"current_inventory"`
	Skills           map[string]int `json:"skills"`
	StationID        string         `json:"station_id,omitempty"`
}

// CraftPathResponse is the output for the craft_path_to tool.
type CraftPathResponse struct {
	Target          CraftPathTarget       `json:"target"`
	Feasible        bool                  `json:"feasible"`
	SkillReady      bool                  `json:"skill_ready"`
	SkillsMissing   []SkillGap            `json:"skills_missing,omitempty"`
	MaterialsNeeded []MaterialRequirement `json:"materials_needed"`
	CraftTimeSec    int                   `json:"craft_time_sec"`
	Summary         CraftPathSummary      `json:"summary"`
}

// CraftPathTarget identifies the target recipe for a craft path query.
type CraftPathTarget struct {
	RecipeID   string `json:"recipe_id"`
	RecipeName string `json:"recipe_name"`
	Quantity   int    `json:"quantity"`
}

// CraftPathSummary provides aggregate info about a craft path.
type CraftPathSummary struct {
	TotalComponents     int `json:"total_components"`
	ComponentsHave      int `json:"components_have"`
	ComponentsToAcquire int `json:"components_to_acquire"`
	ComponentsCraftable int `json:"components_craftable"`
}

// RecipeLookupRequest is the input for the recipe_lookup tool.
type RecipeLookupRequest struct {
	RecipeID  string         `json:"recipe_id,omitempty"`
	Search    string         `json:"search,omitempty"`
	Skills    map[string]int `json:"skills,omitempty"`
	StationID string         `json:"station_id,omitempty"`
}

// RecipeLookupResponse is the output for the recipe_lookup tool.
type RecipeLookupResponse struct {
	Recipe         *Recipe           `json:"recipe,omitempty"`
	SkillReady     bool              `json:"skill_ready"`
	SkillGaps      []SkillGap        `json:"skill_gaps,omitempty"`
	ProfitAnalysis *ProfitAnalysis   `json:"profit_analysis,omitempty"`
	UsedInRecipes  []string          `json:"used_in_recipes,omitempty"`
	SearchResults  []RecipeSearchHit `json:"search_results,omitempty"`
}

// RecipeSearchHit is a lightweight recipe match for search results.
type RecipeSearchHit struct {
	RecipeID string `json:"recipe_id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

// SkillCraftPathsRequest is the input for the skill_craft_paths tool.
type SkillCraftPathsRequest struct {
	Skills         map[string]SkillProgress `json:"skills"`
	CategoryFilter string                   `json:"category_filter,omitempty"`
	Limit          int                      `json:"limit"`
}

// SkillCraftPathsResponse is the output for the skill_craft_paths tool.
type SkillCraftPathsResponse struct {
	SkillPaths []SkillUnlockPath      `json:"skill_paths"`
	Summary    SkillCraftPathsSummary `json:"summary"`
}

// SkillCraftPathsSummary provides aggregate info about skill unlock potential.
type SkillCraftPathsSummary struct {
	TotalRecipes       int    `json:"total_recipes"`
	RecipesUnlocked    int    `json:"recipes_unlocked"`
	RecipesLocked      int    `json:"recipes_locked"`
	ClosestUnlockSkill string `json:"closest_unlock_skill,omitempty"`
	ClosestUnlockXP    int    `json:"closest_unlock_xp,omitempty"`
}

// ComponentUsesRequest is the input for the component_uses tool.
type ComponentUsesRequest struct {
	ComponentID        string               `json:"component_id"`
	Skills             map[string]int       `json:"skills,omitempty"`
	IncludeSkillLocked bool                 `json:"include_skill_locked"`
	StationID          string               `json:"station_id,omitempty"`
	Strategy           OptimizationStrategy `json:"optimization_strategy"`
}

// ComponentUsesResponse is the output for the component_uses tool.
type ComponentUsesResponse struct {
	ComponentID     string             `json:"component_id"`
	ComponentName   string             `json:"component_name,omitempty"`
	UsedIn          []ComponentUseInfo `json:"used_in"`
	TotalUses       int                `json:"total_uses"`
	MarketSellPrice int                `json:"market_sell_price,omitempty"`
}

// ComponentUseInfo describes how a component is used in a recipe.
type ComponentUseInfo struct {
	Recipe           Recipe          `json:"recipe"`
	QuantityPerCraft int             `json:"quantity_per_craft"`
	SkillReady       bool            `json:"skill_ready"`
	SkillGaps        []SkillGap      `json:"skill_gaps,omitempty"`
	ProfitAnalysis   *ProfitAnalysis `json:"profit_analysis,omitempty"`
}

// BillOfMaterialsRequest is the input for the bill_of_materials tool.
type BillOfMaterialsRequest struct {
	RecipeID string `json:"recipe_id"`
	Quantity int    `json:"quantity"`
}

// BillOfMaterialsResponse is the output for the bill_of_materials tool.
type BillOfMaterialsResponse struct {
	RecipeID       string            `json:"recipe_id"`
	RecipeName     string            `json:"recipe_name"`
	OutputItemID   string            `json:"output_item_id"`
	Quantity       int               `json:"quantity"`
	RawMaterials   []BOMItem         `json:"raw_materials"`
	Intermediates  []BOMIntermediate `json:"intermediates"`
	CraftSteps     []BOMCraftStep    `json:"craft_steps"`
	TotalCraftTime int               `json:"total_craft_time_sec"`
}

// BOMItem represents a raw material requirement.
type BOMItem struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

// BOMIntermediate represents an intermediate crafted item in the dependency tree.
type BOMIntermediate struct {
	ItemID        string `json:"item_id"`
	RecipeID      string `json:"recipe_id"`
	RecipeName    string `json:"recipe_name"`
	CraftRuns     int    `json:"craft_runs"`
	TotalProduced int    `json:"total_produced"`
	TotalNeeded   int    `json:"total_needed"`
}

// BOMCraftStep represents a single crafting operation in the build order.
type BOMCraftStep struct {
	StepNumber   int    `json:"step_number"`
	RecipeID     string `json:"recipe_id"`
	RecipeName   string `json:"recipe_name"`
	CraftRuns    int    `json:"craft_runs"`
	OutputItemID string `json:"output_item_id"`
	OutputPerRun int    `json:"output_per_run"`
}
