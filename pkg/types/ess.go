package types

// ESSProviderInfo provides metadata about an Energy Storage System (ESS) provider.
type ESSProviderInfo struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Credentials []ESSCredential `json:"credentials"`
}

// ESSCredential defines a single configuration/credential option for an ESS.
type ESSCredential struct {
	Field       string `json:"field"`
	Name        string `json:"name"`
	Type        string `json:"type"` // e.g. "string" or "password"
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}
