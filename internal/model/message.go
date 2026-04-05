package model

type Message struct {
	Text   string `json:"text"`
	Nested bool   `json:"nested"`
	Dev    bool   `json:"dev,omitempty"` // Only meaningful when Nested is true, indicates a dev-only dependency
}
