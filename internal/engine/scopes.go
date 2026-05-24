package engine

import (
	"context"
	"slices"
	"time"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/parallel"
)

// scopeRun is the shared envelope produced when an operation runs across every
// selected scope: the items it kept plus timing and cancellation.
type scopeRun[T any] struct {
	Items     []T
	StartedAt time.Time
	EndedAt   time.Time
	Canceled  bool
}

// runScopes drives op across specs concurrently, reporting progress, dropping
// scopes that do not resolve or that op excludes, and sorting the kept items.
// Check and audit differ only in op and less; everything else is shared here.
func runScopes[T any](
	ctx context.Context,
	specs []*ecosystem.Spec,
	rc ecosystem.RunContext,
	progress ScanProgress,
	op func(ctx context.Context, spec *ecosystem.Spec, detection ecosystem.Detection) (T, bool),
	less func(a, b T) int,
) scopeRun[T] {
	startedAt := time.Now()

	progress.Plan(len(specs))

	items := parallel.Collect(ctx, specs, func(ctx context.Context, spec *ecosystem.Spec) (T, bool) {
		scopeID := spec.Name

		progress.Start(scopeID, spec.Title())

		var included bool
		defer func() { progress.Finish(scopeID, included) }()

		detection, ok := spec.Resolve(rc)

		if !ok {
			var zero T
			return zero, false
		}

		item, ok := op(ctx, spec, detection)

		if !ok {
			var zero T
			return zero, false
		}

		included = true

		return item, true
	})

	if less != nil {
		slices.SortFunc(items, less)
	}

	return scopeRun[T]{
		Items:     items,
		StartedAt: startedAt,
		EndedAt:   time.Now(),
		Canceled:  ctx.Err() != nil,
	}
}
