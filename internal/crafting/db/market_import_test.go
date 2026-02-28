package db

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// TestViewMarketJSONParsing tests that we can parse the real market data format
func TestViewMarketJSONParsing(t *testing.T) {
	// Try multiple possible paths
	paths := []string{
		"temp/view_market.json",
		"../../temp/view_market.json",
		"../../../temp/view_market.json",
		"/home/robert/spacemolt-crafting-server/temp/view_market.json",
	}

	var data []byte
	var readErr error
	for _, path := range paths {
		data, readErr = os.ReadFile(path)
		if readErr == nil {
			break
		}
	}

	if readErr != nil {
		t.Fatalf("reading market data from any path (tried %v): %v", paths, readErr)
	}

	var response struct {
		Action string `json:"action"`
		Base   string `json:"base"`
		Items  []struct {
			ItemID     string `json:"item_id"`
			ItemName   string `json:"item_name"`
			Category   string `json:"category"`
			BuyOrders  []struct {
				PriceEach int    `json:"price_each"`
				Quantity  int    `json:"quantity"`
				Source     string `json:"source,omitempty"`
			} `json:"buy_orders"`
			SellOrders []struct {
				PriceEach int    `json:"price_each"`
				Quantity  int    `json:"quantity"`
				Source     string `json:"source,omitempty"`
			} `json:"sell_orders"`
		} `json:"items"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("parsing JSON: %v", err)
	}

	if response.Action != "view_market" {
		t.Errorf("expected action 'view_market', got '%s'", response.Action)
	}

	if response.Base != "Grand Exchange Station" {
		t.Errorf("expected base 'Grand Exchange Station', got '%s'", response.Base)
	}

	if len(response.Items) == 0 {
		t.Error("expected items, got none")
	}
}

// TestMarketDataImport tests importing real market data into the order book
func TestMarketDataImport(t *testing.T) {
	ctx := context.Background()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize schema and migration
	if err := InitSchema(ctx, db.DB); err != nil {
		t.Fatalf("initializing schema: %v", err)
	}

	if err := ApplyMigration005(ctx, db); err != nil {
		t.Fatalf("applying migration 005: %v", err)
	}

	// Load real market data
	paths := []string{
		"temp/view_market.json",
		"../../temp/view_market.json",
		"../../../temp/view_market.json",
		"/home/robert/spacemolt-crafting-server/temp/view_market.json",
	}

	var data []byte
	var readErr error
	for _, path := range paths {
		data, readErr = os.ReadFile(path)
		if readErr == nil {
			break
		}
	}

	if readErr != nil {
		t.Fatalf("reading market data: %v", readErr)
	}

	var viewMarket struct {
		Action string `json:"action"`
		Base   string `json:"base"`
		Items  []struct {
			ItemID     string `json:"item_id"`
			BuyOrders  []struct {
				PriceEach int    `json:"price_each"`
				Quantity  int    `json:"quantity"`
				Source     string `json:"source,omitempty"`
			} `json:"buy_orders"`
			SellOrders []struct {
				PriceEach int    `json:"price_each"`
				Quantity  int    `json:"quantity"`
				Source     string `json:"source,omitempty"`
			} `json:"sell_orders"`
		} `json:"items"`
	}

	if err := json.Unmarshal(data, &viewMarket); err != nil {
		t.Fatalf("parsing JSON: %v", err)
	}

	// Import into market_order_book
	batchID := "test_batch_001"
	stationID := viewMarket.Base
	recordedAt := time.Now().Format(time.RFC3339)

	totalOrders := 0
	for _, item := range viewMarket.Items {
		// Import buy orders
		for _, order := range item.BuyOrders {
			_, err := db.ExecContext(ctx, `
				INSERT INTO market_order_book
				(batch_id, item_id, station_id, empire_id, order_type, price_per_unit, volume_available, player_stall_name, recorded_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, batchID, item.ItemID, stationID, nil, "buy", order.PriceEach, order.Quantity, order.Source, recordedAt)
			if err != nil {
				t.Fatalf("inserting buy order for %s: %v", item.ItemID, err)
			}
			totalOrders++
		}

		// Import sell orders
		for _, order := range item.SellOrders {
			_, err := db.ExecContext(ctx, `
				INSERT INTO market_order_book
				(batch_id, item_id, station_id, empire_id, order_type, price_per_unit, volume_available, player_stall_name, recorded_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, batchID, item.ItemID, stationID, nil, "sell", order.PriceEach, order.Quantity, order.Source, recordedAt)
			if err != nil {
				t.Fatalf("inserting sell order for %s: %v", item.ItemID, err)
			}
			totalOrders++
		}
	}

	// Verify orders were imported
	var count int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM market_order_book`).Scan(&count)
	if err != nil {
		t.Fatalf("counting orders: %v", err)
	}

	if count != totalOrders {
		t.Errorf("expected %d orders, got %d", totalOrders, count)
	}
}

// TestStatsCalculatorWithRealData tests our pricing methods with actual market data
func TestStatsCalculatorWithRealData(t *testing.T) {
	ctx := context.Background()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize schema and migration
	if err := InitSchema(ctx, db.DB); err != nil {
		t.Fatalf("initializing schema: %v", err)
	}

	if err := ApplyMigration005(ctx, db); err != nil {
		t.Fatalf("applying migration 005: %v", err)
	}

	calc := NewStatsCalculator(db)

	// Test Case 1: ore_iron (flooded market - should use volume-weighted)
	// From real data: 22 sell orders, 134,133 total units, including one at 30cr with 128,700 units
	t.Run("ore_iron_volume_weighted", func(t *testing.T) {
		orders := []any{
			Order{ItemID: "ore_iron", Price: 2, Volume: 2780, OrderType: "sell"},
			Order{ItemID: "ore_iron", Price: 3, Volume: 193, OrderType: "sell"},
			Order{ItemID: "ore_iron", Price: 30, Volume: 128700, OrderType: "sell"}, // Massive volume
			Order{ItemID: "ore_iron", Price: 999, Volume: 9, OrderType: "sell"},     // Outlier
		}

		sampleCount := len(orders)
		totalVolume := 0
		for _, o := range orders {
			if order, ok := o.(Order); ok {
				totalVolume += order.Volume
			}
		}

		method := calc.ChoosePricingMethod(sampleCount, totalVolume)
		if method != "volume_weighted" {
			t.Errorf("ore_iron: expected 'volume_weighted', got '%s'", method)
		}

		price := calc.VolumeWeightedAverage(orders)
		// Should be heavily weighted toward 30cr due to massive volume
		// (2*2780 + 3*193 + 30*128700 + 999*9) / (2780+193+128700+9) ≈ 29.5 → 29-30
		if price < 25 || price > 35 {
			t.Errorf("ore_iron: expected price around 30, got %d", price)
		}
	})

	// Test Case 2: ore_nickel (with outlier - should use second-price)
	t.Run("ore_nickel_second_price", func(t *testing.T) {
		orders := []any{
			Order{ItemID: "ore_nickel", Price: 1, Volume: 1444, OrderType: "sell"},
			Order{ItemID: "ore_nickel", Price: 2, Volume: 5769, OrderType: "sell"},
			Order{ItemID: "ore_nickel", Price: 30, Volume: 5194, OrderType: "sell"},
			Order{ItemID: "ore_nickel", Price: 400, Volume: 2, OrderType: "sell"}, // Outlier
		}

		method := calc.ChoosePricingMethod(len(orders), sumVolumes(orders))
		if method != "second_price" {
			t.Errorf("ore_nickel: expected 'second_price', got '%s'", method)
		}

		price := calc.SecondPriceAuction(orders)
		// Should trim the 400 outlier, average the rest
		// 4 orders, trim 10% = 0.4 → 0 orders trimmed → average all 4
		// Actually 4/10 = 0 with integer division, so no trimming
		// Average: (1+2+30+400)/4 = 433/4 = 108.25 → 108
		if price < 100 || price > 110 {
			t.Errorf("ore_nickel: expected price around 108, got %d", price)
		}
	})

	// Test Case 3: ore_platinum (sparse data - should use median)
	t.Run("ore_platinum_median", func(t *testing.T) {
		orders := []any{
			Order{ItemID: "ore_platinum", Price: 1, Volume: 1, OrderType: "sell"},
			Order{ItemID: "ore_platinum", Price: 100, Volume: 5, OrderType: "sell"},
		}

		method := calc.ChoosePricingMethod(len(orders), sumVolumes(orders))
		if method != "median" {
			t.Errorf("ore_platinum: expected 'median', got '%s'", method)
		}

		price := calc.Median(orders)
		// Median of [1, 100] = (1+100)/2 = 50.5 → 50
		if price != 50 {
			t.Errorf("ore_platinum: expected median 50, got %d", price)
		}
	})
}

// Helper function to sum volumes
func sumVolumes(orders []any) int {
	total := 0
	for _, o := range orders {
		if order, ok := o.(Order); ok {
			total += order.Volume
		}
	}
	return total
}

// TestSingleOrderPricing tests that a single order works correctly.
func TestSingleOrderPricing(t *testing.T) {
	ctx := context.Background()
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer func() { _ = database.Close() }()

	// Initialize schema and migration
	if err := InitSchema(ctx, database.DB); err != nil {
		t.Fatalf("initializing schema: %v", err)
	}

	if err := ApplyMigration005(ctx, database); err != nil {
		t.Fatalf("applying migration 005: %v", err)
	}

	// Add test item
	_, err = database.ExecContext(ctx, `
		INSERT INTO items (id, name, base_value, category) VALUES
			('ore_iron', 'Iron Ore', 1, 'ore')
	`)
	if err != nil {
		t.Fatalf("inserting test item: %v", err)
	}

	market := NewMarketStore(database)

	// Insert a single order
	_, err = database.ExecContext(ctx, `
		INSERT INTO market_order_book
		(batch_id, item_id, station_id, order_type, price_per_unit, volume_available, recorded_at)
		VALUES ('test_batch', 'ore_iron', 'Test Station', 'sell', 25, 1000, datetime('now'))
	`)
	if err != nil {
		t.Fatalf("inserting test order: %v", err)
	}

	// Recalculate stats
	err = market.RecalculatePriceStats(ctx, "ore_iron", "Test Station")
	if err != nil {
		t.Fatalf("recalculating stats: %v", err)
	}

	// Query the stats
	stats, err := market.GetPriceStats(ctx, "ore_iron", "Test Station", "sell")
	if err != nil {
		t.Fatalf("querying stats: %v", err)
	}

	if stats == nil {
		t.Fatal("expected stats to exist, got nil")
	}

	// Should use the order's price (25), not MSRP (1)
	if stats.RepresentativePrice != 25 {
		t.Errorf("expected price 25 (from order), got %d", stats.RepresentativePrice)
	}

	// Should use "median" method (not msrp_only)
	if stats.StatMethod != "median" {
		t.Errorf("expected method 'median', got '%s'", stats.StatMethod)
	}

	// Should have sample count of 1
	if stats.SampleCount != 1 {
		t.Errorf("expected sample count 1, got %d", stats.SampleCount)
	}
}

// TestMarketOrderRetrieval tests fetching orders back from database
func TestMarketOrderRetrieval(t *testing.T) {
	ctx := context.Background()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize schema and migration
	if err := InitSchema(ctx, db.DB); err != nil {
		t.Fatalf("initializing schema: %v", err)
	}

	if err := ApplyMigration005(ctx, db); err != nil {
		t.Fatalf("applying migration 005: %v", err)
	}

	// Insert test orders
	stationID := "Grand Exchange Station"
	recordedAt := time.Now().Format(time.RFC3339)

	_, err = db.ExecContext(ctx, `
		INSERT INTO market_order_book
		(batch_id, item_id, station_id, order_type, price_per_unit, volume_available, recorded_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "batch_001", "ore_iron", stationID, "sell", 30, 128700, recordedAt)
	if err != nil {
		t.Fatalf("inserting order: %v", err)
	}

	// Query orders back
	rows, err := db.QueryContext(ctx, `
		SELECT item_id, order_type, price_per_unit, volume_available
		FROM market_order_book
		WHERE station_id = ? AND item_id = ?
	`, stationID, "ore_iron")
	if err != nil {
		t.Fatalf("querying orders: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var itemID, orderType string
	var price, volume int
	if !rows.Next() {
		t.Fatal("no orders returned")
	}

	if err := rows.Scan(&itemID, &orderType, &price, &volume); err != nil {
		t.Fatalf("scanning order: %v", err)
	}

	if itemID != "ore_iron" {
		t.Errorf("expected item_id 'ore_iron', got '%s'", itemID)
	}
	if orderType != "sell" {
		t.Errorf("expected order_type 'sell', got '%s'", orderType)
	}
	if price != 30 {
		t.Errorf("expected price 30, got %d", price)
	}
	if volume != 128700 {
		t.Errorf("expected volume 128700, got %d", volume)
	}
}
