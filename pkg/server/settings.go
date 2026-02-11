package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jameshartig/autoenergy/pkg/types"
)

func (s *Server) getSettingsWithMigration(ctx context.Context) (types.Settings, error) {
	settings, version, err := s.storage.GetSettings(ctx)
	if err != nil {
		return types.Settings{}, err
	}

	// Check for migration
	if version < types.CurrentSettingsVersion {
		slog.InfoContext(ctx, "migrating settings", slog.Int("oldVersion", version), slog.Int("newVersion", types.CurrentSettingsVersion))
		newSettings, changed, err := types.MigrateSettings(settings, version)
		if err != nil {
			// Log error but return settings as is (best effort)
			slog.ErrorContext(ctx, "failed to migrate settings", slog.Int("currentVersion", version), slog.Any("error", err))
			return settings, nil
		} else if changed {
			if err := s.storage.SetSettings(ctx, newSettings, types.CurrentSettingsVersion); err != nil {
				slog.ErrorContext(ctx, "failed to save migrated settings", slog.Any("error", err))
				// Return migrated settings even if save failed, so current request works with new defaults
			} else {
				slog.InfoContext(ctx, "saved migrated settings", slog.Int("oldVersion", version), slog.Int("newVersion", types.CurrentSettingsVersion))
			}
			return newSettings, nil
		}
	}
	return settings, nil
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	settings, err := s.getSettingsWithMigration(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get settings", slog.Any("error", err))
		http.Error(w, "failed to get settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(settings); err != nil {
		slog.ErrorContext(ctx, "failed to encode settings", slog.Any("error", err))
	}
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !s.bypassAuth {
		// don't let a misconfiguration to allow updates
		if len(s.adminEmails) == 0 {
			http.Error(w, "settings updates are disabled", http.StatusForbidden)
			return
		}

		// Validate Authentication from Context (set by authMiddleware)
		email, ok := ctx.Value(emailContextKey).(string)
		if !ok || email == "" {
			http.Error(w, "missing authentication", http.StatusUnauthorized)
			return
		}

		var allowed bool
		for _, admin := range s.adminEmails {
			if email == admin {
				allowed = true
				break
			}
		}

		if !allowed {
			slog.WarnContext(ctx, "unauthorized email for settings update", slog.String("email", email))
			http.Error(w, "unauthorized email", http.StatusForbidden)
			return
		}
	}

	var newSettings types.Settings
	if err := json.NewDecoder(r.Body).Decode(&newSettings); err != nil {
		slog.WarnContext(ctx, "failed to decode settings", slog.Any("error", err))
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Basic validation
	if newSettings.AdditionalFeesDollarsPerKWH < 0 ||
		newSettings.MinArbitrageDifferenceDollarsPerKWH < 0 ||
		newSettings.MinBatterySOC < 0 || newSettings.MinBatterySOC > 100 ||
		newSettings.IgnoreHourUsageOverMultiple < 1 ||
		newSettings.SolarBellCurveMultiplier < 0 || newSettings.SolarTrendRatioMax < 1 {
		http.Error(w, "invalid settings values", http.StatusBadRequest)
		return
	}

	if err := s.storage.SetSettings(ctx, newSettings, types.CurrentSettingsVersion); err != nil {
		slog.ErrorContext(ctx, "failed to save settings", slog.Any("error", err))
		http.Error(w, "failed to save settings", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(ctx, "settings updated")

	w.WriteHeader(http.StatusOK)
}
