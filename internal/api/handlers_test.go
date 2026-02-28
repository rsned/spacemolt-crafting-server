package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/rsned/spacemolt-crafting-server/internal/crafting/db"
)

func TestMarketSubmitEndpoint(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer func() { _ = database.Close() }()

	// Initialize schema and migration
	if err := db.InitSchema(ctx, database.DB); err != nil {
		t.Fatalf("initializing schema: %v", err)
	}

	if err := db.ApplyMigration005(ctx, database); err != nil {
		t.Fatalf("applying migration 005: %v", err)
	}

	// Add test items to database
	_, err = database.ExecContext(ctx, `
		INSERT INTO items (id, name, base_value, category) VALUES
			('ore_iron', 'Iron Ore', 1, 'ore'),
			('comp_steel', 'Steel Component', 100, 'component')
	`)
	if err != nil {
		t.Fatalf("inserting test items: %v", err)
	}

	// Create server
	server := NewServer(database, Config{
		Addr:            "127.0.0.1:0",
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	})

	// Start server
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			t.Errorf("server error: %v", err)
		}
	}()

	// Wait for server to be ready
	time.Sleep(200 * time.Millisecond)

	// Cleanup
	defer func() {
		_ = server.Shutdown(ctx)
	}()

	t.Run("valid market data submission", func(t *testing.T) {
		requestBody := MarketSubmitRequest{
			StationID: "Grand Exchange Station",
			Source:    "test_client",
			Orders: []MarketOrder{
				{
					ItemID:         "ore_iron",
					OrderType:       "sell",
					PricePerUnit:    30,
					VolumeAvailable: 128700,
					PlayerStallName:  "TestSeller",
				},
				{
					ItemID:         "ore_iron",
					OrderType:       "sell",
					PricePerUnit:    2,
					VolumeAvailable: 5000,
				},
			},
			SubmittedAt: time.Now().Format(time.RFC3339),
		}

		body, _ := json.Marshal(requestBody)
		resp, err := http.Post(server.URL()+"/api/v1/market/submit", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var response MarketSubmitResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			t.Fatalf("decoding response: %v", err)
		}

		if response.OrdersReceived != 2 {
			t.Errorf("expected 2 orders received, got %d", response.OrdersReceived)
		}
		if response.OrdersAccepted != 2 {
			t.Errorf("expected 2 orders accepted, got %d", response.OrdersAccepted)
		}
		if response.OrdersRejected != 0 {
			t.Errorf("expected 0 orders rejected, got %d", response.OrdersRejected)
		}
		if response.BatchID == "" {
			t.Error("expected non-empty batch ID")
		}
	})

	t.Run("invalid item ID", func(t *testing.T) {
		requestBody := MarketSubmitRequest{
			StationID: "Grand Exchange Station",
			Orders: []MarketOrder{
				{
					ItemID:         "invalid_item_that_does_not_exist",
					OrderType:       "sell",
					PricePerUnit:    10,
					VolumeAvailable: 100,
				},
			},
			SubmittedAt: time.Now().Format(time.RFC3339),
		}

		body, _ := json.Marshal(requestBody)
		resp, err := http.Post(server.URL()+"/api/v1/market/submit", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("duplicate submission is idempotent", func(t *testing.T) {
		requestBody := MarketSubmitRequest{
			StationID: "Grand Exchange Station",
			Orders: []MarketOrder{
				{
					ItemID:         "ore_iron",
					OrderType:       "sell",
					PricePerUnit:    30,
					VolumeAvailable: 100,
				},
			},
			SubmittedAt: time.Now().Format(time.RFC3339),
		}

		body, _ := json.Marshal(requestBody)

		// First submission
		resp1, err := http.Post(server.URL()+"/api/v1/market/submit", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("first POST failed: %v", err)
		}
		func() { _ = resp1.Body.Close() }()

		// Second submission (should be idempotent)
		body2 := make([]byte, len(body))
		copy(body2, body)
		resp2, err := http.Post(server.URL()+"/api/v1/market/submit", "application/json", bytes.NewReader(body2))
		if err != nil {
			t.Fatalf("second POST failed: %v", err)
		}
		defer func() { _ = resp2.Body.Close() }()

		if resp2.StatusCode != http.StatusOK {
			t.Errorf("second submission: expected status 200, got %d", resp2.StatusCode)
		}
	})

	t.Run("query price for item with market data", func(t *testing.T) {
		// First submit some market data
		requestBody := MarketSubmitRequest{
			StationID: "Grand Exchange Station",
			Orders: []MarketOrder{
				{
					ItemID:         "comp_steel",
					OrderType:       "sell",
					PricePerUnit:    120,
					VolumeAvailable: 100,
				},
				{
					ItemID:         "comp_steel",
					OrderType:       "sell",
					PricePerUnit:    130,
					VolumeAvailable: 50,
				},
			},
			SubmittedAt: time.Now().Format(time.RFC3339),
		}

		body, _ := json.Marshal(requestBody)
		resp, err := http.Post(server.URL()+"/api/v1/market/submit", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST failed: %v", err)
		}
		func() { _ = resp.Body.Close() }()

		// Now query the price
		resp, err = http.Get(server.URL() + "/api/v1/market/price/comp_steel")
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var priceResponse struct {
			ItemID     string `json:"item_id"`
			SellPrice  int    `json:"sell_price"`
			BuyPrice   int    `json:"buy_price"`
			MSRP       int    `json:"msrp"`
			MethodName string `json:"method_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&priceResponse); err != nil {
			t.Fatalf("decoding response: %v", err)
		}

		if priceResponse.ItemID != "comp_steel" {
			t.Errorf("expected item_id 'comp_steel', got '%s'", priceResponse.ItemID)
		}
		if priceResponse.SellPrice == 0 {
			t.Error("expected non-zero sell_price")
		}
		if priceResponse.MSRP != 100 {
			t.Errorf("expected MSRP 100, got %d", priceResponse.MSRP)
		}
	})

	t.Run("query price for non-existent item", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/api/v1/market/price/does_not_exist")
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("auto-recalc stats after order submission", func(t *testing.T) {
		// Submit orders for an item
		requestBody := MarketSubmitRequest{
			StationID: "Grand Exchange Station",
			Orders: []MarketOrder{
				{
					ItemID:         "ore_iron",
					OrderType:       "sell",
					PricePerUnit:    25,
					VolumeAvailable: 1000,
				},
				{
					ItemID:         "ore_iron",
					OrderType:       "sell",
					PricePerUnit:    30,
					VolumeAvailable: 2000,
				},
			},
			SubmittedAt: time.Now().Format(time.RFC3339),
		}

		body, _ := json.Marshal(requestBody)
		resp, err := http.Post(server.URL()+"/api/v1/market/submit", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST failed: %v", err)
		}
		func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		// Query the price stats - should have recalculated
		resp, err = http.Get(server.URL() + "/api/v1/market/price/ore_iron")
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var priceResponse struct {
			ItemID     string `json:"item_id"`
			SellPrice  int    `json:"sell_price"`
			BuyPrice   int    `json:"buy_price"`
			MSRP       int    `json:"msrp"`
			MethodName string `json:"method_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&priceResponse); err != nil {
			t.Fatalf("decoding response: %v", err)
		}

		// Should use the submitted orders, not just MSRP
		if priceResponse.MethodName == "msrp_only" {
			t.Error("expected stats to be recalculated from submitted orders, not msrp_only")
		}

		if priceResponse.SellPrice == 0 {
			t.Error("expected non-zero sell price after order submission")
		}
	})
}
