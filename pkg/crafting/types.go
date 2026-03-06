// Package crafting contains the core types for the crafting query server.
package crafting

import "encoding/json"

// ============================================
// ITEM TYPES
// ============================================

// Item represents a game item from the catalog.
type Item struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	Rarity      string `json:"rarity,omitempty"`
	Size        int    `json:"size,omitempty"`
	BaseValue   int    `json:"base_value,omitempty"`
	Stackable   bool   `json:"stackable,omitempty"`
	Tradeable   bool   `json:"tradeable,omitempty"`
}

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
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Description     string              `json:"description,omitempty"`
	Category        string              `json:"category,omitempty"`
	CraftingTime    int                 `json:"crafting_time,omitempty"`
	BaseQuality     int                 `json:"base_quality,omitempty"`
	SkillQualityMod int                 `json:"skill_quality_mod,omitempty"`
	RequiredSkills  map[string]int      `json:"required_skills,omitempty"`
	Inputs          []RecipeInput       `json:"inputs"`
	Outputs         []RecipeOutput      `json:"outputs"`
	SkillsRequired  []SkillRequirement  `json:"skills_required"`
	IllegalStatus   *IllegalStatus      `json:"illegal_status,omitempty"`
}

// RecipeInput represents a required input item for a recipe.
type RecipeInput struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

// SkillRequirement represents a skill level needed for a recipe.
type SkillRequirement struct {
	SkillID       string `json:"skill_id"`
	LevelRequired int    `json:"level_required"`
}

// RecipeOutput represents what a recipe produces.
type RecipeOutput struct {
	ItemID     string `json:"item_id"`
	Quantity   int    `json:"quantity"`
	QualityMod bool   `json:"quality_mod"`
}

// IllegalStatus indicates a recipe cannot be crafted privately.
type IllegalStatus struct {
	IsIllegal     bool   `json:"is_illegal"`
	BanReason     string `json:"ban_reason,omitempty"`
	LegalLocation string `json:"legal_location,omitempty"`
}

// ============================================
// SKILL TYPES
// ============================================

// Skill represents a skill in the progression tree.
type Skill struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Description    string             `json:"description,omitempty"`
	Category       string             `json:"category"`
	MaxLevel       int                `json:"max_level"`
	TrainingSource string             `json:"training_source,omitempty"`
	XPPerLevel     json.RawMessage    `json:"xp_per_level,omitempty"`
	BonusPerLevel  json.RawMessage    `json:"bonus_per_level,omitempty"`
	RequiredSkills json.RawMessage    `json:"required_skills_json,omitempty"`
	Prerequisites  []SkillRequirement `json:"prerequisites,omitempty"`
	XPThresholds   []int              `json:"xp_thresholds"`
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

	// NEW fields from Phase 3: Enhanced Market Data
	MSRP               int    `json:"msrp,omitempty"`
	MarketStatus       string `json:"market_status,omitempty"`       // "high_confidence", "low_confidence", "no_market_data"
	PricingMethod      string `json:"pricing_method,omitempty"`      // "volume_weighted", "second_price", "median", "msrp_only"
	SampleCount        int    `json:"sample_count,omitempty"`        // Number of orders used in calculation

	// Legacy field - renamed for clarity
	TotalVolume24h     int    `json:"total_volume_24h,omitempty"`    // Total trading volume in last 24h
	PriceTrend         string `json:"price_trend,omitempty"`
}

// MarketPriceSummary contains aggregated price data for an item.
type MarketPriceSummary struct {
	ItemID string  `json:"item_id"`
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
	InputsHave       []RecipeInput  `json:"inputs_have"`
	InputsMissing    []RecipeInput  `json:"inputs_missing"`
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
	ItemID string `json:"item_id"`
	Quantity    int    `json:"quantity"`
	Source      string `json:"source"` // "inventory", "previous_step", "acquire"
	SourceStep  int    `json:"source_step,omitempty"`
}

// MaterialRequirement represents an item needed for crafting.
type MaterialRequirement struct {
	ItemID             string        `json:"item_id"`
	QuantityNeeded     int           `json:"quantity_needed"`
	QuantityHave       int           `json:"quantity_have"`
	QuantityToAcquire  int           `json:"quantity_to_acquire"`
	AcquisitionMethods []string      `json:"acquisition_methods,omitempty"`
	IsCraftable        bool          `json:"is_craftable"`
	CraftRecipeID      string        `json:"craft_recipe_id,omitempty"`
	CraftIllegalStatus *IllegalStatus `json:"craft_illegal_status,omitempty"`
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
	Components         []Component          `json:"components"`
	Skills             map[string]int       `json:"skills"`
	IncludePartial     bool                 `json:"include_partial"`
	IncludeAmmunition  bool                 `json:"include_ammunition"`
	MinMatchRatio      float64              `json:"min_match_ratio"`
	Strategy           OptimizationStrategy `json:"optimization_strategy"`
	StationID          string               `json:"station_id,omitempty"`
	CategoryFilter     string               `json:"category_filter,omitempty"`
	Limit              int                  `json:"limit"`
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
	CraftingTime    int                   `json:"crafting_time"`
	Summary         CraftPathSummary      `json:"summary"`
}

// CraftPathTarget identifies the target recipe for a craft path query.
type CraftPathTarget struct {
	RecipeID      string         `json:"recipe_id"`
	RecipeName    string         `json:"recipe_name"`
	Quantity      int            `json:"quantity"`
	IllegalStatus *IllegalStatus `json:"illegal_status,omitempty"`
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
	ItemID             string               `json:"item_id"`
	Skills             map[string]int       `json:"skills,omitempty"`
	IncludeSkillLocked bool                 `json:"include_skill_locked"`
	StationID          string               `json:"station_id,omitempty"`
	Strategy           OptimizationStrategy `json:"optimization_strategy"`
}

// ComponentUsesResponse is the output for the component_uses tool.
type ComponentUsesResponse struct {
	ItemID          string             `json:"item_id"`
	ItemName        string             `json:"item_name,omitempty"`
	UsedIn          []ComponentUseInfo `json:"used_in"`
	TotalUses       int                `json:"total_uses"`
	MarketSellPrice int                `json:"market_sell_price,omitempty"`
}

// ComponentUseInfo describes how an item is used in a recipe.
type ComponentUseInfo struct {
	Recipe           Recipe          `json:"recipe"`
	QuantityPerCraft int             `json:"quantity_per_craft"`
	SkillReady       bool            `json:"skill_ready"`
	SkillGaps        []SkillGap      `json:"skill_gaps,omitempty"`
	ProfitAnalysis   *ProfitAnalysis `json:"profit_analysis,omitempty"`
}

// RecipeMarketProfit represents a single recipe's market profitability.
type RecipeMarketProfit struct {
	RecipeID        string `json:"recipe_id"`
	RecipeName      string `json:"recipe_name"`
	Category        string `json:"category"`
	OutputItemID    string `json:"output_item_id"`
	OutputQuantity  int    `json:"output_quantity"`
	OutputSellPrice int    `json:"output_sell_price"`
	OutputMSRP      int    `json:"output_msrp"`
	OutputUsesMSRP  bool   `json:"output_uses_msrp"`  // true if output price is MSRP, not market data
	InputCost       int    `json:"input_cost"`
	InputUsesMSRP    bool   `json:"input_uses_msrp"`    // true if any input used MSRP
	Profit          int    `json:"profit"`
	ProfitMarginPct float64 `json:"profit_margin_pct"`
}

// RecipeMarketProfitabilityResponse is the output for the recipe_market_profitability tool.
type RecipeMarketProfitabilityResponse struct {
	Recipes         []RecipeMarketProfit `json:"recipes"`
	TotalRecipes    int                  `json:"total_recipes"`
	StationID       string               `json:"station_id,omitempty"`
	EmpireID        string               `json:"empire_id,omitempty"`
	QueryTimestamp  string               `json:"query_timestamp"`
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
