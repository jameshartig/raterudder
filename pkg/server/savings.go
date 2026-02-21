package server

import (
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

	// Fetch prices (these are hourly)
	prices, err := s.storage.GetPriceHistory(ctx, siteID, start, end)
	if err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to get prices", slog.Any("error", err))
		writeJSONError(w, "failed to get price history", http.StatusInternalServerError)
		return
	}

	// Fetch energy stats (these are hourly)
	energyStats, err := s.storage.GetEnergyHistory(ctx, siteID, start, end)
	if err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to get energy history", slog.Any("error", err))
		writeJSONError(w, "failed to get energy history", http.StatusInternalServerError)
		return
	}

	var totalSavings types.SavingsStats
	totalSavings.Timestamp = start
	hourlyExportPrices := make(map[time.Time]float64)
	hourlyImportPrices := make(map[time.Time]float64)

	// TODO: fix this so that we look at the actual time ranges
	var lastPrice time.Time
	for _, p := range prices {
		tsHour := p.TSStart.Truncate(time.Hour)
		hourlyExportPrices[tsHour] = p.DollarsPerKWH
		hourlyImportPrices[tsHour] = p.DollarsPerKWH + p.GridAddlDollarsPerKWH
		if p.TSStart.After(lastPrice) {
			totalSavings.LastCost = p.DollarsPerKWH + p.GridAddlDollarsPerKWH
			totalSavings.LastPrice = p.DollarsPerKWH
			lastPrice = p.TSStart
		}
	}

	for _, stat := range energyStats {
		ts := stat.TSHourStart.Truncate(time.Hour)

		// this will be 0 if we don't have price data for this hour
		gridImportPrice := hourlyImportPrices[ts]
		gridExportPrice := hourlyExportPrices[ts]

		// Accumulate Energy Amounts even if price is missing
		totalSavings.HomeUsed += stat.HomeKWH
		totalSavings.SolarGenerated += stat.SolarKWH
		totalSavings.GridImported += stat.GridImportKWH
		totalSavings.GridExported += stat.GridExportKWH
		totalSavings.BatteryUsed += stat.BatteryUsedKWH

		// Cost and Credit
		cost := stat.GridImportKWH * gridImportPrice
		credit := stat.GridExportKWH * gridExportPrice
		totalSavings.Cost += cost
		totalSavings.Credit += credit

		// Determine how much battery was used to power the home and what cost we
		// avoided by using the battery instead of the grid.
		batteryToHome := stat.BatteryToHomeKWH
		avoided := batteryToHome * gridImportPrice
		totalSavings.AvoidedCost += avoided

		// Determine how much battery was charged from the grid and what cost we
		// paid to charge the battery.
		gridToBattery := math.Max(0, stat.BatteryChargedKWH-stat.SolarToBatteryKWH)
		chargingCost := gridToBattery * gridImportPrice
		totalSavings.ChargingCost += chargingCost

		// Solar Savings: Solar powering the home.
		// you might think to include solar to battery as solar savings but it gets
		// counted as battery savings later when the battery is discharged.
		solarToHome := stat.SolarToHomeKWH
		solarSavings := solarToHome * gridImportPrice
		totalSavings.SolarSavings += solarSavings

		totalSavings.HourlyDebugging = append(totalSavings.HourlyDebugging, types.HourlySavingsStatsDebugging{
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
