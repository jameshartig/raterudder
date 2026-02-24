package types

import (
	"fmt"
	"time"
)

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

	// GridUseDollarsPerKWH is the additional cost of electricity for it to be
	// delivered to the home via the grid in the time interval. This is added to
	// the base price when using the grid.
	GridUseDollarsPerKWH float64 `json:"gridUseDollarsPerKWH"`

	SampleCount int `json:"-"`
}

// UtilityRateOptions represents the options for the utility rate.
type UtilityRateOptions struct {
	RateClass            string `json:"rateClass"`
	VariableDeliveryRate bool   `json:"variableDeliveryRate"`
	NetMeteringCredits   bool   `json:"netMeteringCredits"`
}

// UtilityPeriod defines a particular schedule for some utility rate or fee
type UtilityPeriod struct {
	Start         time.Time      `json:"start"`
	End           time.Time      `json:"end"`
	HourStart     int            `json:"hourStart"`
	HourEnd       int            `json:"hourEnd"`
	DaysOfTheWeek []time.Weekday `json:"daysOfTheWeek"`
	Location      string         `json:"location"`
	LocationPtr   *time.Location `json:"-"`
}

// Contains checks if a time is within the period.
func (p *UtilityPeriod) Contains(t time.Time) (bool, error) {
	if p.LocationPtr != nil {
		t = t.In(p.LocationPtr)
	} else if p.Location != "" {
		loc, err := time.LoadLocation(p.Location)
		if err != nil {
			return false, fmt.Errorf("failed to load location %s: %w", p.Location, err)
		}
		t = t.In(loc)
	}
	if !p.Start.IsZero() && t.Before(p.Start) {
		return false, nil
	}
	if !p.End.IsZero() && t.After(p.End) {
		return false, nil
	}
	if h := t.Hour(); h < p.HourStart || h >= p.HourEnd {
		return false, nil
	}
	if len(p.DaysOfTheWeek) > 0 {
		var found bool
		dow := t.Weekday()
		for _, d := range p.DaysOfTheWeek {
			if d == dow {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}

// UtilityAdditionalFeesPeriod represents a period of time with an additional fee.
type UtilityAdditionalFeesPeriod struct {
	UtilityPeriod
	DollarsPerKWH  float64 `json:"dollarsPerKWH"`
	GridAdditional bool    `json:"gridAdditional"`
	Description    string  `json:"description"`
}
