package model

type Message struct {
	Text     string `json:"text"`
	Nested   bool   `json:"nested"`
	Dev      bool   `json:"dev,omitempty"`      // Only meaningful when Nested is true, indicates a dev-only dependency
	Optional bool   `json:"optional,omitempty"` // Only meaningful when Nested is true, indicates an optional dependency
	Info     bool   `json:"info,omitempty"`     // Only meaningful when Nested is true, indicates an informational note rather than an installed entry
}
