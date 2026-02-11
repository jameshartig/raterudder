package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jameshartig/autoenergy/pkg/types"
)

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if we need to enforce authentication
	email, ok := ctx.Value(emailContextKey).(string)
	if ok && email != "" {
		// User is authenticated via Cookie (OIDC)
		var allowed bool
		if s.updateSpecificEmail != "" && email == s.updateSpecificEmail {
			allowed = true
		} else {
			for _, admin := range s.adminEmails {
				if email == admin {
					allowed = true
					break
				}
			}
		}
		if !allowed {
			slog.WarnContext(ctx, "unauthorized email for update", slog.String("email", email))
			http.Error(w, "unauthorized email", http.StatusForbidden)
			return
		}
		slog.DebugContext(ctx, "update: authorized", slog.String("email", email))
	} else if s.updateSpecificAudience != "" && (s.updateSpecificEmail != "" || len(s.adminEmails) > 0) {
		// Not authenticated via Cookie, check Authorization Header (e.g. Cloud Scheduler)
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "invalid authorization header", http.StatusUnauthorized)
			return
		}

		payload, err := s.tokenValidator(ctx, parts[1], s.updateSpecificAudience)
		if err != nil {
			slog.WarnContext(ctx, "failed to validate id token", slog.Any("error", err))
			http.Error(w, "invalid id token", http.StatusUnauthorized)
			return
		}

		email, ok := payload.Claims["email"].(string)
		if !ok {
			slog.WarnContext(ctx, "invalid email in id token")
			http.Error(w, "invalid token claims", http.StatusForbidden)
			return
		}

		// Check admin emails
		var allowed bool
		if s.updateSpecificEmail != "" && email == s.updateSpecificEmail {
			allowed = true
		} else {
			for _, admin := range s.adminEmails {
				if email == admin {
					allowed = true
					break
				}
			}
		}
		if !allowed {
			slog.WarnContext(ctx, "unauthorized email for update", slog.String("email", email))
			http.Error(w, "unauthorized email", http.StatusForbidden)
			return
		}
		slog.DebugContext(ctx, "update: authorized", slog.String("email", email))
	} else if !s.bypassAuth {
		slog.WarnContext(ctx, "missing authentication for update")
		http.Error(w, "missing authentication", http.StatusUnauthorized)
		return
	}

	// 1. Get Settings
	settings, err := s.getSettingsWithMigration(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get settings", slog.Any("error", err))
		http.Error(w, "failed to get settings", http.StatusInternalServerError)
		return
	}
	// and apply those settings to the ESS
	err = s.essSystem.ApplySettings(ctx, settings)
	if err != nil {
		slog.ErrorContext(ctx, "failed to apply settings", slog.Any("error", err))
		http.Error(w, "failed to apply settings", http.StatusInternalServerError)
		return
	}

	slog.DebugContext(ctx, "update: settings applied")

	// 2. Sync energy history
	{
		// First, find out the last time we have history for
		lastHistoryTime, lastVersion, err := s.storage.GetLatestEnergyHistoryTime(ctx)
		if err != nil {
			slog.WarnContext(ctx, "failed to get latest energy history time", slog.Any("error", err))
		}

		// Determine start time for fetching new data
		// We want at most last 5 days, but starting from the last record
		// truncated to the hour in case we previously stored an incomplete hour.
		// User requested to fetch from the beginning of the 5th previous day.
		now := time.Now()
		fiveDaysAgo := now.Add(-5 * 24 * time.Hour)
		syncStart := time.Date(fiveDaysAgo.Year(), fiveDaysAgo.Month(), fiveDaysAgo.Day(), 0, 0, 0, 0, fiveDaysAgo.Location())

		// Only use lastHistoryTime if version matches
		if !lastHistoryTime.IsZero() && lastVersion >= types.CurrentEnergyStatsVersion && lastHistoryTime.After(syncStart) {
			syncStart = lastHistoryTime.Truncate(time.Hour)
		} else if !lastHistoryTime.IsZero() && lastVersion < types.CurrentEnergyStatsVersion {
			slog.InfoContext(
				ctx,
				"backfilling energy history due to version mismatch",
				slog.Int("lastVersion", lastVersion),
				slog.Int("currentVersion", types.CurrentEnergyStatsVersion),
			)
		}

		slog.DebugContext(ctx, "syncing energy history", slog.Any("since", syncStart))

		// Loop day by day
		for t := syncStart; t.Before(now); t = t.Add(24 * time.Hour) {
			end := t.Add(24 * time.Hour)
			if end.After(now) {
				end = now
			}

			slog.DebugContext(ctx, "syncing energy history batch", slog.Time("start", t), slog.Time("end", end))
			newHistory, err := s.essSystem.GetEnergyHistory(ctx, t, end)
			if err != nil {
				slog.ErrorContext(ctx, "failed to get energy history from ess", slog.Any("error", err), slog.Time("start", t), slog.Time("end", end))
				// continue to next day even if this one failed
			} else {
				for _, h := range newHistory {
					if err := s.storage.UpsertEnergyHistory(ctx, h, types.CurrentEnergyStatsVersion); err != nil {
						slog.ErrorContext(ctx, "failed to upsert energy history", slog.Any("error", err))
					}
				}
			}
		}
	}

	slog.DebugContext(ctx, "update: energy history synced")

	// 2b. Sync price history
	{
		lastPriceTime, lastVersion, err := s.storage.GetLatestPriceHistoryTime(ctx)
		if err != nil {
			slog.WarnContext(ctx, "failed to get latest price history time", slog.Any("error", err))
		}

		now := time.Now()
		fiveDaysAgo := now.Add(-5 * 24 * time.Hour)
		syncStart := time.Date(fiveDaysAgo.Year(), fiveDaysAgo.Month(), fiveDaysAgo.Day(), 0, 0, 0, 0, fiveDaysAgo.Location())

		if !lastPriceTime.IsZero() && lastVersion >= types.CurrentPriceHistoryVersion && lastPriceTime.After(syncStart) {
			syncStart = lastPriceTime.Truncate(time.Hour)
		} else if !lastPriceTime.IsZero() && lastVersion < types.CurrentPriceHistoryVersion {
			slog.InfoContext(
				ctx,
				"backfilling price history due to version mismatch",
				slog.Int("lastVersion", lastVersion),
				slog.Int("currentVersion", types.CurrentPriceHistoryVersion),
			)
		}

		slog.DebugContext(ctx, "syncing price history", slog.Any("since", syncStart))

		// Loop day by day
		for t := syncStart; t.Before(now); t = t.Add(24 * time.Hour) {
			end := t.Add(24 * time.Hour)
			if end.After(now) {
				end = now
			}

			slog.DebugContext(ctx, "syncing price history batch", "start", t, "end", end)
			newPrices, err := s.utilityProvider.GetConfirmedPrices(ctx, t, end)
			if err != nil {
				slog.ErrorContext(ctx, "failed to get confirmed prices", slog.Any("error", err), slog.Time("start", t), slog.Time("end", end))
			} else {
				for _, p := range newPrices {
					if err := s.storage.UpsertPrice(ctx, p, types.CurrentPriceHistoryVersion); err != nil {
						slog.ErrorContext(ctx, "failed to upsert price", slog.Any("error", err))
					}
				}
			}
		}
	}

	slog.DebugContext(ctx, "update: price history synced")

	if settings.Pause {
		slog.InfoContext(ctx, "update: paused")
		w.WriteHeader(http.StatusOK)
		// We return 200 OK so the scheduler doesn't think it failed
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "paused",
		}); err != nil {
			panic(http.ErrAbortHandler)
		}
		return
	}

	// 3. Fetch current ESS status
	status, err := s.essSystem.GetStatus(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get ess status", slog.Any("error", err))
		http.Error(w, "failed to get ess status", http.StatusInternalServerError)
		return
	}

	slog.DebugContext(ctx, "update: ess status fetched")

	// don't update if we're in emergency mode
	if status.EmergencyMode {
		slog.InfoContext(ctx, "update: emergency mode")
		action := types.Action{
			Timestamp:    time.Now(),
			BatteryMode:  types.BatteryModeNoChange,
			SolarMode:    types.SolarModeNoChange,
			Description:  "In emergency mode",
			SystemStatus: status,
			Fault:        true,
		}
		if err := s.storage.InsertAction(ctx, action); err != nil {
			slog.ErrorContext(ctx, "failed to insert action", slog.Any("error", err))
		}
		w.WriteHeader(http.StatusOK)
		// We return 200 OK so the scheduler doesn't think it failed
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "emergency mode",
		}); err != nil {
			panic(http.ErrAbortHandler)
		}
		return
	}

	if len(status.Alarms) > 0 {
		slog.InfoContext(ctx, "update: alarms present", slog.Any("alarms", status.Alarms))
		action := types.Action{
			Timestamp:    time.Now(),
			BatteryMode:  types.BatteryModeNoChange,
			SolarMode:    types.SolarModeNoChange,
			Description:  fmt.Sprintf("%d alarms present", len(status.Alarms)),
			SystemStatus: status,
			Fault:        true,
		}
		if err := s.storage.InsertAction(ctx, action); err != nil {
			slog.ErrorContext(ctx, "failed to insert action", slog.Any("error", err))
		}
		w.WriteHeader(http.StatusOK)
		// We return 200 OK so the scheduler doesn't think it failed
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "alarms present",
		}); err != nil {
			panic(http.ErrAbortHandler)
		}
		return
	}

	// 4. Get Current Price for controller
	currentPrice, err := s.utilityProvider.GetCurrentPrice(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get price", slog.Any("error", err))
		http.Error(w, "failed to get price", http.StatusInternalServerError)
		return
	}

	slog.DebugContext(ctx, "update: current price fetched")

	// 5. Get Future Prices for controller
	futurePrices, err := s.utilityProvider.GetFuturePrices(ctx)
	if err != nil {
		slog.WarnContext(ctx, "failed to get future prices", slog.Any("error", err))
		// Continue with empty future prices
	}

	// 6. Get History for Controller (Last 72 hours from Storage)
	historyStart := time.Now().Add(-72 * time.Hour)
	historyEnd := time.Now()
	energyHistory, err := s.storage.GetEnergyHistory(ctx, historyStart, historyEnd)
	if err != nil {
		slog.WarnContext(ctx, "failed to get energy history from storage", slog.Any("error", err))
	}

	slog.DebugContext(ctx, "update: starting decision")

	// 7. Decide Action
	decision, err := s.controller.Decide(ctx, status, currentPrice, futurePrices, energyHistory, settings)
	if err != nil {
		slog.ErrorContext(ctx, "controller decision failed", slog.Any("error", err))
		http.Error(w, "controller error", http.StatusInternalServerError)
		return
	}

	action := decision.Action
	// Ensure timestamps match if not set
	if action.Timestamp.IsZero() {
		action.Timestamp = time.Now()
	}

	slog.InfoContext(
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
		err = s.essSystem.SetModes(ctx, types.BatteryModeChargeAny, types.SolarModeAny) // Force charge
	case types.BatteryModeLoad:
		err = s.essSystem.SetModes(ctx, types.BatteryModeLoad, types.SolarModeAny) // Use battery
	case types.BatteryModeStandby:
		// "self_consumption" is usually safe for idle too (just don't force charge)
		err = s.essSystem.SetModes(ctx, types.BatteryModeStandby, types.SolarModeAny)
	}
	if err != nil {
		slog.ErrorContext(ctx, "failed to set mode", slog.Any("error", err))
		action.Description += fmt.Sprintf(" (FAILED: %v)", err)
	}
	if settings.DryRun {
		action.DryRun = true
	}

	// 9. Log Action
	if err := s.storage.InsertAction(ctx, action); err != nil {
		slog.ErrorContext(ctx, "failed to insert action", slog.Any("error", err))
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"action": action,
		"price":  currentPrice,
	}); err != nil {
		panic(http.ErrAbortHandler)
	}
}
