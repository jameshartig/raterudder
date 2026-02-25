package types

import "time"

// ESSProviderInfo provides metadata about an Energy Storage System (ESS) provider.
type ESSProviderInfo struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Credentials []ESSCredential `json:"credentials"`
	Hidden      bool            `json:"hidden,omitempty"`
}

// ESSCredential defines a single configuration/credential option for an ESS.
type ESSCredential struct {
	Field       string `json:"field"`
	Name        string `json:"name"`
	Type        string `json:"type"` // e.g. "string" or "password"
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}

// ESSMockState represents the internal state of a mock ESS provider.
type ESSMockState struct {
	Timestamp    time.Time              `json:"timestamp"`
	BatterySOC   float64                `json:"batterySOC"`
	BatteryMode  BatteryMode            `json:"batteryMode"`
	SolarMode    SolarMode              `json:"solarMode"`
	DailyHistory map[string]EnergyStats `json:"dailyHistory"`
}
