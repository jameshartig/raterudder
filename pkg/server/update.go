package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jameshartig/raterudder/pkg/log"
	"github.com/jameshartig/raterudder/pkg/types"
)

// priceCache is a simple cache for utility prices during a bulk update.
type priceCache struct {
	CurrentPrices      map[string]types.Price
	FuturePrices       map[string][]types.Price
	SyncedPriceHistory map[string]bool
}

func newPriceCache() *priceCache {
	return &priceCache{
		CurrentPrices:      make(map[string]types.Price),
		FuturePrices:       make(map[string][]types.Price),
		SyncedPriceHistory: make(map[string]bool),
	}
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	siteID := s.getSiteID(r)

	action, status, err := s.performSiteUpdate(ctx, siteID, newPriceCache())
	if err != nil {
		// Log the error, but check if we returned an error that should be returned to the client
		log.Ctx(ctx).ErrorContext(ctx, "update failed", slog.Any("error", err))
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if action != nil {
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"action": action,
			"price":  action.CurrentPrice,
		}); err != nil {
			panic(http.ErrAbortHandler)
		}
	} else {
		// No action taken (e.g. paused or emergency mode)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status": status,
		}); err != nil {
			panic(http.ErrAbortHandler)
		}
	}
}

func (s *Server) handleUpdateSites(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	sites, err := s.storage.ListSites(ctx)
	if err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to list sites", slog.Any("error", err))
		http.Error(w, "failed to list sites", http.StatusInternalServerError)
		return
	}

	cache := newPriceCache()
	results := make(map[string]string)

	for _, site := range sites {
		log.Ctx(ctx).InfoContext(ctx, "processing site update", slog.String("siteID", site.ID))
		_, status, err := s.performSiteUpdate(ctx, site.ID, cache)
		if err != nil {
			log.Ctx(ctx).ErrorContext(ctx, "site update failed", slog.String("siteID", site.ID), slog.Any("error", err))
			results[site.ID] = fmt.Sprintf("failed: %v", err)
		} else {
			if status == "" {
				status = "success"
			}
			results[site.ID] = status
		}
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(results); err != nil {
		panic(http.ErrAbortHandler)
	}
}

func (s *Server) performSiteUpdate(ctx context.Context, siteID string, cache *priceCache) (*types.Action, string, error) {
	// 1. Get Settings and Credentials
	settings, creds, err := s.getSettingsWithMigration(ctx, siteID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get settings: %w", err)
	}

	// Get ESS System
	essSystem := s.ess.Site(siteID)

	// and apply those settings to the ESS
	err = essSystem.ApplySettings(ctx, settings, creds)
	if err != nil {
		return nil, "", fmt.Errorf("failed to apply settings: %w", err)
	}

	log.Ctx(ctx).DebugContext(ctx, "update: settings applied")

	// get utility
	utility, err := s.utilities.Provider(settings.UtilityProvider)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get utility system (%s): %w", settings.UtilityProvider, err)
	}

	// 2. Sync energy history
	{
		// First, find out the last time we have history for
		lastHistoryTime, lastVersion, err := s.storage.GetLatestEnergyHistoryTime(ctx, siteID)
		if err != nil {
			log.Ctx(ctx).WarnContext(ctx, "failed to get latest energy history time", slog.Any("error", err))
		}

		// Determine start time for fetching new data
		// We want at most last 5 days, but starting from the last record
		// truncated to the hour in case we previously stored an incomplete hour.
		now := time.Now()
		fiveDaysAgo := now.Add(-5 * 24 * time.Hour)
		syncStart := time.Date(fiveDaysAgo.Year(), fiveDaysAgo.Month(), fiveDaysAgo.Day(), 0, 0, 0, 0, fiveDaysAgo.Location())

		// Only use lastHistoryTime if version matches
		if !lastHistoryTime.IsZero() && lastVersion >= types.CurrentEnergyStatsVersion && lastHistoryTime.After(syncStart) {
			syncStart = lastHistoryTime.Truncate(time.Hour)
		} else if !lastHistoryTime.IsZero() && lastVersion < types.CurrentEnergyStatsVersion {
			log.Ctx(ctx).InfoContext(
				ctx,
				"backfilling energy history due to version mismatch",
				slog.Int("lastVersion", lastVersion),
				slog.Int("currentVersion", types.CurrentEnergyStatsVersion),
			)
		}

		log.Ctx(ctx).DebugContext(ctx, "syncing energy history", slog.Any("since", syncStart))

		// Loop day by day
		for t := syncStart; t.Before(now); t = t.Add(24 * time.Hour) {
			end := t.Add(24 * time.Hour)
			if end.After(now) {
				end = now
			}

			log.Ctx(ctx).DebugContext(ctx, "syncing energy history batch", slog.Time("start", t), slog.Time("end", end))
			newHistory, err := essSystem.GetEnergyHistory(ctx, t, end)
			if err != nil {
				log.Ctx(ctx).ErrorContext(ctx, "failed to get energy history from ess", slog.Any("error", err), slog.Time("start", t), slog.Time("end", end))
				// continue to next day even if this one failed
			} else {
				for _, h := range newHistory {
					if err := s.storage.UpsertEnergyHistory(ctx, siteID, h, types.CurrentEnergyStatsVersion); err != nil {
						log.Ctx(ctx).ErrorContext(ctx, "failed to upsert energy history", slog.Any("error", err))
					}
				}
			}
		}
	}

	log.Ctx(ctx).DebugContext(ctx, "update: energy history synced")

	// 2b. Sync price history
	if !cache.SyncedPriceHistory[settings.UtilityProvider] {
		lastPriceTime, lastVersion, err := s.storage.GetLatestPriceHistoryTime(ctx, settings.UtilityProvider)
		if err != nil {
			log.Ctx(ctx).WarnContext(ctx, "failed to get latest price history time", slog.Any("error", err))
		}

		now := time.Now()
		fiveDaysAgo := now.Add(-5 * 24 * time.Hour)
		syncStart := time.Date(fiveDaysAgo.Year(), fiveDaysAgo.Month(), fiveDaysAgo.Day(), 0, 0, 0, 0, fiveDaysAgo.Location())

		if !lastPriceTime.IsZero() && lastVersion >= types.CurrentPriceHistoryVersion && lastPriceTime.After(syncStart) {
			syncStart = lastPriceTime.Truncate(time.Hour)
		} else if !lastPriceTime.IsZero() && lastVersion < types.CurrentPriceHistoryVersion {
			log.Ctx(ctx).InfoContext(
				ctx,
				"backfilling price history due to version mismatch",
				slog.Int("lastVersion", lastVersion),
				slog.Int("currentVersion", types.CurrentPriceHistoryVersion),
			)
		}

		log.Ctx(ctx).DebugContext(ctx, "syncing price history", slog.Any("since", syncStart))

		// Loop day by day
		for t := syncStart; t.Before(now); t = t.Add(24 * time.Hour) {
			end := t.Add(24 * time.Hour)
			if end.After(now) {
				end = now
			}

			// Optimisation: if we're doing a bulk update, we might have already synced this provider recently.
			// But since we're doing it site by site, logic is same.
			log.Ctx(ctx).DebugContext(ctx, "syncing price history batch", "start", t, "end", end)
			newPrices, err := utility.GetConfirmedPrices(ctx, t, end)
			if err != nil {
				log.Ctx(ctx).ErrorContext(ctx, "failed to get confirmed prices", slog.Any("error", err), slog.Time("start", t), slog.Time("end", end))
			} else {
				for _, p := range newPrices {
					if err := s.storage.UpsertPrice(ctx, p, types.CurrentPriceHistoryVersion); err != nil {
						log.Ctx(ctx).ErrorContext(ctx, "failed to upsert price", slog.Any("error", err))
					}
				}
			}
		}
		cache.SyncedPriceHistory[settings.UtilityProvider] = true
	} else {
		log.Ctx(ctx).DebugContext(ctx, "price history already synced", "provider", settings.UtilityProvider)
	}

	log.Ctx(ctx).DebugContext(ctx, "update: price history synced")

	if settings.Pause {
		log.Ctx(ctx).InfoContext(ctx, "update: paused")
		return nil, "paused", nil
	}

	// 3. Fetch current ESS status
	status, err := essSystem.GetStatus(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get ess status: %w", err)
	}

	log.Ctx(ctx).DebugContext(ctx, "update: ess status fetched")

	// don't update if we're in emergency mode
	if status.EmergencyMode {
		log.Ctx(ctx).InfoContext(ctx, "update: emergency mode")
		action := types.Action{
			Timestamp:    time.Now(),
			BatteryMode:  types.BatteryModeNoChange,
			SolarMode:    types.SolarModeNoChange,
			Description:  "In emergency mode",
			SystemStatus: status,
			Fault:        true,
		}
		if err := s.storage.InsertAction(ctx, siteID, action); err != nil {
			log.Ctx(ctx).ErrorContext(ctx, "failed to insert action", slog.Any("error", err))
		}
		return nil, "emergency mode", nil
	}

	if len(status.Alarms) > 0 {
		log.Ctx(ctx).InfoContext(ctx, "update: alarms present", slog.Any("alarms", status.Alarms))
		action := types.Action{
			Timestamp:    time.Now(),
			BatteryMode:  types.BatteryModeNoChange,
			SolarMode:    types.SolarModeNoChange,
			Description:  fmt.Sprintf("%d alarms present", len(status.Alarms)),
			SystemStatus: status,
			Fault:        true,
		}
		if err := s.storage.InsertAction(ctx, siteID, action); err != nil {
			log.Ctx(ctx).ErrorContext(ctx, "failed to insert action", slog.Any("error", err))
		}
		return nil, "alarms present", nil
	}

	// 4. Get Current Price for controller
	var currentPrice types.Price
	if p, ok := cache.CurrentPrices[settings.UtilityProvider]; ok {
		currentPrice = p
	} else {
		currentPrice, err = utility.GetCurrentPrice(ctx)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get price: %w", err)
		}
		cache.CurrentPrices[settings.UtilityProvider] = currentPrice
	}

	log.Ctx(ctx).DebugContext(ctx, "update: current price fetched")

	// 5. Get Future Prices for controller
	var futurePrices []types.Price
	if p, ok := cache.FuturePrices[settings.UtilityProvider]; ok {
		futurePrices = p
	} else {
		futurePrices, err = utility.GetFuturePrices(ctx)
		if err != nil {
			log.Ctx(ctx).WarnContext(ctx, "failed to get future prices", slog.Any("error", err))
			// Continue with empty future prices
		} else {
			cache.FuturePrices[settings.UtilityProvider] = futurePrices
		}
	}

	// 6. Get History for Controller (Last 72 hours from Storage)
	historyStart := time.Now().Add(-72 * time.Hour)
	historyEnd := time.Now()
	energyHistory, err := s.storage.GetEnergyHistory(ctx, siteID, historyStart, historyEnd)
	if err != nil {
		log.Ctx(ctx).WarnContext(ctx, "failed to get energy history from storage", slog.Any("error", err))
	}

	log.Ctx(ctx).DebugContext(ctx, "update: starting decision")

	// 7. Decide Action
	decision, err := s.controller.Decide(ctx, status, currentPrice, futurePrices, energyHistory, settings)
	if err != nil {
		return nil, "", fmt.Errorf("controller decision failed: %w", err)
	}

	action := decision.Action
	// Ensure timestamps match if not set
	if action.Timestamp.IsZero() {
		action.Timestamp = time.Now()
	}

	log.Ctx(ctx).InfoContext(
		ctx,
		"update: decision made",
		slog.Int("batteryMode", int(action.BatteryMode)),
		slog.Int("solarMode", int(action.SolarMode)),
		slog.String("explanation", decision.Explanation),
		slog.String("description", action.Description),
		slog.Float64("price", currentPrice.DollarsPerKWH),
		slog.Float64("batterySOC", status.BatterySOC),
	)

	// 8. Execute Action
	switch action.BatteryMode {
	case types.BatteryModeChargeAny:
		err = essSystem.SetModes(ctx, types.BatteryModeChargeAny, types.SolarModeAny) // Force charge
	case types.BatteryModeLoad:
		err = essSystem.SetModes(ctx, types.BatteryModeLoad, types.SolarModeAny) // Use battery
	case types.BatteryModeStandby:
		// "self_consumption" is usually safe for idle too (just don't force charge)
		err = essSystem.SetModes(ctx, types.BatteryModeStandby, types.SolarModeAny)
	}
	if err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to set mode", slog.Any("error", err))
		action.Description += fmt.Sprintf(" (FAILED: %v)", err)
	}
	if settings.DryRun {
		action.DryRun = true
	}

	// 9. Log Action
	if err := s.storage.InsertAction(ctx, siteID, action); err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to insert action", slog.Any("error", err))
	}

	return &action, "", nil
}
