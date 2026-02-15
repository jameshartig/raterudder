package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jameshartig/raterudder/pkg/log"
)

func (s *Server) handleHistoryPrices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	siteID := s.getSiteID(r)
	start, end, err := parseTimeRange(r)
	if err != nil {
		http.Error(w, "invalid time range: "+err.Error(), http.StatusBadRequest)
		return
	}

	settings, _, err := s.getSettingsWithMigration(ctx, siteID)
	if err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to get settings", slog.Any("error", err))
		http.Error(w, "failed to get settings", http.StatusInternalServerError)
		return
	}

	prices, err := s.storage.GetPriceHistory(ctx, settings.UtilityProvider, start, end)
	if err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to get prices", slog.String("utility", settings.UtilityProvider), slog.Any("error", err))
		http.Error(w, "failed to get prices", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Set Cache-Control headers
	// If the range ends before today (midnight today), cache for 24 hours.
	// Otherwise, cache for 1 minute.
	today := time.Now().Truncate(24 * time.Hour)
	if end.Before(today) {
		w.Header().Set("Cache-Control", "private, max-age=86400")
	} else {
		w.Header().Set("Cache-Control", "private, max-age=60")
	}

	if err := json.NewEncoder(w).Encode(prices); err != nil {
		panic(http.ErrAbortHandler)
	}
}

func (s *Server) handleHistoryActions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	siteID := s.getSiteID(r)
	start, end, err := parseTimeRange(r)
	if err != nil {
		http.Error(w, "invalid time range: "+err.Error(), http.StatusBadRequest)
		return
	}

	actions, err := s.storage.GetActionHistory(ctx, siteID, start, end)
	if err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to get actions", slog.String("siteID", siteID), slog.Any("error", err))
		http.Error(w, "failed to get actions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Set Cache-Control headers
	// If the range ends before today (midnight today), cache for 24 hours.
	// Otherwise, cache for 1 minute.
	today := time.Now().Truncate(24 * time.Hour)
	if end.Before(today) {
		w.Header().Set("Cache-Control", "private, max-age=86400")
	} else {
		w.Header().Set("Cache-Control", "private, max-age=60")
	}

	if err := json.NewEncoder(w).Encode(actions); err != nil {
		panic(http.ErrAbortHandler)
	}
}

func parseTimeRange(r *http.Request) (time.Time, time.Time, error) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	if startStr == "" || endStr == "" {
		// Default to last 24 hours if not specified
		end := time.Now()
		start := end.Add(-24 * time.Hour)
		return start, end, nil
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start time: %w", err)
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end time: %w", err)
	}

	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("start time must be before end time")
	}

	if end.Sub(start) > 24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("time range cannot exceed 24 hours")
	}

	return start, end, nil
}
