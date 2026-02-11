package types

import "fmt"

// CurrentSettingsVersion is the current version of the settings struct.
// Increment this value when adding new fields that require default values.
const CurrentSettingsVersion = 3

// Settings represents the configuration stored in the database.
// These are dynamic settings that can be changed without redeploying.
type Settings struct {
	DryRun bool `json:"dryRun"`
	// Pause updates
	Pause bool `json:"pause"`

	// Power History Settings
	// What multiple over previous days to ignore when calculating power usage
	IgnoreHourUsageOverMultiple float64 `json:"ignoreHourUsageOverMultiple"`

	// Price Settings
	// Always charge when the price is under this amount (in $/kWh)
	AlwaysChargeUnderDollarsPerKWH float64 `json:"alwaysChargeUnderDollarsPerKWH"`
	// Additional fees to add to the price when charging (in $/kWh)
	AdditionalFeesDollarsPerKWH            float64 `json:"additionalFeesDollarsPerKWH"`
	MinArbitrageDifferenceDollarsPerKWH    float64 `json:"minArbitrageDifferenceDollarsPerKWH"`
	MinDeficitPriceDifferenceDollarsPerKWH float64 `json:"minDeficitPriceDifferenceDollarsPerKWH"`
	// TODO: add a setting for solar credit value (in $/kWh)

	// The minimum battery SOC should be charged to at all times.
	MinBatterySOC float64 `json:"minBatterySOC"`

	// Grid Settings
	// Maximum Grid Use (in kW) (not supported yet)
	// MaxGridUseKW float64 `json:"maxGridUseKW"`
	// Can charge batteries from grid
	GridChargeBatteries bool `json:"gridChargeBatteries"`
	// Maximum Grid Export (in kW)
	//MaxGridExportKW float64 `json:"maxGridExportKW"`
	// Can export solar to grid
	GridExportSolar bool `json:"gridExportSolar"`
	// Can export batteries to grid (not supported yet)
	//GridExportBatteries bool `json:"gridExportBatteries"`

	// Solar Settings
	// Maximum ratio for solar trend adjustment (caps recentSolar/modelSolar).
	// Higher values allow more aggressive upward solar predictions.
	SolarTrendRatioMax float64 `json:"solarTrendRatioMax"`
	// Multiplier for bell curve solar smoothing weight.
	// 0 disables bell curve smoothing entirely. 1.0 = full weight.
	SolarBellCurveMultiplier float64 `json:"solarBellCurveMultiplier"`
}

// MigrateSettings migrates the settings to the current version.
// It returns the migrated settings, a boolean indicating if changes were made, and an error if migration failed.
func MigrateSettings(s Settings, currentVersion int) (Settings, bool, error) {
	if currentVersion >= CurrentSettingsVersion {
		return s, false, nil
	}

	migrated := false
	// Loop through versions to apply migrations sequentially
	for version := currentVersion + 1; version <= CurrentSettingsVersion; version++ {
		switch version {
		case 1:
			// version 1: initial
			if s.IgnoreHourUsageOverMultiple == 0 {
				s.IgnoreHourUsageOverMultiple = 2
				migrated = true
			}
			if s.AlwaysChargeUnderDollarsPerKWH == 0 {
				s.AlwaysChargeUnderDollarsPerKWH = 0.05
				migrated = true
			}
			if s.MinArbitrageDifferenceDollarsPerKWH == 0 {
				s.MinArbitrageDifferenceDollarsPerKWH = 0.03
				migrated = true
			}
			if s.MinBatterySOC == 0 {
				s.MinBatterySOC = 20.0
				migrated = true
			}
			// we don't want to assume they can charge from grid or export to grid
		case 2:
			// version 2: add MinDeficitPriceDifferenceDollarsPerKWH
			if s.MinDeficitPriceDifferenceDollarsPerKWH == 0 {
				s.MinDeficitPriceDifferenceDollarsPerKWH = 0.02
				migrated = true
			}
		case 3:
			// version 3: add solar trend ratio max and bell curve multiplier
			if s.SolarTrendRatioMax == 0 {
				s.SolarTrendRatioMax = 3.0
				migrated = true
			}
			if s.SolarBellCurveMultiplier == 0 {
				s.SolarBellCurveMultiplier = 1.0
				migrated = true
			}
		default:
			return s, false, fmt.Errorf("unknown settings version: %d", version)
		}
	}

	return s, migrated, nil
}
