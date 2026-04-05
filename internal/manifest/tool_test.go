package manifest

import (
	"testing"
)

func TestGetTool(t *testing.T) {
	tests := []struct {
		command   string
		wantExist bool
		wantCmd   string
	}{
		{"npm", true, "npm"},
		{"NPM", true, "npm"},
		{"composer", true, "composer"},
		{"go", true, "go"},
		{"unknown", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			tool, exists := GetTool(tt.command)

			if exists != tt.wantExist {
				t.Errorf("exists = %v, want %v", exists, tt.wantExist)
			}

			if exists && tool.Command != tt.wantCmd {
				t.Errorf("command = %q, want %q", tool.Command, tt.wantCmd)
			}
		})
	}
}

func TestGetPackageType(t *testing.T) {
	tests := []struct {
		command  string
		wantType string
		wantOK   bool
	}{
		{"npm", PackageTypeJS, true},
		{"yarn", PackageTypeJS, true},
		{"composer", PackageTypeComposer, true},
		{"go", PackageTypeGo, true},
		{"poetry", PackageTypePython, true},
		{"bundle", PackageTypeRuby, true},
		{"php", "", false},
		{"node", "", false},
		{"unknown", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			pt, ok := GetPackageType(tt.command)

			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}

			if pt != tt.wantType {
				t.Errorf("packageType = %q, want %q", pt, tt.wantType)
			}
		})
	}
}

func TestIsPackageType(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{PackageTypeJS, true},
		{PackageTypeComposer, true},
		{PackageTypeGo, true},
		{PackageTypePython, true},
		{PackageTypeRuby, true},
		{"npm", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPackageType(tt.name)

			if got != tt.want {
				t.Errorf("IsPackageType(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestResolvePackageType(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"npm", PackageTypeJS},
		{"yarn", PackageTypeJS},
		{"composer", PackageTypeComposer},
		{"js", "js"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolvePackageType(tt.name)

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAnyMatchesPackageType(t *testing.T) {
	tests := []struct {
		name        string
		commands    []string
		packageType string
		want        bool
	}{
		{
			name:        "npm matches js",
			commands:    []string{"npm", "composer"},
			packageType: PackageTypeJS,
			want:        true,
		},
		{
			name:        "no match",
			commands:    []string{"npm", "yarn"},
			packageType: PackageTypeComposer,
			want:        false,
		},
		{
			name:        "empty commands",
			commands:    []string{},
			packageType: PackageTypeJS,
			want:        false,
		},
		{
			name:        "unknown commands",
			commands:    []string{"unknown", "invalid"},
			packageType: PackageTypeJS,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnyMatchesPackageType(tt.commands, tt.packageType)

			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetToolsByPackageType(t *testing.T) {
	jsTools := GetToolsByPackageType(PackageTypeJS)

	if len(jsTools) == 0 {
		t.Fatal("expected JS tools, got none")
	}

	for _, tool := range jsTools {
		if tool.PackageType != PackageTypeJS {
			t.Errorf("tool %q has wrong package type %q", tool.Command, tool.PackageType)
		}
	}

	// Verify sorted by command
	for i := 1; i < len(jsTools); i++ {
		if jsTools[i-1].Command > jsTools[i].Command {
			t.Errorf("tools not sorted: %q > %q", jsTools[i-1].Command, jsTools[i].Command)
		}
	}
}
