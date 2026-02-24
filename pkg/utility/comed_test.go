package utility

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/raterudder/raterudder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComEd(t *testing.T) {
	t.Run("GetCurrentPrice_Parsing", func(t *testing.T) {
		// Mock server returning a sample response
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Return JSON mimicking ComEd 5-min feed
			// Two entries in the same hour: 2.0 and 3.0 -> Average 2.5
			// timestamps: 1706227200000 (00:00), 1706227500000 (00:05)
			response := `[
			{"millisUTC":"1706227500000","price":"2.0"},
			{"millisUTC":"1706227800000","price":"3.0"}
		]`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(response))
		}))
		defer ts.Close()

		c := &BaseComEdHourly{
			apiURL:           ts.URL,
			client:           ts.Client(),
			historicalPrices: make(map[int64]types.Price),
		}

		ctx := context.Background()
		price, err := c.GetCurrentPrice(ctx)
		require.NoError(t, err)

		assert.Equal(t, 0.025, price.DollarsPerKWH) // 2.5 cents = 0.025 dollars

		// Takes timestamp of the hour start
		// 1706227200000 is 2024-01-26 00:00:00 UTC
		// CT is UTC-6 (Standard) or UTC-5 (Daylight). Jan is Standard (UTC-6).
		// So 2024-01-25 18:00:00 CT.
		expectedTime := time.UnixMilli(1706227200000).In(ctLocation)
		assert.Equal(t, expectedTime, price.TSStart)
	})

	t.Run("Caching", func(t *testing.T) {
		requests := 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			_, _ = w.Write([]byte(`[{"millisUTC":"1706227200000","price":"2.0"}]`))
		}))
		defer ts.Close()

		c := &BaseComEdHourly{
			apiURL:           ts.URL,
			client:           ts.Client(),
			historicalPrices: make(map[int64]types.Price),
		}

		// First call
		_, err := c.getCachedCurrentPrices(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, requests)

		// Second call (immediate)
		_, err = c.getCachedCurrentPrices(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, requests, "expected cached response")
	})

	t.Run("GetFuturePrices_NoPJM", func(t *testing.T) {
		c := &BaseComEdHourly{
			apiURL:           "http://example.com", // irrelevant
			client:           &http.Client{},
			historicalPrices: make(map[int64]types.Price),
		}

		prices, err := c.GetFuturePrices(context.Background())
		require.NoError(t, err)
		assert.Nil(t, prices)
	})

	t.Run("GetFuturePrices_PJM_Mock", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/da_hrl_lmps" {
				t.Errorf("expected path /api/v1/da_hrl_lmps, got %s", r.URL.Path)
			}
			if r.Header.Get("Ocp-Apim-Subscription-Key") != "test-key" {
				t.Errorf("missing or wrong api key header")
			}

			// Valid response captured from actual API
			response := `[
				{
					"datetime_beginning_ept": "2026-02-02T00:00:00",
					"total_lmp_da": 34.999970
				},
				{
					"datetime_beginning_ept": "2026-02-02T01:00:00",
					"total_lmp_da": 19.775851
				}
			]`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(response))
		}))
		defer ts.Close()

		c := &BaseComEdHourly{
			pjmAPIKey:        "test-key",
			pjmAPIURL:        ts.URL + "/api/v1/da_hrl_lmps", // Mock server address
			client:           ts.Client(),
			historicalPrices: make(map[int64]types.Price),
		}

		prices, err := c.GetFuturePrices(context.Background())
		require.NoError(t, err)
		require.Len(t, prices, 2)

		// Verification
		// Item 1: 00:00 EPT. 34.999970 $/MWh -> 0.03499997 $/kWh
		// 0.03499997 $/kWh x (1.0124 * 1.0002 * 1.047) -> 0.0371067860737561 $/kWh
		assert.InDelta(t, 0.0371067860737561, prices[0].DollarsPerKWH, 0.0000001)

		// Time check
		// 2026-02-02 00:00:00 EPT (America/New_York)
		loc, _ := time.LoadLocation("America/New_York")
		expectedTime := time.Date(2026, 2, 2, 0, 0, 0, 0, loc)
		assert.Equal(t, expectedTime, prices[0].TSStart)
		expectedTime = time.Date(2026, 2, 2, 1, 0, 0, 0, loc)
		assert.Equal(t, expectedTime, prices[0].TSEnd)
	})

	t.Run("Integration_RealAPI", func(t *testing.T) {
		c := &BaseComEdHourly{
			apiURL:           "https://hourlypricing.comed.com/api?",
			client:           &http.Client{Timeout: 10 * time.Second},
			historicalPrices: make(map[int64]types.Price),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		price, err := c.GetCurrentPrice(ctx)
		require.NoError(t, err)

		// Basic sanity checks
		assert.NotZero(t, price.DollarsPerKWH)
		assert.False(t, price.TSStart.IsZero())
	})

	t.Run("GetConfirmedPrices", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now().UTC()

			makeEntry := func(t time.Time, price string) string {
				ms := t.UnixMilli()
				return fmt.Sprintf(`{"millisUTC":"%d","price":"%s"}`, ms, price)
			}

			var entries []string

			// 1. Valid Past Hour (2 hours ago) - 12 entries
			// 0, 5, 10, ..., 55 minutes past the hour = 12 entries
			validStart := now.Add(-2 * time.Hour).Truncate(time.Hour)
			for i := 0; i < 12; i++ {
				t := validStart.Add(time.Duration(i*5) * time.Minute)
				entries = append(entries, makeEntry(t, "2.0"))
			}

			// 2. Partial Past Hour (3 hours ago) - 11 entries
			// Missing one entry
			partialStart := now.Add(-3 * time.Hour).Truncate(time.Hour)
			for i := 0; i < 11; i++ {
				t := partialStart.Add(time.Duration(i*5) * time.Minute)
				entries = append(entries, makeEntry(t, "3.0"))
			}

			// 3. Future Hour (1 hour ahead) - 12 entries
			// Should be ignored even if full because it's in the future
			futureStart := now.Add(1 * time.Hour).Truncate(time.Hour)
			for i := 0; i < 12; i++ {
				t := futureStart.Add(time.Duration(i*5) * time.Minute)
				entries = append(entries, makeEntry(t, "4.0"))
			}

			jsonStr := fmt.Sprintf("[%s]", strings.Join(entries, ","))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(jsonStr))
		}))
		defer ts.Close()

		c := &BaseComEdHourly{
			apiURL:           ts.URL,
			client:           ts.Client(),
			historicalPrices: make(map[int64]types.Price),
		}

		ctx := context.Background()
		now := time.Now()
		// Request broad range covering everything
		prices, err := c.GetConfirmedPrices(ctx, now.Add(-24*time.Hour), now.Add(24*time.Hour))
		require.NoError(t, err)

		// Assertions:
		// - Future (1h ahead) should be ignored.
		// - Partial (3h ago) should be ignored because < 12 entries.
		// - Valid (2h ago) should be accepted.
		assert.Len(t, prices, 1)
		if len(prices) > 0 {
			assert.InDelta(t, 0.02, prices[0].DollarsPerKWH, 0.0001) // 2.0 cents = 0.02 dollars
			// Ensure it's the valid hour
			assert.Equal(t, now.Add(-2*time.Hour).Truncate(time.Hour).Unix(), prices[0].TSStart.Unix())
		}
	})

	t.Run("ApplyFees", func(t *testing.T) {
		baseTime := time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC)

		tests := []struct {
			name     string
			settings types.Settings
			price    types.Price
			want     types.Price
		}{
			{
				name: "time based fee - match",
				settings: types.Settings{
					UtilityProvider: "comed",
					UtilityRate:     "comed_besh",
					AdditionalFeesPeriods: []types.UtilityAdditionalFeesPeriod{
						{
							UtilityPeriod: types.UtilityPeriod{
								HourStart: 9,
								HourEnd:   17,
							},
							DollarsPerKWH:  0.03,
							GridAdditional: true,
						},
					},
				},
				price: types.Price{
					DollarsPerKWH: 0.05,
					TSStart:       baseTime.Add(6 * time.Hour), // 16:00 UTC => 11:00 CDT (Window 09-17)
				},
				want: types.Price{
					DollarsPerKWH:         0.05,
					GridAddlDollarsPerKWH: 0.03, // just fees
					TSStart:               baseTime,
				},
			},
			{
				name: "time based fee - no match",
				settings: types.Settings{
					UtilityProvider: "comed",
					UtilityRate:     "comed_besh",
					AdditionalFeesPeriods: []types.UtilityAdditionalFeesPeriod{
						{
							UtilityPeriod: types.UtilityPeriod{
								HourStart: 12,
								HourEnd:   17,
							},
							DollarsPerKWH:  0.03,
							GridAdditional: true,
						},
					},
				},
				price: types.Price{
					DollarsPerKWH: 0.05,
					TSStart:       baseTime, // 10:00 UTC
				},
				want: types.Price{
					DollarsPerKWH:         0.05,
					GridAddlDollarsPerKWH: 0.00,
					TSStart:               baseTime,
				},
			},
			{
				name: "base price modification",
				settings: types.Settings{
					UtilityProvider: "comed",
					UtilityRate:     "comed_besh",
					AdditionalFeesPeriods: []types.UtilityAdditionalFeesPeriod{
						{
							UtilityPeriod: types.UtilityPeriod{
								HourStart: 0,
								HourEnd:   24,
							},
							DollarsPerKWH:  0.01,
							GridAdditional: false, // Modifies base price
						},
					},
				},
				price: types.Price{
					DollarsPerKWH: 0.05,
					TSStart:       baseTime,
				},
				want: types.Price{
					DollarsPerKWH:         0.06,
					GridAddlDollarsPerKWH: 0.00,
					TSStart:               baseTime,
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := &SiteComEd{
					settings: tt.settings,
				}
				got, err := s.applyFees(tt.price)
				require.NoError(t, err)
				assert.InDelta(t, tt.want.DollarsPerKWH, got.DollarsPerKWH, 0.0001)
				assert.InDelta(t, tt.want.GridAddlDollarsPerKWH, got.GridAddlDollarsPerKWH, 0.0001)
			})
		}
	})
}
