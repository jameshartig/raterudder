package utility

import (
	"context"
	"time"

	"github.com/raterudder/raterudder/pkg/types"
)

func touUtilityInfo() types.UtilityProviderInfo {
	return types.UtilityProviderInfo{
		ID:   "tou",
		Name: "Time of Use",
		Rates: []types.UtilityRateInfo{
			{
				ID:   "tou_custom",
				Name: "Custom TOU",
			},
		},
	}
}

// BaseTOU is a generic TOU utility provider that relies on the
// AdditionalFeesPeriods from Settings to calculate the current and future
// prices. Base prices are 0.
type BaseTOU struct{}

// GetCurrentPrice returns the current hour with base prices of 0.
func (t *BaseTOU) GetCurrentPrice(ctx context.Context) (types.Price, error) {
	now := time.Now().In(time.Local).Truncate(time.Hour)
	return types.Price{
		Provider: "tou_custom",
		TSStart:  now,
		TSEnd:    now.Add(time.Hour),
	}, nil
}

// GetFuturePrices returns the next 48 hours of base prices of 0.
func (t *BaseTOU) GetFuturePrices(ctx context.Context) ([]types.Price, error) {
	now := time.Now().In(time.Local).Truncate(time.Hour)
	// We map out the next 48 hours (excluding current hour since it's future)
	var prices []types.Price
	for i := 1; i <= 48; i++ {
		ts := now.Add(time.Duration(i) * time.Hour)
		prices = append(prices, types.Price{
			Provider: "tou_custom",
			TSStart:  ts,
			TSEnd:    ts.Add(time.Hour),
		})
	}
	return prices, nil
}

// GetConfirmedPrices returns historical hourly prices of 0.
func (t *BaseTOU) GetConfirmedPrices(ctx context.Context, start, end time.Time) ([]types.Price, error) {
	start = start.Truncate(time.Hour)
	end = end.Truncate(time.Hour)
	var prices []types.Price

	// Loop through each hour from start to end (exclusive)
	for ts := start; ts.Before(end); ts = ts.Add(time.Hour) {
		prices = append(prices, types.Price{
			Provider: "tou_custom",
			TSStart:  ts,
			TSEnd:    ts.Add(time.Hour),
		})
	}
	return prices, nil
}
