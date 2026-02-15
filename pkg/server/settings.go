package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jameshartig/raterudder/pkg/log"
	"github.com/jameshartig/raterudder/pkg/types"
)

func (s *Server) getSettingsWithMigration(ctx context.Context, siteID string) (types.Settings, types.Credentials, error) {
	settings, version, err := s.storage.GetSettings(ctx, siteID)
	if err != nil {
		return types.Settings{}, types.Credentials{}, err
	}

	// Check for migration
	if version < types.CurrentSettingsVersion {
		log.Ctx(ctx).InfoContext(ctx, "migrating settings", slog.Int("oldVersion", version), slog.Int("newVersion", types.CurrentSettingsVersion))
		newSettings, changed, err := types.MigrateSettings(settings, version)
		if err != nil {
			// Log error but return settings as is (best effort)
			log.Ctx(ctx).ErrorContext(ctx, "failed to migrate settings", slog.Int("currentVersion", version), slog.Any("error", err))
		} else if changed {
			if err := s.storage.SetSettings(ctx, siteID, newSettings, types.CurrentSettingsVersion); err != nil {
				log.Ctx(ctx).ErrorContext(ctx, "failed to save migrated settings", slog.Any("error", err))
				// Return migrated settings even if save failed, so current request works with new defaults
			} else {
				log.Ctx(ctx).InfoContext(ctx, "saved migrated settings", slog.Int("oldVersion", version), slog.Int("newVersion", types.CurrentSettingsVersion))
			}
			settings = newSettings
		}
	}

	var creds types.Credentials
	if len(settings.EncryptedCredentials) > 0 {
		creds, err = s.decryptCredentials(ctx, settings.EncryptedCredentials)
		if err != nil {
			log.Ctx(ctx).ErrorContext(ctx, "failed to decrypt credentials", slog.Any("error", err))
			return types.Settings{}, types.Credentials{}, err
		}
	}

	return settings, creds, nil
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	siteID := s.getSiteID(r)
	settings, _, err := s.getSettingsWithMigration(ctx, siteID)
	if err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to get settings", slog.Any("error", err))
		http.Error(w, "failed to get settings", http.StatusInternalServerError)
		return
	}
	// remove encrypted credentials from response
	settings.EncryptedCredentials = nil

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(settings); err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to encode settings", slog.Any("error", err))
	}
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	siteID := s.getSiteID(r)

	// Validate Authentication from Context (set by authMiddleware)
	user, ok := ctx.Value(userContextKey).(types.User)
	if !ok {
		http.Error(w, "missing authentication", http.StatusUnauthorized)
		return
	}

	if !user.Admin {
		log.Ctx(ctx).WarnContext(ctx, "unauthorized for settings update", slog.String("userID", user.ID), slog.String("email", user.Email))
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}

	var req struct {
		types.Settings
		Franklin *types.FranklinCredentials `json:"franklin,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Ctx(ctx).WarnContext(ctx, "failed to decode settings", slog.Any("error", err))
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	newSettings := req.Settings

	// Basic validation
	if newSettings.AdditionalFeesDollarsPerKWH < 0 ||
		newSettings.MinArbitrageDifferenceDollarsPerKWH < 0 ||
		newSettings.MinBatterySOC < 0 || newSettings.MinBatterySOC > 100 ||
		newSettings.IgnoreHourUsageOverMultiple < 1 ||
		newSettings.SolarBellCurveMultiplier < 0 || newSettings.SolarTrendRatioMax < 1 {
		http.Error(w, "invalid settings values", http.StatusBadRequest)
		return
	}

	// Validate utility provider
	if _, err := s.utilities.Provider(newSettings.UtilityProvider); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Handle credentials update
	if req.Franklin != nil {
		// Get existing credentials to preserve other fields
		existing, _, err := s.storage.GetSettings(ctx, siteID)
		if err != nil {
			log.Ctx(ctx).ErrorContext(ctx, "failed to get settings", slog.Any("error", err))
			http.Error(w, "failed to get settings", http.StatusInternalServerError)
			return
		}

		var existingCreds types.Credentials
		if len(existing.EncryptedCredentials) > 0 {
			existingCreds, err = s.decryptCredentials(ctx, existing.EncryptedCredentials)
			if err != nil {
				log.Ctx(ctx).ErrorContext(ctx, "failed to decrypt credentials", slog.Any("error", err))
				http.Error(w, "failed to decrypt credentials", http.StatusInternalServerError)
				return
			}
		}

		existingCreds.Franklin = req.Franklin

		encrypted, err := s.encryptCredentials(ctx, existingCreds)
		if err != nil {
			log.Ctx(ctx).ErrorContext(ctx, "failed to encrypt credentials", slog.Any("error", err))
			http.Error(w, "failed to encrypt credentials", http.StatusInternalServerError)
			return
		}
		newSettings.EncryptedCredentials = encrypted
	} else {
		// Preserve existing encrypted credentials if not updating
		existing, _, err := s.storage.GetSettings(ctx, siteID)
		if err != nil {
			log.Ctx(ctx).ErrorContext(ctx, "failed to get settings", slog.Any("error", err))
			http.Error(w, "failed to get settings", http.StatusInternalServerError)
			return
		}
		newSettings.EncryptedCredentials = existing.EncryptedCredentials
	}

	if err := s.storage.SetSettings(ctx, siteID, newSettings, types.CurrentSettingsVersion); err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to save settings", slog.Any("error", err))
		http.Error(w, "failed to save settings", http.StatusInternalServerError)
		return
	}

	log.Ctx(ctx).InfoContext(ctx, "settings updated")

	w.WriteHeader(http.StatusOK)
}
