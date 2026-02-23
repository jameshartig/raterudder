package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateSettings(t *testing.T) {
	t.Run("v1: initial defaults", func(t *testing.T) {
		s, changed, err := MigrateSettings(Settings{}, 0)
		require.NoError(t, err)
		assert.True(t, changed)
		assert.Equal(t, 2.0, s.IgnoreHourUsageOverMultiple)
		assert.Equal(t, 0.03, s.MinArbitrageDifferenceDollarsPerKWH)
		assert.Equal(t, 20.0, s.MinBatterySOC)
	})

	t.Run("v5 to v6: release production", func(t *testing.T) {
		s, changed, err := MigrateSettings(Settings{Release: ""}, 5)
		require.NoError(t, err)
		assert.True(t, changed)
		assert.Equal(t, "production", s.Release)
	})

	t.Run("v5 to v7: comed_hourly to comed/comed_besh", func(t *testing.T) {
		old := Settings{
			UtilityProvider: "comed_hourly",
		}
		s, changed, err := MigrateSettings(old, 4)
		require.NoError(t, err)
		assert.True(t, changed)
		// v5 change: comed_hourly -> comed_besh
		// v7 change: comed_besh -> (comed, comed_besh)
		assert.Equal(t, "comed", s.UtilityProvider)
		assert.Equal(t, "comed_besh", s.UtilityRate)
	})

	t.Run("v6 to v7: comed_besh to comed/comed_besh", func(t *testing.T) {
		old := Settings{
			UtilityProvider: "comed_besh",
			UtilityRateOptions: UtilityRateOptions{
				RateClass: "singleFamilyWithoutElectricHeat",
			},
		}
		s, changed, err := MigrateSettings(old, 6)
		require.NoError(t, err)
		assert.True(t, changed)
		assert.Equal(t, "comed", s.UtilityProvider)
		assert.Equal(t, "comed_besh", s.UtilityRate)
		assert.Equal(t, "singleFamilyWithoutElectricHeat", s.UtilityRateOptions.RateClass)
	})

	t.Run("no change: current version", func(t *testing.T) {
		current := Settings{
			UtilityProvider: "comed",
			UtilityRate:     "comed_besh",
			Release:         "production",
		}
		s, changed, err := MigrateSettings(current, CurrentSettingsVersion)
		require.NoError(t, err)
		assert.False(t, changed)
		assert.Equal(t, current, s)
	})
}
