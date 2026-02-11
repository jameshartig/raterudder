package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

func (s *Server) handleModeling(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Get Settings
	settings, err := s.getSettingsWithMigration(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get settings", slog.Any("error", err))
		http.Error(w, "failed to get settings", http.StatusInternalServerError)
		return
	}

	// 2. Fetch current ESS status
	status, err := s.essSystem.GetStatus(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get ess status", slog.Any("error", err))
		http.Error(w, "failed to get ess status", http.StatusInternalServerError)
		return
	}

	// 3. Get Current Price
	currentPrice, err := s.utilityProvider.GetCurrentPrice(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get price", slog.Any("error", err))
		http.Error(w, "failed to get price", http.StatusInternalServerError)
		return
	}

	// 4. Get Future Prices
	futurePrices, err := s.utilityProvider.GetFuturePrices(ctx)
	if err != nil {
		slog.WarnContext(ctx, "failed to get future prices", slog.Any("error", err))
		// Continue with empty future prices
	}

	// 5. Get History (Last 72 hours from Storage) - no backfill
	historyStart := time.Now().Add(-72 * time.Hour)
	historyEnd := time.Now()
	energyHistory, err := s.storage.GetEnergyHistory(ctx, historyStart, historyEnd)
	if err != nil {
		slog.WarnContext(ctx, "failed to get energy history from storage", slog.Any("error", err))
		http.Error(w, "failed to get energy history", http.StatusInternalServerError)
		return
	}

	// 6. Run Simulation
	now := time.Now().In(status.Timestamp.Location())
	simHours := s.controller.SimulateState(ctx, now, status, currentPrice, futurePrices, energyHistory, settings)

	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(simHours); err != nil {
		panic(http.ErrAbortHandler)
	}
}
