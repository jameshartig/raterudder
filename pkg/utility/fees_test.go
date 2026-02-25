package utility

import (
	"context"
	"testing"
	"time"

	"github.com/raterudder/raterudder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockUtilityPrices is a mock implementation of the UtilityPrices interface.
type mockUtilityPrices struct {
	mock.Mock
}

func (m *mockUtilityPrices) GetCurrentPrice(ctx context.Context) (types.Price, error) {
	args := m.Called(ctx)
	return args.Get(0).(types.Price), args.Error(1)
}

func (m *mockUtilityPrices) GetFuturePrices(ctx context.Context) ([]types.Price, error) {
	args := m.Called(ctx)
	return args.Get(0).([]types.Price), args.Error(1)
}

func (m *mockUtilityPrices) GetConfirmedPrices(ctx context.Context, start, end time.Time) ([]types.Price, error) {
	args := m.Called(ctx, start, end)
	return args.Get(0).([]types.Price), args.Error(1)
}

func TestSiteFees(t *testing.T) {
	ctx := context.Background()
	now := time.Now().In(ctLocation).Truncate(time.Hour)

	t.Run("ApplySettings defaults", func(t *testing.T) {
		t.Run("comed", func(t *testing.T) {
			s := &SiteFees{}
			settings := types.Settings{
				UtilityProvider: "comed",
				UtilityRate:     "comed_besh",
				UtilityRateOptions: types.UtilityRateOptions{
					RateClass: ComEdRateClassSingleFamilyResidenceWithoutElectricSpaceHeat,
				},
			}
			err := s.ApplySettings(ctx, settings)
			require.NoError(t, err)
			assert.NotEmpty(t, s.periods)
		})

		t.Run("ameren", func(t *testing.T) {
			s := &SiteFees{}
			settings := types.Settings{
				UtilityProvider: "ameren",
				UtilityRate:     "ameren_psp",
			}
			err := s.ApplySettings(ctx, settings)
			require.NoError(t, err)
			// ameren currently returns nil periods but should not error
		})

		t.Run("invalid provider", func(t *testing.T) {
			s := &SiteFees{}
			settings := types.Settings{
				UtilityProvider: "invalid",
			}
			err := s.ApplySettings(ctx, settings)
			assert.Error(t, err)
		})
	})

	t.Run("ApplySettings custom periods", func(t *testing.T) {
		s := &SiteFees{}
		periods := []types.UtilityAdditionalFeesPeriod{
			{
				Description:   "Custom Fee",
				DollarsPerKWH: 0.05,
			},
		}
		settings := types.Settings{
			AdditionalFeesPeriods: periods,
		}
		err := s.ApplySettings(ctx, settings)
		require.NoError(t, err)
		assert.Equal(t, periods, s.periods)
	})

	t.Run("applyFees logic", func(t *testing.T) {
		periods := []types.UtilityAdditionalFeesPeriod{
			{
				UtilityPeriod: types.UtilityPeriod{
					HourStart: 14,
					HourEnd:   18,
				},
				DollarsPerKWH: 0.10,
				Description:   "Peak Fee",
			},
			{
				UtilityPeriod: types.UtilityPeriod{
					// End is exclusive: summer ends at the start of Sept 1, so Aug 31 is the last covered day.
					Start:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
					End:     time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
					HourEnd: 24,
				},
				DollarsPerKWH: 0.05,
				Description:   "Summer Fee",
			},
			{
				UtilityPeriod: types.UtilityPeriod{
					HourStart: 0,
					HourEnd:   24,
				},
				DollarsPerKWH:  0.02,
				GridAdditional: true,
				Description:    "Grid Fee",
			},
		}
		s := &SiteFees{periods: periods}

		t.Run("peak hour in summer", func(t *testing.T) {
			p := types.Price{
				TSStart:       time.Date(2026, 7, 1, 15, 0, 0, 0, ctLocation),
				DollarsPerKWH: 0.10,
			}
			result, err := s.applyFees(p)
			require.NoError(t, err)
			// Base (0.10) + Peak (0.10) + Summer (0.05) = 0.25
			assert.InDelta(t, 0.25, result.DollarsPerKWH, 0.0001)
			// Grid Fee (0.02)
			assert.InDelta(t, 0.02, result.GridUseDollarsPerKWH, 0.0001)
		})

		t.Run("off-peak in winter", func(t *testing.T) {
			p := types.Price{
				TSStart:       time.Date(2026, 1, 1, 10, 0, 0, 0, ctLocation),
				DollarsPerKWH: 0.10,
			}
			result, err := s.applyFees(p)
			require.NoError(t, err)
			// Base (0.10) + 0 = 0.10
			assert.InDelta(t, 0.10, result.DollarsPerKWH, 0.0001)
			// Grid Fee (0.02)
			assert.InDelta(t, 0.02, result.GridUseDollarsPerKWH, 0.0001)
		})

		t.Run("boundary checks", func(t *testing.T) {
			// Exactly at start of peak (14:00) - inclusive
			p1, _ := s.applyFees(types.Price{
				TSStart:       time.Date(2026, 1, 1, 14, 0, 0, 0, ctLocation),
				DollarsPerKWH: 0.10,
			})
			assert.InDelta(t, 0.20, p1.DollarsPerKWH, 0.0001)

			// Exactly at HourEnd of peak (18:00) - exclusive
			p2, _ := s.applyFees(types.Price{
				TSStart:       time.Date(2026, 1, 1, 18, 0, 0, 0, ctLocation),
				DollarsPerKWH: 0.10,
			})
			assert.InDelta(t, 0.10, p2.DollarsPerKWH, 0.0001)
		})

		t.Run("End is exclusive", func(t *testing.T) {
			// Use a dedicated SiteFees with a single date-bounded period to isolate period.End behavior.
			endTime := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
			boundedS := &SiteFees{
				periods: []types.UtilityAdditionalFeesPeriod{
					{
						UtilityPeriod: types.UtilityPeriod{
							Start:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
							End:     endTime, // exclusive
							HourEnd: 24,
						},
						DollarsPerKWH: 0.05,
						Description:   "Bounded Fee",
					},
				},
			}

			// One hour before End - fee should apply
			beforeEnd, _ := boundedS.applyFees(types.Price{
				TSStart:       endTime.Add(-time.Hour), // Aug 31 23:00 UTC
				DollarsPerKWH: 0.10,
			})
			assert.InDelta(t, 0.15, beforeEnd.DollarsPerKWH, 0.0001, "price one hour before End should have fee applied")

			// Exactly at End - fee should NOT apply (End is exclusive)
			atEnd, _ := boundedS.applyFees(types.Price{
				TSStart:       endTime, // Sept 1 00:00 UTC
				DollarsPerKWH: 0.10,
			})
			assert.InDelta(t, 0.10, atEnd.DollarsPerKWH, 0.0001, "price exactly at End should NOT have fee applied")

			// One hour after End - fee should NOT apply
			afterEnd, _ := boundedS.applyFees(types.Price{
				TSStart:       endTime.Add(time.Hour), // Sept 1 01:00 UTC
				DollarsPerKWH: 0.10,
			})
			assert.InDelta(t, 0.10, afterEnd.DollarsPerKWH, 0.0001, "price after End should NOT have fee applied")
		})
	})

	t.Run("GetConfirmedPrices", func(t *testing.T) {
		m := new(mockUtilityPrices)
		s := &SiteFees{
			base: m,
			periods: []types.UtilityAdditionalFeesPeriod{
				{
					UtilityPeriod: types.UtilityPeriod{HourStart: 0, HourEnd: 24},
					DollarsPerKWH: 0.01,
				},
			},
		}

		start := now.Add(-2 * time.Hour)
		end := now
		basePrices := []types.Price{
			{TSStart: start, DollarsPerKWH: 0.10},
			{TSStart: start.Add(time.Hour), DollarsPerKWH: 0.20},
		}
		m.On("GetConfirmedPrices", ctx, start, end).Return(basePrices, nil)

		prices, err := s.GetConfirmedPrices(ctx, start, end)
		require.NoError(t, err)
		assert.Equal(t, 2, len(prices))
		assert.InDelta(t, 0.11, prices[0].DollarsPerKWH, 0.0001)
		assert.InDelta(t, 0.21, prices[1].DollarsPerKWH, 0.0001)
		m.AssertExpectations(t)
	})

	t.Run("GetCurrentPrice", func(t *testing.T) {
		m := new(mockUtilityPrices)
		s := &SiteFees{
			base:    m,
			periods: []types.UtilityAdditionalFeesPeriod{{UtilityPeriod: types.UtilityPeriod{HourStart: 0, HourEnd: 24}, DollarsPerKWH: 0.01}},
		}

		basePrice := types.Price{TSStart: now, DollarsPerKWH: 0.50}
		m.On("GetCurrentPrice", ctx).Return(basePrice, nil)

		price, err := s.GetCurrentPrice(ctx)
		require.NoError(t, err)
		assert.InDelta(t, 0.51, price.DollarsPerKWH, 0.0001)
		m.AssertExpectations(t)
	})

	t.Run("GetFuturePrices", func(t *testing.T) {
		m := new(mockUtilityPrices)
		s := &SiteFees{
			base:    m,
			periods: []types.UtilityAdditionalFeesPeriod{{UtilityPeriod: types.UtilityPeriod{HourStart: 0, HourEnd: 24}, DollarsPerKWH: 0.01}},
		}

		basePrices := []types.Price{
			{TSStart: now.Add(time.Hour), DollarsPerKWH: 0.30},
		}
		m.On("GetFuturePrices", ctx).Return(basePrices, nil)

		prices, err := s.GetFuturePrices(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, len(prices))
		assert.InDelta(t, 0.31, prices[0].DollarsPerKWH, 0.0001)
		m.AssertExpectations(t)
	})
}
