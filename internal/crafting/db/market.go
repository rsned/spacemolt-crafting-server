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
	ItemID    string
	StationID string
	BuyPrice  int
	SellPrice int
	Volume24h int
	Timestamp time.Time
}

// GetPriceSummary retrieves the price summary for an item at a station.
func (s *MarketStore) GetPriceSummary(ctx context.Context, itemID, stationID string) (*crafting.MarketPriceSummary, *crafting.MarketPriceSummary, error) {
	var buySummary, sellSummary *crafting.MarketPriceSummary

	// Get buy summary
	var buy crafting.MarketPriceSummary
	err := s.db.QueryRowContext(ctx, `
		SELECT item_id, station_id, price_type, avg_price_7d, min_price_7d, max_price_7d, price_trend
		FROM market_price_summary
		WHERE item_id = ? AND station_id = ? AND price_type = 'buy'
	`, itemID, stationID).Scan(
		&buy.ItemID, &buy.StationID, &buy.PriceType,
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
		SELECT item_id, station_id, price_type, avg_price_7d, min_price_7d, max_price_7d, price_trend
		FROM market_price_summary
		WHERE item_id = ? AND station_id = ? AND price_type = 'sell'
	`, itemID, stationID).Scan(
		&sell.ItemID, &sell.StationID, &sell.PriceType,
		&sell.AvgPrice7d, &sell.MinPrice7d, &sell.MaxPrice7d, &sell.PriceTrend,
	)
	if err == nil {
		sellSummary = &sell
	} else if err != sql.ErrNoRows {
		return nil, nil, fmt.Errorf("querying sell summary: %w", err)
	}

	return buySummary, sellSummary, nil
}

// GetSellPrice retrieves the current sell price for an item at a station.
// Returns 0 if not found.
func (s *MarketStore) GetSellPrice(ctx context.Context, itemID, stationID string) (int, error) {
	var price int
	err := s.db.QueryRowContext(ctx, `
		SELECT CAST(avg_price_7d AS INTEGER)
		FROM market_price_summary
		WHERE item_id = ? AND station_id = ? AND price_type = 'sell'
	`, itemID, stationID).Scan(&price)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("querying sell price: %w", err)
	}
	return price, nil
}

// GetBuyPrice retrieves the current buy price for an item at a station.
// Returns 0 if not found.
func (s *MarketStore) GetBuyPrice(ctx context.Context, itemID, stationID string) (int, error) {
	var price int
	err := s.db.QueryRowContext(ctx, `
		SELECT CAST(avg_price_7d AS INTEGER)
		FROM market_price_summary
		WHERE item_id = ? AND station_id = ? AND price_type = 'buy'
	`, itemID, stationID).Scan(&price)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("querying buy price: %w", err)
	}
	return price, nil
}

// GetPriceTrend retrieves the price trend for an item.
func (s *MarketStore) GetPriceTrend(ctx context.Context, itemID, stationID string) (string, error) {
	var trend string
	err := s.db.QueryRowContext(ctx, `
		SELECT price_trend
		FROM market_price_summary
		WHERE item_id = ? AND station_id = ? AND price_type = 'sell'
	`, itemID, stationID).Scan(&trend)
	if err == sql.ErrNoRows {
		return "unknown", nil
	}
	if err != nil {
		return "", fmt.Errorf("querying price trend: %w", err)
	}
	return trend, nil
}

// GetVolume24h retrieves the 24h trading volume for an item.
func (s *MarketStore) GetVolume24h(ctx context.Context, itemID, stationID string) (int, error) {
	var volume int
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(volume_24h, 0)
		FROM market_prices
		WHERE item_id = ? AND station_id = ? AND price_type = 'sell'
		ORDER BY recorded_at DESC
		LIMIT 1
	`, itemID, stationID).Scan(&volume)
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
			(item_id, station_id, price_type, price, volume_24h, recorded_at)
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
					d.ItemID, d.StationID, "buy", d.BuyPrice, d.Volume24h, ts,
				)
				if err != nil {
					return fmt.Errorf("inserting buy price for %s: %w", d.ItemID, err)
				}
			}

			// Insert sell price
			if d.SellPrice > 0 {
				_, err := stmt.ExecContext(ctx,
					d.ItemID, d.StationID, "sell", d.SellPrice, d.Volume24h, ts,
				)
				if err != nil {
					return fmt.Errorf("inserting sell price for %s: %w", d.ItemID, err)
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
		(item_id, station_id, price_type, avg_price_7d, min_price_7d, max_price_7d, price_trend, last_updated)
		SELECT
			item_id,
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
		GROUP BY item_id, station_id, price_type
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

// MarketPriceStats represents detailed market statistics from market_price_stats table.
type MarketPriceStats struct {
	ItemID             string
	StationID          string
	EmpireID           *string  // Nullable for global stats
	OrderType          string
	RepresentativePrice int
	StatMethod         string
	SampleCount        int
	TotalVolume        int
	MinPrice           int
	MaxPrice           int
	StdDev             *float64 // Nullable
	ConfidenceScore    float64
	PriceTrend         *string  // Nullable
}

// GetPriceStats retrieves market price statistics from the new market_price_stats table.
// Returns nil if not found.
func (s *MarketStore) GetPriceStats(ctx context.Context, itemID, stationID, orderType string) (*MarketPriceStats, error) {
	var stats MarketPriceStats
	err := s.db.QueryRowContext(ctx, `
		SELECT item_id, station_id, empire_id, order_type,
		       representative_price, stat_method, sample_count, total_volume,
		       min_price, max_price, stddev, confidence_score, price_trend
		FROM market_price_stats
		WHERE item_id = ? AND station_id = ? AND order_type = ?
		ORDER BY empire_id NULLS LAST
		LIMIT 1
	`, itemID, stationID, orderType).Scan(
		&stats.ItemID, &stats.StationID, &stats.EmpireID, &stats.OrderType,
		&stats.RepresentativePrice, &stats.StatMethod, &stats.SampleCount, &stats.TotalVolume,
		&stats.MinPrice, &stats.MaxPrice, &stats.StdDev, &stats.ConfidenceScore, &stats.PriceTrend,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying price stats: %w", err)
	}

	return &stats, nil
}

// GetItemMSRP retrieves the base value (MSRP) for an item from the items table.
func (s *MarketStore) GetItemMSRP(ctx context.Context, itemID string) (int, error) {
	var msrp int
	err := s.db.QueryRowContext(ctx, `SELECT base_value FROM items WHERE id = ?`, itemID).Scan(&msrp)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("querying item MSRP: %w", err)
	}
	return msrp, nil
}

// RecalculatePriceStats recalculates market price statistics from the order book.
// Updates market_price_stats table with new computed values.
func (s *MarketStore) RecalculatePriceStats(ctx context.Context, itemID, stationID string) error {
	calc := NewStatsCalculator(s.db)

	// Recalculate for both buy and sell orders
	for _, orderType := range []string{"buy", "sell"} {
		// Fetch orders from order book
		rows, err := s.db.QueryContext(ctx, `
			SELECT price_per_unit, volume_available
			FROM market_order_book
			WHERE item_id = ? AND station_id = ? AND order_type = ?
			ORDER BY recorded_at DESC
		`, itemID, stationID, orderType)
		if err != nil {
			return fmt.Errorf("fetching orders: %w", err)
		}
		defer func() { _ = rows.Close() }()

		// Collect orders
		var orders []any
		var totalVolume int
		for rows.Next() {
			var price, volume int
			if err := rows.Scan(&price, &volume); err != nil {
				return fmt.Errorf("scanning order: %w", err)
			}
			orders = append(orders, Order{
				ItemID:    itemID,
				Price:     price,
				Volume:    volume,
				OrderType: orderType,
			})
			totalVolume += volume
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterating orders: %w", err)
		}

		// Get MSRP for fallback
		msrp, err := s.GetItemMSRP(ctx, itemID)
		if err != nil {
			return fmt.Errorf("getting MSRP: %w", err)
		}

		// Calculate representative price using hybrid method
		var representativePrice int
		var statMethod string
		var sampleCount int
		var confidenceScore float64

		if len(orders) == 0 {
			// No market data, use MSRP
			representativePrice = msrp
			statMethod = "msrp_only"
			sampleCount = 0
			confidenceScore = 0
		} else {
			statMethod = calc.ChoosePricingMethod(len(orders), totalVolume)
			sampleCount = len(orders)

			switch statMethod {
			case "volume_weighted":
				representativePrice = calc.VolumeWeightedAverage(orders)
				confidenceScore = 0.95 // High confidence with large volume
			case "second_price":
				representativePrice = calc.SecondPriceAuction(orders)
				confidenceScore = 0.75 // Medium confidence
			case "median":
				representativePrice = calc.Median(orders)
				confidenceScore = 0.5 // Lower confidence with sparse data
			default:
				representativePrice = msrp
				statMethod = "msrp_only"
				confidenceScore = 0
			}
		}

		// Calculate min/max/stddev
		var minPrice, maxPrice = representativePrice, representativePrice
		var sum, sumSquares float64
		for _, o := range orders {
			if order, ok := o.(Order); ok {
				if order.Price < minPrice {
					minPrice = order.Price
				}
				if order.Price > maxPrice {
					maxPrice = order.Price
				}
				sum += float64(order.Price)
				sumSquares += float64(order.Price * order.Price)
			}
		}

		var stddev *float64
		if len(orders) > 1 {
			mean := sum / float64(len(orders))
			variance := (sumSquares / float64(len(orders))) - (mean * mean)
			if variance > 0 {
				s := float64(variance)
				stddev = &s
			}
		}

		// Upsert to market_price_stats
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO market_price_stats
			(item_id, station_id, empire_id, order_type, stat_method,
			 representative_price, sample_count, total_volume, min_price,
			 max_price, stddev, confidence_score, last_updated)
			VALUES (?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
			ON CONFLICT(item_id, station_id, empire_id, order_type)
			DO UPDATE SET
				stat_method = excluded.stat_method,
				representative_price = excluded.representative_price,
				sample_count = excluded.sample_count,
				total_volume = excluded.total_volume,
				min_price = excluded.min_price,
				max_price = excluded.max_price,
				stddev = excluded.stddev,
				confidence_score = excluded.confidence_score,
				last_updated = excluded.last_updated
		`, itemID, stationID, orderType, statMethod, representativePrice,
			sampleCount, totalVolume, minPrice, maxPrice, stddev, confidenceScore)

		if err != nil {
			return fmt.Errorf("upserting price stats: %w", err)
		}
	}

	return nil
}

// PruneOldOrders removes order book records older than the specified number of days.
// Returns the number of orders deleted.
func (s *MarketStore) PruneOldOrders(ctx context.Context, olderThanDays int) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM market_order_book
		WHERE recorded_at < datetime('now', '-' || ? || ' days')
	`, olderThanDays)
	if err != nil {
		return 0, fmt.Errorf("pruning old orders: %w", err)
	}

	return result.RowsAffected()
}
