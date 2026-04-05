package adapter

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/fs"
	"github.com/JacobJoergensen/preflight/internal/manifest"
	"github.com/JacobJoergensen/preflight/internal/model"
)

type Message = model.Message

type DisplayNamer interface {
	DisplayName() string
}

type Dependencies struct {
	Loader manifest.Loader
	FS     fs.FS
	Runner exec.Runner
	Stream exec.StreamRunner
}

type Adapter interface {
	Name() string
	Check(ctx context.Context, deps Dependencies) (errors []Message, warnings []Message, successes []Message)
}

type DependencyLister interface {
	ListDependencies(ctx context.Context, deps Dependencies) ([]string, error)
}

type Fixer interface {
	Fix(ctx context.Context, deps Dependencies, selectors []string, options FixOptions) (FixItem, error)
}

type FixOptions struct {
	Force      bool
	SkipBackup bool
	DryRun     bool
}

type FixItem struct {
	ScopeID        string
	ManagerCommand string
	ManagerName    string
	Version        string
	Args           []string
	WouldRun       string
	Success        bool
	Error          string
}

var (
	mu        sync.RWMutex
	available = make(map[string]Adapter)
)

var priorities = map[string]int{
	"php": 1, "composer": 2, "node": 3, "js": 4, "go": 5, "python": 6, "ruby": 7, "env": 8,
}

const defaultPriority = 1000

func DisplayName(adapter Adapter) string {
	if adapter == nil {
		return ""
	}

	if displayNamer, ok := adapter.(DisplayNamer); ok {
		name := strings.TrimSpace(displayNamer.DisplayName())

		if name != "" {
			return name
		}
	}

	id := strings.TrimSpace(strings.ToLower(adapter.Name()))

	if id == "" {
		return ""
	}

	return strings.ToUpper(id[:1]) + id[1:]
}

func Register(adapter Adapter) {
	if adapter == nil {
		return
	}

	mu.Lock()
	available[strings.ToLower(adapter.Name())] = adapter
	mu.Unlock()
}

func Select(names ...string) ([]Adapter, error) {
	mu.RLock()
	defer mu.RUnlock()

	selected := make(map[string]Adapter)

	if len(names) == 0 {
		maps.Copy(selected, available)

		return sortByPriority(selected), nil
	}

	var errors []string

	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))

		if name == "" {
			continue
		}

		adapter, exists := available[name]

		if !exists {
			errors = append(errors, "unknown adapter: "+name)
			continue
		}

		selected[name] = adapter
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("selection errors: %s", strings.Join(errors, "; "))
	}

	return sortByPriority(selected), nil
}

func sortByPriority(adapters map[string]Adapter) []Adapter {
	sorted := slices.Collect(maps.Values(adapters))

	slices.SortStableFunc(sorted, func(first, second Adapter) int {
		return cmp.Compare(GetPriority(first.Name()), GetPriority(second.Name()))
	})

	return sorted
}

func GetPriority(name string) int {
	priority, exists := priorities[strings.ToLower(name)]

	if !exists {
		return defaultPriority
	}

	return priority
}

func IsRegistered(name string) bool {
	mu.RLock()
	defer mu.RUnlock()

	_, exists := available[strings.ToLower(strings.TrimSpace(name))]
	return exists
}

func Names(adapters []Adapter) []string {
	names := make([]string, 0, len(adapters))

	for _, adapter := range adapters {
		names = append(names, adapter.Name())
	}

	return names
}
