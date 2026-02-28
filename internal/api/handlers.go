package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/rsned/spacemolt-crafting-server/internal/crafting/db"
)

// Config holds server configuration.
type Config struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

// MarketSubmitRequest represents a market data submission.
type MarketSubmitRequest struct {
	StationID  string        `json:"station_id"`
	EmpireID   string        `json:"empire_id,omitempty"`
	Source     string        `json:"source,omitempty"`
	Orders     []MarketOrder `json:"orders"`
	SubmittedAt string        `json:"submitted_at"`
	SubmitterID string        `json:"submitter_id,omitempty"`
}

// MarketOrder represents a single buy or sell order.
type MarketOrder struct {
	ItemID         string `json:"item_id"`
	OrderType       string `json:"order_type"` // "buy" or "sell"
	PricePerUnit    int    `json:"price_per_unit"`
	VolumeAvailable int    `json:"volume_available"`
	PlayerStallName string `json:"player_stall_name,omitempty"`
}

// MarketSubmitResponse represents the response to market data submission.
type MarketSubmitResponse struct {
	BatchID         string   `json:"batch_id"`
	OrdersReceived  int      `json:"orders_received"`
	OrdersAccepted  int      `json:"orders_accepted"`
	OrdersRejected  int      `json:"orders_rejected"`
	Errors          []string `json:"errors,omitempty"`
}

// Server handles HTTP requests for market data API.
type Server struct {
	db     *db.DB
	config Config
	server *http.Server
	addr   string
}

// NewServer creates a new HTTP server.
func NewServer(database *db.DB, cfg Config) *Server {
	return &Server{
		db:     database,
		config: cfg,
	}
}

// URL returns the base URL of the server.
func (s *Server) URL() string {
	if s.addr != "" {
		return "http://" + s.addr
	}
	return ""
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API v1 routes
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/market/submit", s.handleMarketSubmit)
	mux.HandleFunc("/api/v1/market/price/", s.handleMarketPrice)

	listener, err := net.Listen("tcp", s.config.Addr)
	if err != nil {
		return err
	}

	s.addr = listener.Addr().String()

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	return s.server.Serve(listener)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	return s.server.Shutdown(shutdownCtx)
}

// handleHealth returns server health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintln(w, `{"status":"healthy"}`)
}

// handleMarketSubmit processes market data submissions.
func (s *Server) handleMarketSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MarketSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	// Validate request
	if req.StationID == "" {
		http.Error(w, "station_id is required", http.StatusBadRequest)
		return
	}

	response, err := s.processMarketSubmission(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return 400 if any orders were rejected
	statusCode := http.StatusOK
	if response.OrdersRejected > 0 {
		statusCode = http.StatusBadRequest
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

// processMarketSubmission processes a market data submission.
func (s *Server) processMarketSubmission(ctx context.Context, req MarketSubmitRequest) (*MarketSubmitResponse, error) {
	// Generate batch ID
	batchID := fmt.Sprintf("batch_%s", time.Now().Format("20060102_150405"))

	accepted := 0
	rejected := 0
	var errors []string

	for _, order := range req.Orders {
		// Validate item exists
		var exists bool
		err := s.db.QueryRowContext(ctx, `SELECT 1 FROM items WHERE id = ?`, order.ItemID).Scan(&exists)
		if err != nil || !exists {
			rejected++
			errors = append(errors, fmt.Sprintf("item '%s' not found in database", order.ItemID))
			continue
		}

		// Validate order type
		if order.OrderType != "buy" && order.OrderType != "sell" {
			rejected++
			errors = append(errors, fmt.Sprintf("invalid order_type '%s' for item %s", order.OrderType, order.ItemID))
			continue
		}

		// Validate price
		if order.PricePerUnit <= 0 {
			rejected++
			errors = append(errors, fmt.Sprintf("invalid price %d for item %s", order.PricePerUnit, order.ItemID))
			continue
		}

		// Validate volume
		if order.VolumeAvailable <= 0 {
			rejected++
			errors = append(errors, fmt.Sprintf("invalid volume %d for item %s", order.VolumeAvailable, order.ItemID))
			continue
		}

		// Insert into database
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO market_order_book
			(batch_id, item_id, station_id, empire_id, order_type, price_per_unit, volume_available, player_stall_name, recorded_at, submitter_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), ?)
		`, batchID, order.ItemID, req.StationID, req.EmpireID, order.OrderType, order.PricePerUnit, order.VolumeAvailable, order.PlayerStallName, req.SubmitterID)

		if err != nil {
			rejected++
			errors = append(errors, fmt.Sprintf("database error for item %s: %v", order.ItemID, err))
			continue
		}

		accepted++
	}

	// Recalculate market stats for affected items
	uniqueItems := make(map[string]bool)
	for _, order := range req.Orders {
		uniqueItems[order.ItemID] = true
	}

	for itemID := range uniqueItems {
		market := db.NewMarketStore(s.db)
		if err := market.RecalculatePriceStats(ctx, itemID, req.StationID); err != nil {
			// Log error but don't fail the submission
			// The orders are already stored, recalc can be retried later
			errors = append(errors, fmt.Sprintf("warning: failed to recalculate stats for %s: %v", itemID, err))
		}
	}

	return &MarketSubmitResponse{
		BatchID:        batchID,
		OrdersReceived: len(req.Orders),
		OrdersAccepted: accepted,
		OrdersRejected: rejected,
		Errors:         errors,
	}, nil
}

// MarketPriceResponse represents the response to a price query.
type MarketPriceResponse struct {
	ItemID     string `json:"item_id"`
	SellPrice  int    `json:"sell_price"`
	BuyPrice   int    `json:"buy_price"`
	MSRP       int    `json:"msrp"`
	MethodName string `json:"method_name"`
}

// handleMarketPrice processes market price queries.
func (s *Server) handleMarketPrice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract item ID from path
	itemID := r.URL.Path[len("/api/v1/market/price/"):]
	if itemID == "" {
		http.Error(w, "item_id is required", http.StatusBadRequest)
		return
	}

	// Validate item exists
	var exists bool
	err := s.db.QueryRowContext(r.Context(), `SELECT 1 FROM items WHERE id = ?`, itemID).Scan(&exists)
	if err != nil || !exists {
		http.Error(w, "item not found", http.StatusNotFound)
		return
	}

	response, err := s.queryMarketPrice(r.Context(), itemID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// queryMarketPrice queries market price for an item.
func (s *Server) queryMarketPrice(ctx context.Context, itemID string) (*MarketPriceResponse, error) {
	market := db.NewMarketStore(s.db)

	// Get item MSRP
	msrp, err := market.GetItemMSRP(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("querying item MSRP: %w", err)
	}

	// Try to get sell price stats
	sellStats, err := market.GetPriceStats(ctx, itemID, "Grand Exchange Station", "sell")
	if err != nil {
		return nil, fmt.Errorf("querying sell stats: %w", err)
	}

	// Try to get buy price stats
	buyStats, err := market.GetPriceStats(ctx, itemID, "Grand Exchange Station", "buy")
	if err != nil {
		return nil, fmt.Errorf("querying buy stats: %w", err)
	}

	// Use stats if available, otherwise fallback to MSRP
	sellPrice := msrp
	buyPrice := msrp
	methodName := "msrp_only"

	if sellStats != nil {
		sellPrice = sellStats.RepresentativePrice
		methodName = sellStats.StatMethod
	}

	if buyStats != nil {
		buyPrice = buyStats.RepresentativePrice
	}

	response := &MarketPriceResponse{
		ItemID:     itemID,
		SellPrice:  sellPrice,
		BuyPrice:   buyPrice,
		MSRP:       msrp,
		MethodName: methodName,
	}

	return response, nil
}
