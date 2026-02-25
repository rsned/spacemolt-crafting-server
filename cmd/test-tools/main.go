// Command test-tools provides comprehensive testing for the MCP crafting server.
// It tests all 6 tools with invalid/non-existent IDs, simple queries, and complex queries.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/rsned/spacemolt-crafting-server/internal/crafting/db"
	"github.com/rsned/spacemolt-crafting-server/internal/crafting/engine"
	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// TestResult tracks the outcome of a single test case
type TestResult struct {
	Name     string
	Tool     string
	Category string // "invalid", "simple", "complex"
	Passed   bool
	Error    error
	Duration time.Duration
}

// TestStats aggregates test results
type TestStats struct {
	Total       int
	Passed      int
	Failed      int
	ByTool      map[string]int
	ByCategory  map[string]int
}

func main() {
	verbose := flag.Bool("v", false, "show full results instead of eliding them")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Find database path
	dbPath := os.Getenv("CRAFTING_DB")
	if dbPath == "" {
		// Try default location
		if _, err := os.Stat("database/crafting.db"); err == nil {
			dbPath = "database/crafting.db"
		} else if _, err := os.Stat("data/crafting/crafting.db"); err == nil {
			dbPath = "data/crafting/crafting.db"
		} else {
			log.Error("Could not find database file. Set CRAFTING_DB environment variable or ensure database exists in default location")
			os.Exit(1)
		}
	}

	// Initialize database
	ctx := context.Background()
	database, err := db.Open(dbPath)
	if err != nil {
		log.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Error("Failed to close database", "error", err)
		}
	}()

	// Initialize engine
	eng := engine.New(database)

	// Run all tests
	results := runAllTests(ctx, eng, log, *verbose)

	// Print summary
	printSummary(results)
}

func runAllTests(ctx context.Context, eng *engine.Engine, log *slog.Logger, verbose bool) []TestResult {
	var results []TestResult

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("CRAFTING SERVER MCP TOOLS TEST SUITE")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	// Test 1: craft_query
	fmt.Println("Testing: craft_query")
	results = append(results, testCraftQuery(ctx, eng, log, verbose)...)

	// Test 2: craft_path_to
	fmt.Println("\nTesting: craft_path_to")
	results = append(results, testCraftPathTo(ctx, eng, log, verbose)...)

	// Test 3: recipe_lookup
	fmt.Println("\nTesting: recipe_lookup")
	results = append(results, testRecipeLookup(ctx, eng, log, verbose)...)

	// Test 4: skill_craft_paths
	fmt.Println("\nTesting: skill_craft_paths")
	results = append(results, testSkillCraftPaths(ctx, eng, log, verbose)...)

	// Test 5: component_uses
	fmt.Println("\nTesting: component_uses")
	results = append(results, testComponentUses(ctx, eng, log, verbose)...)

	// Test 6: bill_of_materials
	fmt.Println("\nTesting: bill_of_materials")
	results = append(results, testBillOfMaterials(ctx, eng, log, verbose)...)

	return results
}

// ============================================================================
// TOOL 1: craft_query
// ============================================================================

func testCraftQuery(ctx context.Context, eng *engine.Engine, _ *slog.Logger, verbose bool) []TestResult {
	var results []TestResult

	baseSkills := map[string]int{"crafting_basic": 1}

	// INVALID: Non-existent items
	results = append(results, runTest(ctx, eng, "craft_query", "invalid",
		"craft_query with non-existent items",
		func() (any, error) {
			return eng.CraftQuery(ctx, crafting.CraftQueryRequest{
				Components: []crafting.Component{
					{ID: "chicken_pot_pie", Quantity: 10},
					{ID: "dragon_scales", Quantity: 5},
				},
				Skills: baseSkills,
				Limit:  10,
			})
		}, verbose,
	))

	// INVALID: Empty components
	results = append(results, runTest(ctx, eng, "craft_query", "invalid",
		"craft_query with empty components",
		func() (any, error) {
			return eng.CraftQuery(ctx, crafting.CraftQueryRequest{
				Components: []crafting.Component{},
				Skills:     baseSkills,
			})
		}, verbose,
	))

	// INVALID: Negative quantity
	results = append(results, runTest(ctx, eng, "craft_query", "invalid",
		"craft_query with negative quantity",
		func() (any, error) {
			return eng.CraftQuery(ctx, crafting.CraftQueryRequest{
				Components: []crafting.Component{
					{ID: "ore_iron", Quantity: -5},
				},
				Skills: baseSkills,
			})
		}, verbose,
	))

	// SIMPLE: Basic iron ore query
	results = append(results, runTest(ctx, eng, "craft_query", "simple",
		"craft_query with ore_iron: 10",
		func() (any, error) {
			return eng.CraftQuery(ctx, crafting.CraftQueryRequest{
				Components: []crafting.Component{
					{ID: "ore_iron", Quantity: 10},
				},
				Skills: baseSkills,
				Limit:  10,
			})
		}, verbose,
	))

	// SIMPLE: Multiple components
	results = append(results, runTest(ctx, eng, "craft_query", "simple",
		"craft_query with iron and copper ore",
		func() (any, error) {
			return eng.CraftQuery(ctx, crafting.CraftQueryRequest{
				Components: []crafting.Component{
					{ID: "ore_iron", Quantity: 20},
					{ID: "ore_copper", Quantity: 15},
				},
				Skills: baseSkills,
				Limit:  15,
			})
		}, verbose,
	))

	// COMPLEX: With optimization strategies
	for _, strategy := range []crafting.OptimizationStrategy{
		crafting.StrategyMaximizeProfit,
		crafting.StrategyMaximizeVolume,
		crafting.StrategyOptimizeCraftPath,
		crafting.StrategyUseInventoryFirst,
		crafting.StrategyMinimizeAcquisition,
	} {
		results = append(results, runTest(ctx, eng, "craft_query", "complex",
			fmt.Sprintf("craft_query with strategy: %s", strategy),
			func() (any, error) {
				return eng.CraftQuery(ctx, crafting.CraftQueryRequest{
					Components: []crafting.Component{
						{ID: "ore_iron", Quantity: 50},
						{ID: "ore_copper", Quantity: 30},
						{ID: "comp_circuit_board", Quantity: 5},
					},
					Skills:              baseSkills,
					Limit:               20,
					Strategy:            strategy,
					IncludePartial:      true,
					MinMatchRatio:       0.25,
				})
			}, verbose,
		))
	}

	// COMPLEX: With category filter
	results = append(results, runTest(ctx, eng, "craft_query", "complex",
		"craft_query with category filter",
		func() (any, error) {
			return eng.CraftQuery(ctx, crafting.CraftQueryRequest{
				Components: []crafting.Component{
					{ID: "ore_iron", Quantity: 100},
					{ID: "comp_capital_frame", Quantity: 2},
				},
				Skills:         baseSkills,
				CategoryFilter: "Components",
				Limit:          25,
			})
		}, verbose,
	))

	return results
}

// ============================================================================
// TOOL 2: craft_path_to
// ============================================================================

func testCraftPathTo(ctx context.Context, eng *engine.Engine, _ *slog.Logger, verbose bool) []TestResult {
	var results []TestResult

	baseSkills := map[string]int{"crafting_basic": 1}

	// INVALID: Non-existent recipe
	results = append(results, runTest(ctx, eng, "craft_path_to", "invalid",
		"craft_path_to for non-existent recipe",
		func() (any, error) {
			return eng.CraftPathTo(ctx, crafting.CraftPathRequest{
				TargetRecipeID: "chicken_pot_pie",
				TargetQuantity: 1,
				Skills:         baseSkills,
			})
		}, verbose,
	))

	// INVALID: Zero or negative quantity
	results = append(results, runTest(ctx, eng, "craft_path_to", "invalid",
		"craft_path_to with zero quantity",
		func() (any, error) {
			return eng.CraftPathTo(ctx, crafting.CraftPathRequest{
				TargetRecipeID: "craft_engine_core",
				TargetQuantity: 0,
				Skills:         baseSkills,
			})
		}, verbose,
	))

	// SIMPLE: Basic component path
	results = append(results, runTest(ctx, eng, "craft_path_to", "simple",
		"craft_path_to for engine_core",
		func() (any, error) {
			return eng.CraftPathTo(ctx, crafting.CraftPathRequest{
				TargetRecipeID: "craft_engine_core",
				TargetQuantity: 1,
				Skills:         baseSkills,
			})
		}, verbose,
	))

	// SIMPLE: With inventory
	results = append(results, runTest(ctx, eng, "craft_path_to", "simple",
		"craft_path_to with partial inventory",
		func() (any, error) {
			return eng.CraftPathTo(ctx, crafting.CraftPathRequest{
				TargetRecipeID: "craft_laser_focus",
				TargetQuantity: 1,
				CurrentInventory: []crafting.Component{
					{ID: "comp_optical_sensor", Quantity: 2},
				},
				Skills: baseSkills,
			})
		}, verbose,
	))

	// COMPLEX: Multi-tier item (quantum_matrix has 3 tiers deep)
	results = append(results, runTest(ctx, eng, "craft_path_to", "complex",
		"craft_path_to for quantum_matrix (multi-tier)",
		func() (any, error) {
			return eng.CraftPathTo(ctx, crafting.CraftPathRequest{
				TargetRecipeID: "assemble_quantum_matrix",
				TargetQuantity: 1,
				CurrentInventory: []crafting.Component{
					{ID: "comp_quantum_core", Quantity: 1},
				},
				Skills: baseSkills,
			})
		}, verbose,
	))

	// COMPLEX: Large quantity
	results = append(results, runTest(ctx, eng, "craft_path_to", "complex",
		"craft_path_to for large quantity",
		func() (any, error) {
			return eng.CraftPathTo(ctx, crafting.CraftPathRequest{
				TargetRecipeID: "craft_capital_armor_plate",
				TargetQuantity: 10,
				Skills:         baseSkills,
			})
		}, verbose,
	))

	return results
}

// ============================================================================
// TOOL 3: recipe_lookup
// ============================================================================

func testRecipeLookup(ctx context.Context, eng *engine.Engine, _ *slog.Logger, verbose bool) []TestResult {
	var results []TestResult

	baseSkills := map[string]int{"crafting_basic": 1}

	// INVALID: Non-existent recipe
	results = append(results, runTest(ctx, eng, "recipe_lookup", "invalid",
		"recipe_lookup for non-existent recipe",
		func() (any, error) {
			return eng.RecipeLookup(ctx, crafting.RecipeLookupRequest{
				RecipeID: "chicken_pot_pie",
				Skills:   baseSkills,
			})
		}, verbose,
	))

	// INVALID: Empty search
	results = append(results, runTest(ctx, eng, "recipe_lookup", "invalid",
		"recipe_lookup with empty search",
		func() (any, error) {
			return eng.RecipeLookup(ctx, crafting.RecipeLookupRequest{
				Search: "",
				Skills: baseSkills,
			})
		}, verbose,
	))

	// SIMPLE: Lookup by exact ID
	results = append(results, runTest(ctx, eng, "recipe_lookup", "simple",
		"recipe_lookup by ID for engine_core",
		func() (any, error) {
			return eng.RecipeLookup(ctx, crafting.RecipeLookupRequest{
				RecipeID: "craft_engine_core",
				Skills:   baseSkills,
			})
		}, verbose,
	))

	// SIMPLE: Search by name
	results = append(results, runTest(ctx, eng, "recipe_lookup", "simple",
		"recipe_lookup search for 'laser'",
		func() (any, error) {
			return eng.RecipeLookup(ctx, crafting.RecipeLookupRequest{
				Search: "laser",
				Skills: baseSkills,
			})
		}, verbose,
	))

	// COMPLEX: Search with skill gap analysis
	results = append(results, runTest(ctx, eng, "recipe_lookup", "complex",
		"recipe_lookup with skill gap analysis",
		func() (any, error) {
			return eng.RecipeLookup(ctx, crafting.RecipeLookupRequest{
				RecipeID: "craft_adaptive_shield_1",
				Skills: map[string]int{
					"crafting_basic": 1,
					"shields_basic":  2,
				},
			})
		}, verbose,
	))

	// COMPLEX: Search for used_in recipes
	results = append(results, runTest(ctx, eng, "recipe_lookup", "complex",
		"recipe_lookup showing used_in chain",
		func() (any, error) {
			return eng.RecipeLookup(ctx, crafting.RecipeLookupRequest{
				RecipeID: "craft_engine_core",
				Skills:   baseSkills,
			})
		}, verbose,
	))

	return results
}

// ============================================================================
// TOOL 4: skill_craft_paths
// ============================================================================

func testSkillCraftPaths(ctx context.Context, eng *engine.Engine, _ *slog.Logger, verbose bool) []TestResult {
	var results []TestResult

	// INVALID: Non-existent skill
	results = append(results, runTest(ctx, eng, "skill_craft_paths", "invalid",
		"skill_craft_paths with non-existent skill",
		func() (any, error) {
			return eng.SkillCraftPaths(ctx, crafting.SkillCraftPathsRequest{
				Skills: map[string]crafting.SkillProgress{
					"dragon_taming": {Level: 5},
				},
			})
		}, verbose,
	))

	// INVALID: Empty skills
	results = append(results, runTest(ctx, eng, "skill_craft_paths", "invalid",
		"skill_craft_paths with empty skills",
		func() (any, error) {
			return eng.SkillCraftPaths(ctx, crafting.SkillCraftPathsRequest{
				Skills: map[string]crafting.SkillProgress{},
			})
		}, verbose,
	))

	// SIMPLE: Single basic skill
	results = append(results, runTest(ctx, eng, "skill_craft_paths", "simple",
		"skill_craft_paths for basic crafting",
		func() (any, error) {
			return eng.SkillCraftPaths(ctx, crafting.SkillCraftPathsRequest{
				Skills: map[string]crafting.SkillProgress{
					"crafting_basic": {Level: 1, CurrentXP: 100},
				},
				Limit: 10,
			})
		}, verbose,
	))

	// SIMPLE: Multiple skills
	results = append(results, runTest(ctx, eng, "skill_craft_paths", "simple",
		"skill_craft_paths for multiple skills",
		func() (any, error) {
			return eng.SkillCraftPaths(ctx, crafting.SkillCraftPathsRequest{
				Skills: map[string]crafting.SkillProgress{
					"armor":            {Level: 2, CurrentXP: 500},
					"weapons_basic":    {Level: 1, CurrentXP: 200},
					"shields_advanced": {Level: 3, CurrentXP: 1500},
				},
				Limit: 15,
			})
		}, verbose,
	))

	// COMPLEX: With category filter
	results = append(results, runTest(ctx, eng, "skill_craft_paths", "complex",
		"skill_craft_paths with category filter",
		func() (any, error) {
			return eng.SkillCraftPaths(ctx, crafting.SkillCraftPathsRequest{
				Skills: map[string]crafting.SkillProgress{
					"capital_weapons":  {Level: 2, CurrentXP: 800},
					"weapons_advanced": {Level: 1, CurrentXP: 300},
					"armor":            {Level: 3, CurrentXP: 2000},
				},
				CategoryFilter: "Combat",
				Limit:          20,
			})
		}, verbose,
	))

	// COMPLEX: High-level skill with many unlocks
	results = append(results, runTest(ctx, eng, "skill_craft_paths", "complex",
		"skill_craft_paths for advanced skill",
		func() (any, error) {
			return eng.SkillCraftPaths(ctx, crafting.SkillCraftPathsRequest{
				Skills: map[string]crafting.SkillProgress{
					"armor_advanced": {Level: 5, CurrentXP: 5000},
				},
				Limit: 50,
			})
		}, verbose,
	))

	return results
}

// ============================================================================
// TOOL 5: component_uses
// ============================================================================

func testComponentUses(ctx context.Context, eng *engine.Engine, _ *slog.Logger, verbose bool) []TestResult {
	var results []TestResult

	baseSkills := map[string]int{"crafting_basic": 1}

	// INVALID: Non-existent component
	results = append(results, runTest(ctx, eng, "component_uses", "invalid",
		"component_uses for non-existent component",
		func() (any, error) {
			return eng.ComponentUses(ctx, crafting.ComponentUsesRequest{
				ItemID: "chicken_pot_pie",
				Skills: baseSkills,
			})
		}, verbose,
	))

	// INVALID: Empty component ID
	results = append(results, runTest(ctx, eng, "component_uses", "invalid",
		"component_uses with empty component ID",
		func() (any, error) {
			return eng.ComponentUses(ctx, crafting.ComponentUsesRequest{
				ItemID: "",
				Skills: baseSkills,
			})
		}, verbose,
	))

	// SIMPLE: Basic raw material
	results = append(results, runTest(ctx, eng, "component_uses", "simple",
		"component_uses for ore_iron",
		func() (any, error) {
			return eng.ComponentUses(ctx, crafting.ComponentUsesRequest{
				ItemID: "ore_iron",
				Skills: baseSkills,
			})
		}, verbose,
	))

	// SIMPLE: Intermediate component
	results = append(results, runTest(ctx, eng, "component_uses", "simple",
		"component_uses for circuit_board",
		func() (any, error) {
			return eng.ComponentUses(ctx, crafting.ComponentUsesRequest{
				ItemID: "comp_circuit_board",
				Skills: baseSkills,
			})
		}, verbose,
	))

	// COMPLEX: With optimization strategies
	for _, strategy := range []crafting.OptimizationStrategy{
		crafting.StrategyMaximizeProfit,
		crafting.StrategyUseInventoryFirst,
	} {
		results = append(results, runTest(ctx, eng, "component_uses", "complex",
			fmt.Sprintf("component_uses with strategy: %s", strategy),
			func() (any, error) {
				return eng.ComponentUses(ctx, crafting.ComponentUsesRequest{
					ItemID:             "comp_capital_frame",
					Skills:             baseSkills,
					IncludeSkillLocked: true,
					Strategy:           strategy,
				})
			}, verbose,
		))
	}

	// COMPLEX: Component used in many recipes
	results = append(results, runTest(ctx, eng, "component_uses", "complex",
		"component_uses for widely-used component",
		func() (any, error) {
			return eng.ComponentUses(ctx, crafting.ComponentUsesRequest{
				ItemID:             "comp_armor_plate",
				Skills:             baseSkills,
				IncludeSkillLocked: true,
			})
		}, verbose,
	))

	return results
}

// ============================================================================
// TOOL 6: bill_of_materials
// ============================================================================

func testBillOfMaterials(ctx context.Context, eng *engine.Engine, _ *slog.Logger, verbose bool) []TestResult {
	var results []TestResult

	// INVALID: Non-existent recipe
	results = append(results, runTest(ctx, eng, "bill_of_materials", "invalid",
		"bill_of_materials for non-existent recipe",
		func() (any, error) {
			return eng.BillOfMaterials(ctx, crafting.BillOfMaterialsRequest{
				RecipeID: "chicken_pot_pie",
				Quantity: 1,
			})
		}, verbose,
	))

	// INVALID: Zero or negative quantity
	results = append(results, runTest(ctx, eng, "bill_of_materials", "invalid",
		"bill_of_materials with zero quantity",
		func() (any, error) {
			return eng.BillOfMaterials(ctx, crafting.BillOfMaterialsRequest{
				RecipeID: "craft_engine_core",
				Quantity: 0,
			})
		}, verbose,
	))

	// SIMPLE: Basic component recipe
	results = append(results, runTest(ctx, eng, "bill_of_materials", "simple",
		"bill_of_materials for engine_core",
		func() (any, error) {
			return eng.BillOfMaterials(ctx, crafting.BillOfMaterialsRequest{
				RecipeID: "craft_engine_core",
				Quantity: 1,
			})
		}, verbose,
	))

	// SIMPLE: Multiple quantity
	results = append(results, runTest(ctx, eng, "bill_of_materials", "simple",
		"bill_of_materials for multiple units",
		func() (any, error) {
			return eng.BillOfMaterials(ctx, crafting.BillOfMaterialsRequest{
				RecipeID: "craft_power_core",
				Quantity: 5,
			})
		}, verbose,
	))

	// COMPLEX: Deep recursive BOM (quantum_matrix - 3+ tiers)
	results = append(results, runTest(ctx, eng, "bill_of_materials", "complex",
		"bill_of_materials for quantum_matrix (deep recursion)",
		func() (any, error) {
			return eng.BillOfMaterials(ctx, crafting.BillOfMaterialsRequest{
				RecipeID: "assemble_quantum_matrix",
				Quantity: 1,
			})
		}, verbose,
	))

	// COMPLEX: Large quantity of complex item
	results = append(results, runTest(ctx, eng, "bill_of_materials", "complex",
		"bill_of_materials for large batch of complex item",
		func() (any, error) {
			return eng.BillOfMaterials(ctx, crafting.BillOfMaterialsRequest{
				RecipeID: "craft_adaptive_shield_1",
				Quantity: 10,
			})
		}, verbose,
	))

	// COMPLEX: Multi-tier capital component
	results = append(results, runTest(ctx, eng, "bill_of_materials", "complex",
		"bill_of_materials for capital armor plate (multi-tier)",
		func() (any, error) {
			return eng.BillOfMaterials(ctx, crafting.BillOfMaterialsRequest{
				RecipeID: "craft_capital_armor_plate",
				Quantity: 1,
			})
		}, verbose,
	))

	return results
}

// ============================================================================
// TEST UTILITIES
// ============================================================================

func runTest(_ context.Context, _ *engine.Engine, tool, category, name string, testFunc func() (any, error), verbose bool) TestResult {
	start := time.Now()
	result, err := testFunc()
	duration := time.Since(start)

	passed := err == nil

	resultObj := TestResult{
		Name:     name,
		Tool:     tool,
		Category: category,
		Passed:   passed,
		Error:    err,
		Duration: duration,
	}

	// Print result
	status := "✓ PASS"
	if !passed {
		status = "✗ FAIL"
	}

	fmt.Printf("  %s [%s] %s (%s)\n", status, category, name, duration.Round(time.Millisecond))

	if !passed {
		fmt.Printf("       Error: %v\n", err)
	} else if shouldShowDetails(category) {
		// Show a preview of the result for simple/complex tests
		showResultPreview(result, verbose)
	}

	return resultObj
}

func shouldShowDetails(category string) bool {
	return category == "simple" || category == "complex"
}

func showResultPreview(result any, verbose bool) {
	// Convert to JSON for preview
	jsonData, err := json.MarshalIndent(result, "         ", "  ")
	if err != nil {
		return
	}

	// If verbose is true, show full output; otherwise show first 10 lines
	str := string(jsonData)
	lines := strings.Split(str, "\n")

	if verbose {
		// Show all lines when verbose is true
		for _, line := range lines {
			fmt.Println(line)
		}
	} else {
		// Show first few lines when not verbose
		if len(lines) > 10 {
			lines = append(lines[:10], "         ...")
		}
		for _, line := range lines {
			if len(line) > 100 {
				line = line[:97] + "..."
			}
			fmt.Println(line)
		}
	}
}

func printSummary(results []TestResult) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("TEST SUMMARY")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	stats := TestStats{
		ByTool:     make(map[string]int),
		ByCategory: make(map[string]int),
	}

	// Count results
	for _, r := range results {
		stats.Total++
		stats.ByTool[r.Tool]++
		stats.ByCategory[r.Category]++

		if r.Passed {
			stats.Passed++
		} else {
			stats.Failed++
		}
	}

	// Print overall stats
	fmt.Printf("Total Tests:  %d\n", stats.Total)
	fmt.Printf("Passed:       %d (%.1f%%)\n", stats.Passed, float64(stats.Passed)*100/float64(stats.Total))
	fmt.Printf("Failed:       %d (%.1f%%)\n\n", stats.Failed, float64(stats.Failed)*100/float64(stats.Total))

	// Print by tool
	fmt.Println("By Tool:")
	for tool, count := range stats.ByTool {
		fmt.Printf("  %-20s %d tests\n", tool, count)
	}
	fmt.Println()

	// Print by category
	fmt.Println("By Category:")
	for category, count := range stats.ByCategory {
		fmt.Printf("  %-20s %d tests\n", category, count)
	}
	fmt.Println()

	// Print failed tests if any
	if stats.Failed > 0 {
		fmt.Println("Failed Tests:")
		fmt.Println(strings.Repeat("-", 80))
		for _, r := range results {
			if !r.Passed {
				fmt.Printf("  ✗ [%s] %s\n", r.Tool, r.Name)
				fmt.Printf("    Error: %v\n", r.Error)
			}
		}
		fmt.Println()
	}

	// Print timing info
	fmt.Println("Timing:")
	fmt.Println(strings.Repeat("-", 80))
	var totalDuration time.Duration
	for _, r := range results {
		totalDuration += r.Duration
	}
	fmt.Printf("  Total Time:     %v\n", totalDuration.Round(time.Millisecond))
	if stats.Total > 0 {
		fmt.Printf("  Average:        %v\n", (totalDuration / time.Duration(stats.Total)).Round(time.Millisecond))
	}
	fmt.Println()

	// Exit code
	if stats.Failed > 0 {
		os.Exit(1)
	}
}
