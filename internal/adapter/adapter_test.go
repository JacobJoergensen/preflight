package adapter

import (
	"context"
	"slices"
	"testing"
)

type mockAdapter struct {
	name        string
	displayName string
}

type mockAdapterNoDisplay struct {
	name string
}

func (m mockAdapter) Name() string {
	return m.name
}

func (m mockAdapter) DisplayName() string {
	return m.displayName
}

func (m mockAdapter) Check(_ context.Context, _ Dependencies) ([]Message, []Message, []Message) {
	return nil, nil, nil
}

func (m mockAdapterNoDisplay) Name() string {
	return m.name
}

func (m mockAdapterNoDisplay) Check(_ context.Context, _ Dependencies) ([]Message, []Message, []Message) {
	return nil, nil, nil
}

func TestGetPriority(t *testing.T) {
	tests := []struct {
		name    string
		want    int
		isKnown bool
	}{
		{"php", 1, true},
		{"composer", 2, true},
		{"node", 3, true},
		{"js", 4, true},
		{"go", 5, true},
		{"python", 6, true},
		{"ruby", 7, true},
		{"env", 8, true},
		{"PHP", 1, true}, // case insensitive
		{"unknown", 1000, false},
		{"", 1000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPriority(tt.name)

			if got != tt.want {
				t.Errorf("GetPriority(%q) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestNames(t *testing.T) {
	tests := []struct {
		name     string
		adapters []Adapter
		want     []string
	}{
		{
			name:     "empty slice",
			adapters: []Adapter{},
			want:     []string{},
		},
		{
			name:     "single adapter",
			adapters: []Adapter{mockAdapter{name: "php"}},
			want:     []string{"php"},
		},
		{
			name: "multiple adapters",
			adapters: []Adapter{
				mockAdapter{name: "php"},
				mockAdapter{name: "node"},
				mockAdapter{name: "go"},
			},
			want: []string{"php", "node", "go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Names(tt.adapters)

			if !slices.Equal(got, tt.want) {
				t.Errorf("Names() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDisplayName(t *testing.T) {
	tests := []struct {
		name    string
		adapter Adapter
		want    string
	}{
		{
			name:    "nil adapter",
			adapter: nil,
			want:    "",
		},
		{
			name:    "uses DisplayName method when available",
			adapter: mockAdapter{name: "php", displayName: "PHP Runtime"},
			want:    "PHP Runtime",
		},
		{
			name:    "falls back to capitalized name",
			adapter: mockAdapterNoDisplay{name: "composer"},
			want:    "Composer",
		},
		{
			name:    "handles empty display name",
			adapter: mockAdapter{name: "node", displayName: ""},
			want:    "Node",
		},
		{
			name:    "handles whitespace display name",
			adapter: mockAdapter{name: "go", displayName: "   "},
			want:    "Go",
		},
		{
			name:    "handles empty name",
			adapter: mockAdapterNoDisplay{name: ""},
			want:    "",
		},
		{
			name:    "handles whitespace name",
			adapter: mockAdapterNoDisplay{name: "   "},
			want:    "",
		},
		{
			name:    "capitalizes lowercase name",
			adapter: mockAdapterNoDisplay{name: "ruby"},
			want:    "Ruby",
		},
		{
			name:    "handles uppercase name",
			adapter: mockAdapterNoDisplay{name: "PHP"},
			want:    "Php",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DisplayName(tt.adapter)

			if got != tt.want {
				t.Errorf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}
