package db

import (
	"context"
	"testing"
)

func TestStatsCalculator_VolumeWeightedAverage(t *testing.T) {
	ctx := context.Background()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := InitSchema(ctx, db.DB); err != nil {
		t.Fatalf("initializing schema: %v", err)
	}

	calc := NewStatsCalculator(db)

	// Create test orders with different volumes
	orders := []any{
		Order{ItemID: "ore_iron", Price: 1, Volume: 5000, OrderType: "sell"},   // Large volume at 1cr
		Order{ItemID: "ore_iron", Price: 2, Volume: 100, OrderType: "sell"},    // Small volume at 2cr
		Order{ItemID: "ore_iron", Price: 1, Volume: 10000, OrderType: "sell"},  // Very large volume at 1cr
		Order{ItemID: "ore_iron", Price: 3, Volume: 50, OrderType: "sell"},     // Tiny volume at 3cr
	}

	// Expected: (1*5000 + 2*100 + 1*10000 + 3*50) / (5000 + 100 + 10000 + 50)
	// = (5000 + 200 + 10000 + 150) / 15150
	// = 15350 / 15150
	// = 1.01... ≈ 1
	expectedPrice := 1

	price := calc.VolumeWeightedAverage(orders)

	if price != expectedPrice {
		t.Errorf("expected price %d, got %d", expectedPrice, price)
	}
}

func TestStatsCalculator_SecondPriceAuction(t *testing.T) {
	ctx := context.Background()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := InitSchema(ctx, db.DB); err != nil {
		t.Fatalf("initializing schema: %v", err)
	}

	calc := NewStatsCalculator(db)

	// Create test orders with outliers
	orders := []any{
		Order{ItemID: "comp_steel", Price: 100, Volume: 10, OrderType: "sell"},  // Low outlier (bottom 10%)
		Order{ItemID: "comp_steel", Price: 120, Volume: 20, OrderType: "sell"},
		Order{ItemID: "comp_steel", Price: 125, Volume: 30, OrderType: "sell"},
		Order{ItemID: "comp_steel", Price: 130, Volume: 25, OrderType: "sell"},
		Order{ItemID: "comp_steel", Price: 135, Volume: 15, OrderType: "sell"},
		Order{ItemID: "comp_steel", Price: 140, Volume: 20, OrderType: "sell"},
		Order{ItemID: "comp_steel", Price: 200, Volume: 5, OrderType: "sell"},   // High outlier (top 10%)
	}

	// 7 orders, 7/10 = 0 with integer division, so no trimming
	// Average all: (100 + 120 + 125 + 130 + 135 + 140 + 200) / 7 = 950 / 7 = 135
	expectedPrice := 135

	price := calc.SecondPriceAuction(orders)

	if price != expectedPrice {
		t.Errorf("expected price %d, got %d", expectedPrice, price)
	}
}

func TestStatsCalculator_Median(t *testing.T) {
	ctx := context.Background()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := InitSchema(ctx, db.DB); err != nil {
		t.Fatalf("initializing schema: %v", err)
	}

	calc := NewStatsCalculator(db)

	// Odd number of orders
	ordersOdd := []any{
		Order{ItemID: "item", Price: 100, Volume: 10, OrderType: "sell"},
		Order{ItemID: "item", Price: 110, Volume: 10, OrderType: "sell"},
		Order{ItemID: "item", Price: 120, Volume: 10, OrderType: "sell"},  // Median
		Order{ItemID: "item", Price: 130, Volume: 10, OrderType: "sell"},
		Order{ItemID: "item", Price: 140, Volume: 10, OrderType: "sell"},
	}
	expectedPriceOdd := 120
	priceOdd := calc.Median(ordersOdd)
	if priceOdd != expectedPriceOdd {
		t.Errorf("expected price %d, got %d", expectedPriceOdd, priceOdd)
	}

	// Even number of orders (average of two middle)
	ordersEven := []any{
		Order{ItemID: "item", Price: 100, Volume: 10, OrderType: "sell"},
		Order{ItemID: "item", Price: 110, Volume: 10, OrderType: "sell"},
		Order{ItemID: "item", Price: 120, Volume: 10, OrderType: "sell"},  // Middle
		Order{ItemID: "item", Price: 130, Volume: 10, OrderType: "sell"},  // Middle
	}
	expectedPriceEven := 115 // (110 + 120) / 2
	priceEven := calc.Median(ordersEven)
	if priceEven != expectedPriceEven {
		t.Errorf("expected price %d, got %d", expectedPriceEven, priceEven)
	}
}

func TestStatsCalculator_ChoosePricingMethod(t *testing.T) {
	ctx := context.Background()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := InitSchema(ctx, db.DB); err != nil {
		t.Fatalf("initializing schema: %v", err)
	}

	calc := NewStatsCalculator(db)

	tests := []struct {
		name           string
		sampleCount    int
		totalVolume    int
		expectedMethod string
	}{
		{
			name:           "Very high volume flooded market (ores)",
			sampleCount:    50,
			totalVolume:    50000,
			expectedMethod: "volume_weighted",
		},
		{
			name:           "High volume regardless of orders",
			sampleCount:    5,
			totalVolume:    100000,
			expectedMethod: "volume_weighted",
		},
		{
			name:           "Normal liquidity market",
			sampleCount:    10,
			totalVolume:    100,
			expectedMethod: "second_price",
		},
		{
			name:           "Sparse market (2-3 orders)",
			sampleCount:    3,
			totalVolume:    10,
			expectedMethod: "second_price",
		},
		{
			name:           "Very sparse (2 orders)",
			sampleCount:    2,
			totalVolume:    10,
			expectedMethod: "median",
		},
		{
			name:           "Single order",
			sampleCount:    1,
			totalVolume:    100,
			expectedMethod: "median",
		},
		{
			name:           "No data",
			sampleCount:    0,
			totalVolume:    0,
			expectedMethod: "msrp_only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method := calc.ChoosePricingMethod(tt.sampleCount, tt.totalVolume)
			if method != tt.expectedMethod {
				t.Errorf("expected method %s, got %s", tt.expectedMethod, method)
			}
		})
	}
}
