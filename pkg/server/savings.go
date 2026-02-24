package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/raterudder/raterudder/pkg/log"
	"github.com/raterudder/raterudder/pkg/types"
)

func (s *Server) handleHistorySavings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	siteID := s.getSiteID(r)
	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSONError(w, fmt.Sprintf("invalid time range: %v", err), http.StatusBadRequest)
		return
	}

	var siteIDs []string
	if siteID == SiteIDAll {
		siteIDs = s.getAllUserSiteIDs(r)
	} else {
		siteIDs = []string{siteID}
	}

	var totalSavings types.SavingsStats
	totalSavings.Timestamp = start

	for _, id := range siteIDs {
		stats, err := s.getSiteSavings(ctx, id, start, end)
		if err != nil {
			log.Ctx(ctx).ErrorContext(ctx, "failed to get savings for site", slog.String("siteID", id), slog.Any("error", err))
			// If one site fails, maybe continue or fail fast? Failing fast for now to be safe.
			writeJSONError(w, fmt.Sprintf("failed to get savings for site %s", id), http.StatusInternalServerError)
			return
		}

		totalSavings.HomeUsed += stats.HomeUsed
		totalSavings.SolarGenerated += stats.SolarGenerated
		totalSavings.GridImported += stats.GridImported
		totalSavings.GridExported += stats.GridExported
		totalSavings.BatteryUsed += stats.BatteryUsed
		totalSavings.Cost += stats.Cost
		totalSavings.Credit += stats.Credit
		totalSavings.AvoidedCost += stats.AvoidedCost
		totalSavings.ChargingCost += stats.ChargingCost
		totalSavings.SolarSavings += stats.SolarSavings

		// Only include hourly debugging if it's a single site request
		if siteID != SiteIDAll {
			totalSavings.HourlyDebugging = stats.HourlyDebugging
		}
	}

	totalSavings.BatterySavings = totalSavings.AvoidedCost - totalSavings.ChargingCost

	w.Header().Set("Content-Type", "application/json")

	// Set Cache-Control (copying pattern from history.go)
	today := time.Now().Truncate(24 * time.Hour)
	if end.Before(today) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=60")
	}

	if err := json.NewEncoder(w).Encode(totalSavings); err != nil {
		panic(http.ErrAbortHandler)
	}
}

func (s *Server) getSiteSavings(ctx context.Context, siteID string, start, end time.Time) (types.SavingsStats, error) {
	// Fetch prices (these are hourly)
	prices, err := s.storage.GetPriceHistory(ctx, siteID, start, end)
	if err != nil {
		return types.SavingsStats{}, err
	}

	// Fetch energy stats (these are hourly)
	energyStats, err := s.storage.GetEnergyHistory(ctx, siteID, start, end)
	if err != nil {
		return types.SavingsStats{}, err
	}

	var stats types.SavingsStats
	stats.Timestamp = start
	hourlyExportPrices := make(map[time.Time]float64)
	hourlyImportPrices := make(map[time.Time]float64)

	for _, p := range prices {
		tsHour := p.TSStart.Truncate(time.Hour)
		hourlyExportPrices[tsHour] = p.DollarsPerKWH
		hourlyImportPrices[tsHour] = p.DollarsPerKWH + p.GridAddlDollarsPerKWH
	}

	for _, stat := range energyStats {
		ts := stat.TSHourStart.Truncate(time.Hour)

		// this will be 0 if we don't have price data for this hour
		gridImportPrice := hourlyImportPrices[ts]
		gridExportPrice := hourlyExportPrices[ts]

		// Accumulate Energy Amounts even if price is missing
		stats.HomeUsed += stat.HomeKWH
		stats.SolarGenerated += stat.SolarKWH
		stats.GridImported += stat.GridImportKWH
		stats.GridExported += stat.GridExportKWH
		stats.BatteryUsed += stat.BatteryUsedKWH

		// Cost and Credit
		cost := stat.GridImportKWH * gridImportPrice
		credit := stat.GridExportKWH * gridExportPrice
		stats.Cost += cost
		stats.Credit += credit

		// Determine how much battery was used to power the home and what cost we
		// avoided by using the battery instead of the grid.
		batteryToHome := stat.BatteryToHomeKWH
		avoided := batteryToHome * gridImportPrice
		stats.AvoidedCost += avoided

		// Determine how much battery was charged from the grid and what cost we
		// paid to charge the battery.
		gridToBattery := math.Max(0, stat.BatteryChargedKWH-stat.SolarToBatteryKWH)
		chargingCost := gridToBattery * gridImportPrice
		stats.ChargingCost += chargingCost

		// Solar Savings: Solar powering the home.
		// you might think to include solar to battery as solar savings but it gets
		// counted as battery savings later when the battery is discharged.
		solarToHome := stat.SolarToHomeKWH
		solarSavings := solarToHome * gridImportPrice
		stats.SolarSavings += solarSavings

		stats.HourlyDebugging = append(stats.HourlyDebugging, types.HourlySavingsStatsDebugging{
			ExportPrice:   gridExportPrice,
			ImportPrice:   gridImportPrice,
			BatteryToHome: batteryToHome,
			Avoided:       avoided,
			GridToBattery: gridToBattery,
			ChargingCost:  chargingCost,
			SolarToHome:   solarToHome,
			SolarSavings:  solarSavings,
		})
	}

	stats.BatterySavings = stats.AvoidedCost - stats.ChargingCost
	return stats, nil
}
