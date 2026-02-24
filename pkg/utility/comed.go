package utility

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/levenlabs/go-lflag"
	"github.com/raterudder/raterudder/pkg/common"
	"github.com/raterudder/raterudder/pkg/log"
	"github.com/raterudder/raterudder/pkg/types"
)

var (
	// PJM uses Eastern Time
	etLocation = func() *time.Location {
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			panic(fmt.Errorf("failed to load eastern time location: %w", err))
		}
		return loc
	}()

	// ComEd uses Central Time
	ctLocation = func() *time.Location {
		loc, err := time.LoadLocation("America/Chicago")
		if err != nil {
			panic(fmt.Errorf("failed to load central time location: %w", err))
		}
		return loc
	}()
)

const (
	ComEdRateClassSingleFamilyResidenceWithoutElectricSpaceHeat = "singleFamilyWithoutElectricHeat"
	ComEdRateClassMultiFamilyResidenceWithoutElectricSpaceHeat  = "multiFamilyWithoutElectricHeat"
	ComEdRateClassSingleFamilyResidenceWithElectricSpaceHeat    = "singleFamilyElectricHeat"
	ComEdRateClassMultiFamilyResidenceWithElectricSpaceHeat     = "multiFamilyElectricHeat"
)

// ComEdUtilityInfo returns metadata about ComEd and its supported rate plans.
func ComEdUtilityInfo() types.UtilityProviderInfo {
	return types.UtilityProviderInfo{
		ID:   "comed",
		Name: "ComEd",
		Rates: []types.UtilityRateInfo{
			{
				ID:   "comed_besh",
				Name: "Hourly Pricing Program (BESH)",
				Options: []types.UtilityRateOption{
					{
						Field: "rateClass",
						Name:  "Rate Class",
						Type:  types.UtilityOptionTypeSelect,
						Choices: []types.UtilityOptionChoice{
							{Value: ComEdRateClassSingleFamilyResidenceWithoutElectricSpaceHeat, Name: "Residential Single Family Without Electric Space Heat"},
							{Value: ComEdRateClassMultiFamilyResidenceWithoutElectricSpaceHeat, Name: "Residential Multi Family Without Electric Space Heat"},
							{Value: ComEdRateClassSingleFamilyResidenceWithElectricSpaceHeat, Name: "Residential Single Family With Electric Space Heat"},
							{Value: ComEdRateClassMultiFamilyResidenceWithElectricSpaceHeat, Name: "Residential Multi Family With Electric Space Heat"},
						},
						Default: ComEdRateClassSingleFamilyResidenceWithoutElectricSpaceHeat,
					},
					{
						Field:       "variableDeliveryRate",
						Name:        "Delivery Time-of-Day (DTOD)",
						Type:        types.UtilityOptionTypeSwitch,
						Description: "Enable if you are enrolled in ComEd's Delivery Time-of-Day pricing. 30%-47% cheaper than fixed delivery rates in off-peak hours but 2x more expensive in on-peak hours (1pm-7pm).",
						Default:     false,
					},
					{
						Field:       "netMetering",
						Name:        "Pre-2025 Full Net Metering",
						Type:        types.UtilityOptionTypeSwitch,
						Description: "Enable if you are grandfathered into ComEd's pre-2025 full net metering program. You are credited for your supply and delivery charges at the full retail rate.",
						Default:     false,
					},
				},
			},
		},
	}
}

const pjmComedPNodeID = "33092371"

// BaseComEdHourly implements the UtilityPrices interface for ComEd Hourly Energy Pricing (BESH).
type BaseComEdHourly struct {
	apiURL    string
	pjmAPIKey string
	pjmAPIURL string
	client    *http.Client

	mu               sync.Mutex
	lastFetchTime    time.Time
	cachedPrices     []types.Price
	lastFutureFetch  time.Time
	cachedFuture     []types.Price
	historicalPrices map[int64]types.Price // Cache for historical prices (key: unix timestamp of start)
}

// configuredComEd sets up flags for ComEd and returns the instance.
// It uses lflag to register command-line flags for configuration.
func configuredComEdHourly() *BaseComEdHourly {
	c := &BaseComEdHourly{
		client:           common.HTTPClient(time.Minute),
		historicalPrices: make(map[int64]types.Price),
	}
	apiURL := lflag.String("comed-api-url", "https://hourlypricing.comed.com/api", "URL for the ComEd Hourly Pricing API")
	pjmURL := lflag.String("pjm-api-url", "https://api.pjm.com/api/v1/da_hrl_lmps", "URL for the PJM API")
	pjmKey := lflag.String("pjm-api-key", "", "API Key for PJM Data Miner 2 (optional)")

	lflag.Do(func() {
		c.apiURL = *apiURL
		c.pjmAPIURL = *pjmURL
		c.pjmAPIKey = *pjmKey
	})

	return c
}

// Validate ensures the configuration is valid.
func (c *BaseComEdHourly) Validate() error {
	if c.apiURL == "" {
		return fmt.Errorf("comed-api-url is required")
	}
	if _, err := url.Parse(c.apiURL); err != nil {
		return fmt.Errorf("failed to parse comed url (%s): %w", c.apiURL, err)
	}
	if c.pjmAPIURL != "" {
		if _, err := url.Parse(c.pjmAPIURL); err != nil {
			return fmt.Errorf("failed to parse pjm url (%s): %w", c.pjmAPIURL, err)
		}
	}
	return nil
}

// apiResponse represents the structure of the JSON returned by ComEd.
type comedPriceEntry struct {
	MillisUTC string `json:"millisUTC"`
	Price     string `json:"price"`
}

// fetchPrices retrieves prices from the ComEd API with specific parameters.
// It caches the result for 5 minutes.
func (c *BaseComEdHourly) getCachedCurrentPrices(ctx context.Context) ([]types.Price, error) {
	now := time.Now().In(ctLocation)

	c.mu.Lock()
	// we only need to fetch if it's been a new 5 minute block
	if !c.lastFetchTime.IsZero() && !now.Truncate(5*time.Minute).After(c.lastFetchTime) {
		prices := c.cachedPrices
		c.mu.Unlock()
		return prices, nil
	}
	c.mu.Unlock()

	// Fetch enough history to get at least the last few hours complete.
	// 6 hours back should be plenty to get full hours even with delays.
	start := now.Add(-6 * time.Hour)
	prices, err := c.fetchPricesRange(ctx, start, now)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cachedPrices = prices
	c.lastFetchTime = now
	c.mu.Unlock()

	return prices, nil
}

// GetConfirmedPrices returns confirmed prices for a specific time range.
// This requests 5-minute feed data and averages it into hourly buckets.
func (c *BaseComEdHourly) GetConfirmedPrices(ctx context.Context, start, end time.Time) ([]types.Price, error) {
	ctx = log.With(ctx, log.Ctx(ctx).With(slog.Time("start", start), slog.Time("end", end)))

	// Check if all needed hours are in cache
	c.mu.Lock()
	var cached []types.Price

	// iterate hourly
	curr := start.Truncate(time.Hour)
	allCached := true
	for curr.Before(end) {
		// key is unixtimestamp of start of hour
		if p, ok := c.historicalPrices[curr.Unix()]; ok {
			cached = append(cached, p)
		} else {
			allCached = false
		}
		curr = curr.Add(time.Hour)
	}
	c.mu.Unlock()

	if allCached {
		log.Ctx(ctx).DebugContext(ctx, "confirmed prices found in cache")
		return cached, nil
	}

	log.Ctx(ctx).DebugContext(ctx, "fetching confirmed price history from api")
	prices, err := c.fetchPricesRange(ctx, start, end)
	if err != nil {
		return nil, err
	}

	now := time.Now().In(ctLocation)
	confirmedPrices := make([]types.Price, 0, len(prices))
	var earliest time.Time
	var latest time.Time

	// Iterate backwards to find the first complete hour.
	for i := len(prices) - 1; i >= 0; i-- {
		p := prices[i]
		// ignore any prices that are in the future
		if p.TSEnd.After(now) {
			continue
		}

		// less than 59 minutes means we don't have a full hour
		if p.TSEnd.Sub(p.TSStart) <= 59*time.Minute {
			continue
		}

		// require full 12 5-minute periods to ensure complete data but somehow we
		// don't have enough samples even though we have 59+minutes of data
		if p.SampleCount != 12 {
			log.Ctx(ctx).ErrorContext(
				ctx,
				"incomplete price data for hour",
				slog.Time("tsStart", p.TSStart),
				slog.Time("tsEnd", p.TSEnd),
				slog.Int("sampleCount", p.SampleCount))
			continue
		}

		confirmedPrices = append(confirmedPrices, p)

		if earliest.IsZero() || p.TSStart.Before(earliest) {
			earliest = p.TSStart
		}
		if p.TSEnd.After(latest) {
			latest = p.TSEnd
		}
	}

	log.Ctx(ctx).DebugContext(
		ctx,
		"got comed confirmed prices",
		slog.Time("earliest", earliest),
		slog.Time("latest", latest),
		slog.Int("count", len(confirmedPrices)),
	)

	// Update cache with confirmed prices
	c.mu.Lock()
	for _, p := range confirmedPrices {
		c.historicalPrices[p.TSStart.Unix()] = p
	}
	c.mu.Unlock()

	return confirmedPrices, nil
}

// fetchPricesRange retrieves prices from the ComEd API for a specific range.
func (c *BaseComEdHourly) fetchPricesRange(ctx context.Context, start, end time.Time) ([]types.Price, error) {
	start = start.In(ctLocation)
	end = end.In(ctLocation)

	u, err := url.Parse(c.apiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid api url: %w", err)
	}

	params := url.Values{}
	params.Set("type", "5minutefeed")
	params.Set("datestart", start.Format("200601021504"))
	params.Set("dateend", end.Format("200601021504"))
	params.Set("format", "json")
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	log.Ctx(ctx).DebugContext(ctx, "fetching prices from comed", "url", u.String())

	resp, err := c.client.Do(req)
	if err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to fetch prices", "error", err)
		return nil, fmt.Errorf("failed to fetch prices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("comed api returned status: %d", resp.StatusCode)
	}

	var data []comedPriceEntry
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		// Sometimes ComEd returns empty body or non-json on error or no data
		log.Ctx(ctx).ErrorContext(ctx, "failed to decode comed response", slog.Any("error", err))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	log.Ctx(ctx).DebugContext(
		ctx,
		"fetched prices",
		slog.Int("count", len(data)),
		slog.String("start", start.Format(time.RFC3339)),
		slog.String("end", end.Format(time.RFC3339)),
	)

	// Map to group prices by hour
	type hourlyData struct {
		start    time.Time
		sum      float64
		count    int
		lastTime time.Time
	}
	hours := make(map[int64]*hourlyData) // Key by unix hour to handle map keys

	for _, item := range data {
		ms, err := strconv.ParseInt(item.MillisUTC, 10, 64)
		if err != nil {
			log.Ctx(ctx).WarnContext(ctx, "failed to parse comed millisUTC", slog.String("value", item.MillisUTC), slog.Any("error", err))
			continue
		}
		centsPerKWH, err := strconv.ParseFloat(item.Price, 64)
		if err != nil {
			log.Ctx(ctx).WarnContext(ctx, "failed to parse comed price", slog.String("value", item.Price), slog.Any("error", err))
			continue
		}

		tsEnd := time.UnixMilli(ms).In(ctLocation)
		hourStart := tsEnd.Truncate(time.Hour)
		key := hourStart.Unix()

		if _, exists := hours[key]; !exists {
			hours[key] = &hourlyData{start: hourStart}
		}
		h := hours[key]
		h.sum += centsPerKWH
		h.count++
		if tsEnd.After(h.lastTime) {
			h.lastTime = tsEnd
		}
	}

	var prices []types.Price
	for _, h := range hours {
		avgCents := h.sum / float64(h.count)
		prices = append(prices, types.Price{
			Provider:      "comed_besh",
			TSStart:       h.start,
			TSEnd:         h.lastTime.Add(4*time.Minute + 59*time.Second),
			DollarsPerKWH: avgCents / 100, // Cents to Dollars
			SampleCount:   h.count,
		})
	}

	// Sort by TSStart
	sort.Slice(prices, func(i, j int) bool {
		return prices[i].TSStart.Before(prices[j].TSStart)
	})

	return prices, nil
}

// GetCurrentPrice returns the latest hourly-averaged price.
func (c *BaseComEdHourly) GetCurrentPrice(ctx context.Context) (types.Price, error) {
	log.Ctx(ctx).DebugContext(ctx, "getting current price")

	prices, err := c.getCachedCurrentPrices(ctx)
	if err != nil {
		return types.Price{}, err
	}

	if len(prices) == 0 {
		return types.Price{}, fmt.Errorf("no prices returned for current window")
	}

	// Return the latest available price (even if incomplete)
	latest := prices[len(prices)-1]
	log.Ctx(ctx).DebugContext(
		ctx,
		"got current price",
		slog.Float64("price", latest.DollarsPerKWH),
		slog.Time("ts", latest.TSStart),
	)
	return latest, nil
}

// GetFuturePrices returns predicted or day-ahead prices.
// Prefers PJM API if configured, otherwise returns nothing
func (c *BaseComEdHourly) GetFuturePrices(ctx context.Context) ([]types.Price, error) {
	if c.pjmAPIKey == "" {
		return nil, nil
	}

	c.mu.Lock()
	// TODO: instead we should only update if we're running out of future prices
	// but what if they change?
	if !c.lastFutureFetch.IsZero() && time.Since(c.lastFutureFetch) < 15*time.Minute {
		prices := c.cachedFuture
		c.mu.Unlock()
		return prices, nil
	}
	c.mu.Unlock()

	log.Ctx(ctx).DebugContext(ctx, "fetching pjm day ahead prices for comed")
	prices, err := c.fetchPJMDayAhead(ctx, pjmComedPNodeID)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cachedFuture = prices
	c.lastFutureFetch = time.Now()
	c.mu.Unlock()

	return prices, nil
}

// PJM API Support

type pjmItem struct {
	DatetimeBeginningEPT string  `json:"datetime_beginning_ept"`
	TotalLMPDA           float64 `json:"total_lmp_da"`
}

func (c *BaseComEdHourly) fetchPJMDayAhead(ctx context.Context, pnodeID string) ([]types.Price, error) {
	now := time.Now().In(etLocation)
	today := now.Format("2006-01-02")
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
	dateRange := fmt.Sprintf("%s 00:00 to %s 23:59", today, tomorrow)

	u, err := url.Parse(c.pjmAPIURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pjm url (%s): %w", c.pjmAPIURL, err)
	}
	q := u.Query()
	q.Set("pnode_id", pnodeID)
	q.Set("datetime_beginning_ept", dateRange)
	q.Set("format", "json")
	q.Set("fields", "datetime_beginning_ept,total_lmp_da")
	// download true removes the metadata and returns only the data
	q.Set("download", "true")
	q.Set("startRow", "1")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", c.pjmAPIKey)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "application/json")

	log.Ctx(ctx).DebugContext(
		ctx,
		"fetching pjm prices",
		slog.String("url", u.String()),
		slog.String("pnodeID", pnodeID),
	)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pjm api status: %d", resp.StatusCode)
	}

	var res []pjmItem
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	var prices []types.Price
	var earliest time.Time
	var latest time.Time
	for _, item := range res {
		// Parse EPT time
		t, err := time.ParseInLocation("2006-01-02T15:04:05", item.DatetimeBeginningEPT, etLocation)
		if err != nil {
			log.Ctx(ctx).WarnContext(ctx, "failed to parse pjm time", slog.String("time", item.DatetimeBeginningEPT), slog.Any("error", err))
			continue
		}
		// make sure it's truncated to the hour
		t = t.Truncate(time.Hour)

		// Convert $/MWh to $/kWh

		// HEC = LMP x (1 MWh/ 1000 kWh) x BUF x ISUF x (1 + DLF)
		// Residential Single Family Without Electric Space Heat 0.0517 0.0459
		// Residential Multi Family Without Electric Space Heat 0.0532 0.0468
		// Residential Single Family With Electric Space Heat 0.0554 0.0473
		// Residential Multi Family With Electric Space Heat 0.0567 0.0497
		// TODO: how to apply PJM service charge
		hec := (item.TotalLMPDA / 1000) * 1.0124 * 1.0002 * (1.0 + .047)

		prices = append(prices, types.Price{
			Provider:      "comed_besh",
			TSStart:       t,
			TSEnd:         t.Add(time.Hour),
			DollarsPerKWH: hec,
		})
		if earliest.IsZero() || t.Before(earliest) {
			earliest = t
		}
		if latest.IsZero() || t.After(latest) {
			latest = t
		}
	}

	log.Ctx(ctx).DebugContext(
		ctx,
		"fetched pjm prices",
		slog.Int("count", len(prices)),
		slog.String("pnodeID", pnodeID),
		slog.Time("earliest", earliest),
		slog.Time("latest", latest),
	)
	return prices, nil
}

// SiteComEd wraps BaseComEd to apply site-specific settings and fees.
type SiteComEd struct {
	base     UtilityPrices
	mu       sync.Mutex
	siteID   string
	settings types.Settings
}

// ApplySettings implements the Utility interface
func (s *SiteComEd) ApplySettings(ctx context.Context, settings types.Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if settings.UtilityProvider != "comed" {
		return fmt.Errorf("invalid utility provider for ComEd: %s", settings.UtilityProvider)
	}
	if settings.UtilityRate != "comed_besh" {
		return fmt.Errorf("invalid utility rate for ComEd: %s", settings.UtilityRate)
	}

	switch settings.UtilityRateOptions.RateClass {
	case ComEdRateClassSingleFamilyResidenceWithoutElectricSpaceHeat,
		ComEdRateClassMultiFamilyResidenceWithoutElectricSpaceHeat,
		ComEdRateClassSingleFamilyResidenceWithElectricSpaceHeat,
		ComEdRateClassMultiFamilyResidenceWithElectricSpaceHeat:
		break
	default:
		return fmt.Errorf("invalid rate class for ComEd: %s", settings.UtilityRateOptions.RateClass)
	}

	s.settings = settings
	return nil
}

func (s *SiteComEd) getDefaultAdditionalFees(ts time.Time, ro types.UtilityRateOptions) ([]types.UtilityAdditionalFeesPeriod, error) {
	// TODO: use the ts to decide to use 2026 or 2027 prices

	// The incremental distribution uncollectible cost factor applicable for residential
	// retail customers (IDUFR) equals the applicable IDUFR listed in Informational
	// Sheet No. 20.
	iduf := 1.0090 // for 2026

	// The applicable Delivery Reconciliation Adjustment Factor listed in Informational
	// Sheet No. 9.
	draf := 0.0 // for 2026

	// The applicable Excess Deferred Income Tax Factor listed in Informational Sheet No. 62.
	edaf := 0.0 // for 2026

	// The applicable Total Plan Adjustment Factor listed in Informational Sheet No. 65.
	tpaf := 0.06551 // for 2026

	// The applicable Revenue Balancing Adjustment Factor for a Delivery Class D listed in
	// Informational Sheet No. 18.
	var rbafd float64
	switch ro.RateClass {
	case ComEdRateClassSingleFamilyResidenceWithoutElectricSpaceHeat, "":
		rbafd = 0.007668
	case ComEdRateClassMultiFamilyResidenceWithoutElectricSpaceHeat:
		rbafd = 0.011682
	case ComEdRateClassSingleFamilyResidenceWithElectricSpaceHeat:
		rbafd = 0.069810
	case ComEdRateClassMultiFamilyResidenceWithElectricSpaceHeat:
		rbafd = 0.064486
	default:
		return nil, fmt.Errorf("unknown rate class: %s", ro.RateClass)
	}

	// DGRAD = The applicable Distributed Generation (DG) Rebate Adjustment listed in Informational
	// Sheet No. 56.
	// provided in cents per kWh
	dgrad := 0.062 / 100 // for 2026

	var iedt float64
	switch ro.RateClass {
	case ComEdRateClassSingleFamilyResidenceWithoutElectricSpaceHeat, "":
		iedt = 0.00124
	case ComEdRateClassMultiFamilyResidenceWithoutElectricSpaceHeat:
		iedt = 0.00124
	case ComEdRateClassSingleFamilyResidenceWithElectricSpaceHeat:
		iedt = 0.00124
	case ComEdRateClassMultiFamilyResidenceWithElectricSpaceHeat:
		iedt = 0.00124
	default:
		return nil, fmt.Errorf("unknown rate class: %s", ro.RateClass)
	}
	// IEDT & ADJ = IEDT x (IDUF + RBAFD)
	//1.082178
	iedtAdj := iedt * (iduf + rbafd)

	fees := []types.UtilityAdditionalFeesPeriod{
		{
			UtilityPeriod: types.UtilityPeriod{
				Start:       time.Date(2026, 1, 1, 0, 0, 0, 0, ctLocation),
				End:         time.Date(2027, 1, 1, 0, 0, 0, 0, ctLocation),
				HourStart:   0,
				HourEnd:     24,
				LocationPtr: ctLocation,
			},
			GridAdditional: true,
			DollarsPerKWH:  iedtAdj,
			Description:    "IL Electricity Distribution Charge - IEDT & ADJ",
		},
	}

	if !ro.VariableDeliveryRate {
		var dfc float64
		switch ro.RateClass {
		case ComEdRateClassSingleFamilyResidenceWithoutElectricSpaceHeat, "":
			dfc = 0.05698
		case ComEdRateClassMultiFamilyResidenceWithoutElectricSpaceHeat:
			dfc = 0.04354
		case ComEdRateClassSingleFamilyResidenceWithElectricSpaceHeat:
			dfc = 0.02712
		case ComEdRateClassMultiFamilyResidenceWithElectricSpaceHeat:
			dfc = 0.02576
		default:
			return nil, fmt.Errorf("unknown rate class: %s", ro.RateClass)
		}
		// DFC & ADJ = DFC x (IDUF + DRAF + EDAF + TPAF + RBAFD) + DGRAD
		dfcAdj := dfc*(iduf+draf+edaf+tpaf+rbafd) + dgrad

		return append(fees, types.UtilityAdditionalFeesPeriod{
			UtilityPeriod: types.UtilityPeriod{
				Start:       time.Date(2026, 1, 1, 0, 0, 0, 0, ctLocation),
				End:         time.Date(2027, 1, 1, 0, 0, 0, 0, ctLocation),
				HourStart:   0,
				HourEnd:     24,
				LocationPtr: ctLocation,
			},
			GridAdditional: true,
			DollarsPerKWH:  dfcAdj,
			Description:    "Distribution Facilities Charge - DFC & ADJ",
		}), nil
	} else {
		// time of use distribution facilities charges
		var morningDFC float64
		var midDayDFC float64
		var eveningDFC float64
		var nightDFC float64
		switch ro.RateClass {
		case ComEdRateClassSingleFamilyResidenceWithoutElectricSpaceHeat, "":
			// we default to single family non-electric heating
			morningDFC = 0.04009
			midDayDFC = 0.10712
			eveningDFC = 0.03747
			nightDFC = 0.02984
		case ComEdRateClassMultiFamilyResidenceWithoutElectricSpaceHeat:
			morningDFC = 0.03073
			midDayDFC = 0.08689
			eveningDFC = 0.02856
			nightDFC = 0.02251
		case ComEdRateClassSingleFamilyResidenceWithElectricSpaceHeat:
			morningDFC = 0.01999
			midDayDFC = 0.05329
			eveningDFC = 0.01890
			nightDFC = 0.01550
		case ComEdRateClassMultiFamilyResidenceWithElectricSpaceHeat:
			morningDFC = 0.01925
			midDayDFC = 0.04975
			eveningDFC = 0.01823
			nightDFC = 0.01512
		default:
			return nil, fmt.Errorf("unknown rate class: %s", ro.RateClass)
		}
		// DFC & ADJ = DFC x (IDUF + DRAF + EDAF + TPAF + RBAFD) + DGRAD
		morningDFCAdj := morningDFC*(iduf+draf+edaf+tpaf+rbafd) + dgrad
		midDayDFCAdj := midDayDFC*(iduf+draf+edaf+tpaf+rbafd) + dgrad
		eveningDFCAdj := eveningDFC*(iduf+draf+edaf+tpaf+rbafd) + dgrad
		nightDFCAdj := nightDFC*(iduf+draf+edaf+tpaf+rbafd) + dgrad
		return append(fees, []types.UtilityAdditionalFeesPeriod{
			// night (midnight - 6am)
			{
				UtilityPeriod: types.UtilityPeriod{
					Start:       time.Date(2026, 1, 1, 0, 0, 0, 0, ctLocation),
					End:         time.Date(2027, 1, 1, 0, 0, 0, 0, ctLocation),
					HourStart:   0,
					HourEnd:     6,
					LocationPtr: ctLocation,
				},
				GridAdditional: true,
				DollarsPerKWH:  nightDFCAdj,
				Description:    "TOU Distribution Facilities Charge (Night) - DFC & ADJ",
			},
			// morning (6am - 1pm)
			{
				UtilityPeriod: types.UtilityPeriod{
					Start:       time.Date(2026, 1, 1, 0, 0, 0, 0, ctLocation),
					End:         time.Date(2027, 1, 1, 0, 0, 0, 0, ctLocation),
					HourStart:   6,
					HourEnd:     13,
					LocationPtr: ctLocation,
				},
				GridAdditional: true,
				DollarsPerKWH:  morningDFCAdj,
				Description:    "TOU Distribution Facilities Charge (Morning) - DFC & ADJ",
			},
			// mid day (1pm - 7pm)
			{
				UtilityPeriod: types.UtilityPeriod{
					Start:       time.Date(2026, 1, 1, 0, 0, 0, 0, ctLocation),
					End:         time.Date(2027, 1, 1, 0, 0, 0, 0, ctLocation),
					HourStart:   13,
					HourEnd:     19,
					LocationPtr: ctLocation,
				},
				GridAdditional: true,
				DollarsPerKWH:  midDayDFCAdj,
				Description:    "TOU Distribution Facilities Charge (Mid Day) - DFC & ADJ",
			},
			// evening (7pm - 9pm)
			{
				UtilityPeriod: types.UtilityPeriod{
					Start:       time.Date(2026, 1, 1, 0, 0, 0, 0, ctLocation),
					End:         time.Date(2027, 1, 1, 0, 0, 0, 0, ctLocation),
					HourStart:   19,
					HourEnd:     21,
					LocationPtr: ctLocation,
				},
				GridAdditional: true,
				DollarsPerKWH:  eveningDFCAdj,
				Description:    "TOU Distribution Facilities Charge (Evening) - DFC & ADJ",
			},
			// night (9pm - midnight)
			{
				UtilityPeriod: types.UtilityPeriod{
					Start:       time.Date(2026, 1, 1, 0, 0, 0, 0, ctLocation),
					End:         time.Date(2027, 1, 1, 0, 0, 0, 0, ctLocation),
					HourStart:   21,
					HourEnd:     24,
					LocationPtr: ctLocation,
				},
				GridAdditional: true,
				DollarsPerKWH:  nightDFCAdj,
				Description:    "TOU Distribution Facilities Charge (Night) - DFC & ADJ",
			},
		}...), nil
	}
}

func (s *SiteComEd) getPeriods(ts time.Time) ([]types.UtilityAdditionalFeesPeriod, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.settings.AdditionalFeesPeriods) == 0 {
		return s.getDefaultAdditionalFees(ts, s.settings.UtilityRateOptions)
	}
	return s.settings.AdditionalFeesPeriods, nil
}

func (s *SiteComEd) applyFees(p types.Price) (types.Price, error) {
	periods, err := s.getPeriods(p.TSStart)
	if err != nil {
		return p, err
	}
	for _, period := range periods {
		// Calculate time-of-day in minutes for easier comparison if needed, or just use hour
		// Check date range
		if !period.Start.IsZero() && p.TSStart.Before(period.Start) {
			continue
		}
		if !period.End.IsZero() && p.TSStart.After(period.End) {
			continue
		}

		// Check hour range (inclusive start, exclusive end)
		// Assuming p.TSStart is the start of the hour
		h := p.TSStart.In(ctLocation).Hour()
		if h < period.HourStart || h >= period.HourEnd {
			continue
		}

		// Apply fee
		if period.GridAdditional {
			p.GridUseDollarsPerKWH += period.DollarsPerKWH
		} else {
			p.DollarsPerKWH += period.DollarsPerKWH
		}
	}
	return p, nil
}

func (s *SiteComEd) GetConfirmedPrices(ctx context.Context, start, end time.Time) ([]types.Price, error) {
	prices, err := s.base.GetConfirmedPrices(ctx, start, end)
	if err != nil {
		return nil, err
	}
	for i := range prices {
		prices[i], err = s.applyFees(prices[i])
		if err != nil {
			return nil, err
		}
	}
	return prices, nil
}

func (s *SiteComEd) GetCurrentPrice(ctx context.Context) (types.Price, error) {
	p, err := s.base.GetCurrentPrice(ctx)
	if err != nil {
		return types.Price{}, err
	}
	return s.applyFees(p)
}

func (s *SiteComEd) GetFuturePrices(ctx context.Context) ([]types.Price, error) {
	prices, err := s.base.GetFuturePrices(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]types.Price, len(prices))
	for i, p := range prices {
		out[i], err = s.applyFees(p)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
