package utility

import (
	"log/slog"
	"testing"

	"github.com/raterudder/raterudder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/raterudder/raterudder/pkg/log"
)

func init() {
	log.SetDefaultLogLevel(slog.LevelError)
}

func TestListUtilities(t *testing.T) {
	m := NewMap()
	utilities := m.ListUtilities()

	require.NotEmpty(t, utilities, "expected at least one utility")

	ids := make(map[string]bool)
	for _, u := range utilities {
		assert.NotEmpty(t, u.ID, "utility must have an ID")
		assert.NotEmpty(t, u.Name, "utility must have a name")
		assert.False(t, ids[u.ID], "utility IDs must be unique, duplicate: %s", u.ID)
		ids[u.ID] = true

		for _, rate := range u.Rates {
			assert.NotEmpty(t, rate.ID, "rate must have an ID")
			assert.NotEmpty(t, rate.Name, "rate must have a name")

			for _, opt := range rate.Options {
				assert.NotEmpty(t, opt.Field, "option must have a Field")
				assert.NotEmpty(t, opt.Name, "option must have a name")
				assert.True(t, opt.Type == types.UtilityOptionTypeSelect || opt.Type == types.UtilityOptionTypeSwitch,
					"option type must be 'select' or 'switch', got %q", opt.Type)

				if opt.Type == types.UtilityOptionTypeSelect {
					assert.NotEmpty(t, opt.Choices, "select option %q must have choices", opt.Field)
					for _, c := range opt.Choices {
						assert.NotEmpty(t, c.Value, "choice must have a Value in option %q", opt.Field)
						assert.NotEmpty(t, c.Name, "choice must have a name in option %q", opt.Field)
					}
					assert.NotNil(t, opt.Default, "select option %q must have a default", opt.Field)
				}
			}
		}
	}
}

func TestComEdUtilityInfo(t *testing.T) {
	info := comEdUtilityInfo()

	assert.Equal(t, "comed", info.ID)
	assert.Equal(t, "ComEd", info.Name)
	require.Len(t, info.Rates, 1, "comed should have exactly 1 rate")

	rate := info.Rates[0]
	assert.Equal(t, "comed_besh", rate.ID)
	assert.NotEmpty(t, rate.Name)
	require.Len(t, rate.Options, 3, "comed_besh should have exactly 3 options")

	t.Run("RateClass option", func(t *testing.T) {
		opt := rate.Options[0]
		assert.Equal(t, "rateClass", opt.Field)
		assert.Equal(t, types.UtilityOptionTypeSelect, opt.Type)
		require.Len(t, opt.Choices, 4, "rateClass should have 4 choices")

		choiceValues := make([]string, len(opt.Choices))
		for i, c := range opt.Choices {
			choiceValues[i] = c.Value
		}
		assert.Contains(t, choiceValues, ComEdRateClassSingleFamilyResidenceWithoutElectricSpaceHeat)
		assert.Contains(t, choiceValues, ComEdRateClassMultiFamilyResidenceWithoutElectricSpaceHeat)
		assert.Contains(t, choiceValues, ComEdRateClassSingleFamilyResidenceWithElectricSpaceHeat)
		assert.Contains(t, choiceValues, ComEdRateClassMultiFamilyResidenceWithElectricSpaceHeat)

		assert.Equal(t, ComEdRateClassSingleFamilyResidenceWithoutElectricSpaceHeat, opt.Default)
	})

	t.Run("VariableDeliveryRate option", func(t *testing.T) {
		opt := rate.Options[1]
		assert.Equal(t, "variableDeliveryRate", opt.Field)
		assert.Equal(t, types.UtilityOptionTypeSwitch, opt.Type)
		assert.NotEmpty(t, opt.Description)
		assert.Equal(t, false, opt.Default)
	})
}
