package types

import "time"

// UtilityProviderInfo provides metadata about a utility provider.
type UtilityProviderInfo struct {
	ID    string            `json:"id"`
	Name  string            `json:"name"`
	Rates []UtilityRateInfo `json:"rates"`
}

// UtilityRateInfo provides metadata about a specific utility rate.
type UtilityRateInfo struct {
	ID      string              `json:"id"`
	Name    string              `json:"name"`
	Options []UtilityRateOption `json:"options"`
}

// UtilityOptionType defines the type of input field for a utility option.
type UtilityOptionType string

const (
	UtilityOptionTypeSelect UtilityOptionType = "select"
	UtilityOptionTypeSwitch UtilityOptionType = "switch"
)

// UtilityRateOption represents a single configuration option for a utility rate.
type UtilityRateOption struct {
	Field       string                `json:"field"`
	Name        string                `json:"name"`
	Type        UtilityOptionType     `json:"type"`
	Description string                `json:"description,omitempty"`
	Choices     []UtilityOptionChoice `json:"choices,omitempty"` // Populated if Type is UtilityOptionTypeSelect
	Default     any                   `json:"default,omitempty"`
}

// UtilityOptionChoice represents a single choice in a select-type utility option.
type UtilityOptionChoice struct {
	Value string `json:"value"`
	Name  string `json:"name"`
}

// Price represents the cost of electricity in a time interval.
type Price struct {
	Provider string    `json:"provider"`
	TSStart  time.Time `json:"tsStart"`
	TSEnd    time.Time `json:"tsEnd"`

	// DollarsPerKWH is the base cost of electricity in the time interval.
	DollarsPerKWH float64 `json:"dollarsPerKWH"`

	// GridAddlDollarsPerKWH is the cost of electricity delivered to the home in the time interval.
	// This is added to the base price for grid use.
	GridAddlDollarsPerKWH float64 `json:"gridUseDollarsPerKWH"`

	SampleCount int `json:"-"`
}

// UtilityRateOptions represents the options for the utility rate.
type UtilityRateOptions struct {
	RateClass            string `json:"rateClass"`
	VariableDeliveryRate bool   `json:"variableDeliveryRate"`
}

// UtilityAdditionalFeesPeriod represents a period of time with an additional fee.
type UtilityAdditionalFeesPeriod struct {
	Start          time.Time `json:"start"`
	End            time.Time `json:"end"`
	HourStart      int       `json:"hourStart"`
	HourEnd        int       `json:"hourEnd"`
	DollarsPerKWH  float64   `json:"dollarsPerKWH"`
	GridAdditional bool      `json:"gridAdditional"`
	Description    string    `json:"description"`
}
