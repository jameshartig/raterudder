package utility

import (
	"context"
	"testing"
	"time"

	"github.com/raterudder/raterudder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTOUUtility(t *testing.T) {
	ctx := context.Background()
	m := Configured()

	settings := types.Settings{
		UtilityProvider: "tou",
		UtilityRate:     "tou_custom",
		AdditionalFeesPeriods: []types.UtilityAdditionalFeesPeriod{
			{
				UtilityPeriod: types.UtilityPeriod{
					HourStart: 0,
					HourEnd:   24,
				},
				DollarsPerKWH: 0.10,
				Description:   "Base Rate",
			},
			{
				UtilityPeriod: types.UtilityPeriod{
					HourStart: 14,
					HourEnd:   19, // 2pm to 7pm
				},
				DollarsPerKWH: 0.15, // Extra 0.15 during peak
				Description:   "Peak Rate",
			},
		},
	}

	u, err := m.Site(ctx, "site1", settings)
	require.NoError(t, err)

	// Test GetCurrentPrice
	p, err := u.GetCurrentPrice(ctx)
	require.NoError(t, err)
	assert.Equal(t, "tou_custom", p.Provider)

	now := time.Now().In(time.Local).Truncate(time.Hour)
	assert.Equal(t, now, p.TSStart)

	h := now.In(ctLocation).Hour()
	expectedPrice := 0.10
	if h >= 14 && h < 19 {
		expectedPrice += 0.15
	}
	assert.Equal(t, expectedPrice, p.DollarsPerKWH)

	// Test GetFuturePrices
	future, err := u.GetFuturePrices(ctx)
	require.NoError(t, err)
	assert.Len(t, future, 48)

	for i, fp := range future {
		ts := now.Add(time.Duration(i+1) * time.Hour)
		assert.Equal(t, ts, fp.TSStart)

		fh := ts.In(ctLocation).Hour()
		expectedPrice := 0.10
		if fh >= 14 && fh < 19 {
			expectedPrice += 0.15
		}
		assert.Equal(t, expectedPrice, fp.DollarsPerKWH)
	}

	// Test GetConfirmedPrices
	start := now.Add(-24 * time.Hour)
	end := now
	confirmed, err := u.GetConfirmedPrices(ctx, start, end)
	require.NoError(t, err)
	assert.Len(t, confirmed, 24)

	for i, cp := range confirmed {
		ts := start.Add(time.Duration(i) * time.Hour)
		assert.Equal(t, ts, cp.TSStart)

		ch := ts.In(ctLocation).Hour()
		expectedPrice := 0.10
		if ch >= 14 && ch < 19 {
			expectedPrice += 0.15
		}
		assert.Equal(t, expectedPrice, cp.DollarsPerKWH)
	}
}
