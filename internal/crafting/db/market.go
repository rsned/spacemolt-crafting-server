package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rsned/spacemolt-crafting-server/pkg/crafting"
)

// MarketStore handles market data access.
type MarketStore struct {
	db *DB
}

// NewMarketStore creates a new MarketStore.
func NewMarketStore(db *DB) *MarketStore {
	return &MarketStore{db: db}
}

// MarketDataPoint represents a single price record for import.
type MarketDataPoint struct {
	ComponentID string
	StationID   string
	BuyPrice    int
	SellPrice   int
	Volume24h   int
	Timestamp   time.Time
}

// GetPriceSummary retrieves the price summary for a component at a station.
func (s *MarketStore) GetPriceSummary(ctx context.Context, componentID, stationID string) (*crafting.MarketPriceSummary, *crafting.MarketPriceSummary, error) {
	var buySummary, sellSummary *crafting.MarketPriceSummary
	
	// Get buy summary
	var buy crafting.MarketPriceSummary
	err := s.db.QueryRowContext(ctx, `
		SELECT component_id, station_id, price_type, avg_price_7d, min_price_7d, max_price_7d, price_trend
		FROM market_price_summary
		WHERE component_id = ? AND station_id = ? AND price_type = 'buy'
	`, componentID, stationID).Scan(
		&buy.ComponentID, &buy.StationID, &buy.PriceType,
		&buy.AvgPrice7d, &buy.MinPrice7d, &buy.MaxPrice7d, &buy.PriceTrend,
	)
	if err == nil {
		buySummary = &buy
	} else if err != sql.ErrNoRows {
		return nil, nil, fmt.Errorf("querying buy summary: %w", err)
	}
	
	// Get sell summary
	var sell crafting.MarketPriceSummary
	err = s.db.QueryRowContext(ctx, `
		SELECT component_id, station_id, price_type, avg_price_7d, min_price_7d, max_price_7d, price_trend
		FROM market_price_summary
		WHERE component_id = ? AND station_id = ? AND price_type = 'sell'
	`, componentID, stationID).Scan(
		&sell.ComponentID, &sell.StationID, &sell.PriceType,
		&sell.AvgPrice7d, &sell.MinPrice7d, &sell.MaxPrice7d, &sell.PriceTrend,
	)
	if err == nil {
		sellSummary = &sell
	} else if err != sql.ErrNoRows {
		return nil, nil, fmt.Errorf("querying sell summary: %w", err)
	}
	
	return buySummary, sellSummary, nil
}

// GetSellPrice retrieves the current sell price for a component at a station.
// Returns 0 if not found.
func (s *MarketStore) GetSellPrice(ctx context.Context, componentID, stationID string) (int, error) {
	var price int
	err := s.db.QueryRowContext(ctx, `
		SELECT CAST(avg_price_7d AS INTEGER)
		FROM market_price_summary
		WHERE component_id = ? AND station_id = ? AND price_type = 'sell'
	`, componentID, stationID).Scan(&price)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("querying sell price: %w", err)
	}
	return price, nil
}

// GetBuyPrice retrieves the current buy price for a component at a station.
// Returns 0 if not found.
func (s *MarketStore) GetBuyPrice(ctx context.Context, componentID, stationID string) (int, error) {
	var price int
	err := s.db.QueryRowContext(ctx, `
		SELECT CAST(avg_price_7d AS INTEGER)
		FROM market_price_summary
		WHERE component_id = ? AND station_id = ? AND price_type = 'buy'
	`, componentID, stationID).Scan(&price)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("querying buy price: %w", err)
	}
	return price, nil
}

// GetPriceTrend retrieves the price trend for a component.
func (s *MarketStore) GetPriceTrend(ctx context.Context, componentID, stationID string) (string, error) {
	var trend string
	err := s.db.QueryRowContext(ctx, `
		SELECT price_trend
		FROM market_price_summary
		WHERE component_id = ? AND station_id = ? AND price_type = 'sell'
	`, componentID, stationID).Scan(&trend)
	if err == sql.ErrNoRows {
		return "unknown", nil
	}
	if err != nil {
		return "", fmt.Errorf("querying price trend: %w", err)
	}
	return trend, nil
}

// GetVolume24h retrieves the 24h trading volume for a component.
func (s *MarketStore) GetVolume24h(ctx context.Context, componentID, stationID string) (int, error) {
	var volume int
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(volume_24h, 0)
		FROM market_prices
		WHERE component_id = ? AND station_id = ? AND price_type = 'sell'
		ORDER BY recorded_at DESC
		LIMIT 1
	`, componentID, stationID).Scan(&volume)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("querying volume: %w", err)
	}
	return volume, nil
}

// ImportMarketData imports market price data points.
func (s *MarketStore) ImportMarketData(ctx context.Context, data []MarketDataPoint) error {
	return s.db.InTransaction(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO market_prices 
			(component_id, station_id, price_type, price, volume_24h, recorded_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("preparing statement: %w", err)
		}
		defer func() { _ = stmt.Close() }()
		
		for _, d := range data {
			ts := d.Timestamp.Format(time.RFC3339)
			
			// Insert buy price
			if d.BuyPrice > 0 {
				_, err := stmt.ExecContext(ctx,
					d.ComponentID, d.StationID, "buy", d.BuyPrice, d.Volume24h, ts,
				)
				if err != nil {
					return fmt.Errorf("inserting buy price for %s: %w", d.ComponentID, err)
				}
			}
			
			// Insert sell price
			if d.SellPrice > 0 {
				_, err := stmt.ExecContext(ctx,
					d.ComponentID, d.StationID, "sell", d.SellPrice, d.Volume24h, ts,
				)
				if err != nil {
					return fmt.Errorf("inserting sell price for %s: %w", d.ComponentID, err)
				}
			}
		}
		
		return nil
	})
}

// RefreshPriceSummaries recalculates the price summary table from raw data.
func (s *MarketStore) RefreshPriceSummaries(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO market_price_summary 
		(component_id, station_id, price_type, avg_price_7d, min_price_7d, max_price_7d, price_trend, last_updated)
		SELECT 
			component_id, 
			station_id, 
			price_type,
			AVG(price) as avg_price_7d,
			MIN(price) as min_price_7d,
			MAX(price) as max_price_7d,
			CASE 
				WHEN AVG(CASE WHEN recorded_at > datetime('now', '-1 day') THEN price END) >
					 AVG(CASE WHEN recorded_at <= datetime('now', '-1 day') THEN price END) * 1.05
				THEN 'rising'
				WHEN AVG(CASE WHEN recorded_at > datetime('now', '-1 day') THEN price END) <
					 AVG(CASE WHEN recorded_at <= datetime('now', '-1 day') THEN price END) * 0.95
				THEN 'falling'
				ELSE 'stable'
			END as price_trend,
			datetime('now') as last_updated
		FROM market_prices
		WHERE recorded_at > datetime('now', '-7 days')
		GROUP BY component_id, station_id, price_type
	`)
	if err != nil {
		return fmt.Errorf("refreshing price summaries: %w", err)
	}
	return nil
}

// PruneOldPrices removes price records older than the specified days.
func (s *MarketStore) PruneOldPrices(ctx context.Context, olderThanDays int) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM market_prices 
		WHERE recorded_at < datetime('now', ?)
	`, fmt.Sprintf("-%d days", olderThanDays))
	if err != nil {
		return 0, fmt.Errorf("pruning old prices: %w", err)
	}
	return result.RowsAffected()
}

// ClearMarketData removes all market data.
func (s *MarketStore) ClearMarketData(ctx context.Context) error {
	return s.db.InTransaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM market_prices`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM market_price_summary`); err != nil {
			return err
		}
		return nil
	})
}
