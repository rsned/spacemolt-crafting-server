package db

import (
	"sort"
)

// StatsCalculator calculates market price statistics.
type StatsCalculator struct {
	db *DB
}

// NewStatsCalculator creates a new stats calculator.
func NewStatsCalculator(db *DB) *StatsCalculator {
	return &StatsCalculator{db: db}
}

// Order represents a market order for calculation.
type Order struct {
	ItemID    string
	Price     int
	Volume    int
	OrderType string
}

// VolumeWeightedAverage calculates volume-weighted average price.
func (sc *StatsCalculator) VolumeWeightedAverage(orders []any) int {
	if len(orders) == 0 {
		return 0
	}

	var weightedSum int
	var totalVolume int

	for _, o := range orders {
		order, ok := o.(Order)
		if !ok {
			continue
		}
		weightedSum += order.Price * order.Volume
		totalVolume += order.Volume
	}

	if totalVolume == 0 {
		return 0
	}

	return weightedSum / totalVolume
}

// SecondPriceAuction calculates average after trimming top/bottom 10%.
func (sc *StatsCalculator) SecondPriceAuction(orders []any) int {
	if len(orders) == 0 {
		return 0
	}

	// Sort by price
	sorted := make([]Order, 0, len(orders))
	for _, o := range orders {
		if order, ok := o.(Order); ok {
			sorted = append(sorted, order)
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Price < sorted[j].Price
	})

	// Trim top and bottom 10%
	trimCount := len(sorted) / 10
	start := trimCount
	end := len(sorted) - trimCount

	if start >= end {
		// Too few orders to trim
		start = 0
		end = len(sorted)
	}

	sum := 0
	count := 0
	for i := start; i < end; i++ {
		sum += sorted[i].Price
		count++
	}

	if count == 0 {
		return 0
	}

	return sum / count
}

// Median calculates median price.
func (sc *StatsCalculator) Median(orders []any) int {
	if len(orders) == 0 {
		return 0
	}

	// Extract and sort prices
	prices := make([]int, 0, len(orders))
	for _, o := range orders {
		if order, ok := o.(Order); ok {
			prices = append(prices, order.Price)
		}
	}
	sort.Ints(prices)

	n := len(prices)
	if n == 0 {
		return 0
	}

	if n%2 == 1 {
		// Odd: return middle
		return prices[n/2]
	}

	// Even: return average of two middle
	return (prices[n/2-1] + prices[n/2]) / 2
}

// ChoosePricingMethod selects pricing method based on data characteristics.
func (sc *StatsCalculator) ChoosePricingMethod(sampleCount, totalVolume int) string {
	// Volume-weighted: High volume market (10+ orders AND 50K+ volume)
	// OR very high volume regardless of order count
	if (sampleCount >= 10 && totalVolume >= 50000) || totalVolume >= 100000 {
		return "volume_weighted"
	}
	// Second-price auction: Normal liquidity (3+ orders)
	if sampleCount >= 3 {
		return "second_price"
	}
	// Median: Sparse but real data (1+ orders)
	// Single order will just use that order's price as the median
	if sampleCount >= 1 {
		return "median"
	}
	// MSRP fallback: No market data
	return "msrp_only"
}
