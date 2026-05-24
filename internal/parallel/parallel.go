package parallel

import (
	"context"
	"runtime"
	"sync"
)

// Collect runs work over jobs with a bounded number of workers and returns the
// results for which work reported include=true. It stops scheduling new work
// once ctx is canceled.
func Collect[Job, Result any](ctx context.Context, jobs []Job, work func(context.Context, Job) (Result, bool)) []Result {
	if len(jobs) == 0 {
		return nil
	}

	results := make([]Result, len(jobs))
	includes := make([]bool, len(jobs))
	semaphore := make(chan struct{}, workerCount(len(jobs)))

	var wg sync.WaitGroup

	wg.Add(len(jobs))

	for index, job := range jobs {
		go func(index int, job Job) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}

			defer func() { <-semaphore }()

			if ctx.Err() != nil {
				return
			}

			item, include := work(ctx, job)

			if !include {
				return
			}

			results[index] = item
			includes[index] = true
		}(index, job)
	}

	wg.Wait()

	output := make([]Result, 0, len(jobs))

	for i, include := range includes {
		if include {
			output = append(output, results[i])
		}
	}

	return output
}

func workerCount(jobCount int) int {
	if jobCount <= 0 {
		return 1
	}

	const maxWorkers = 8

	workers := runtime.GOMAXPROCS(0)
	workers = max(workers, 1)
	workers = min(workers, maxWorkers)
	workers = min(workers, jobCount)

	return workers
}
