package fetcher

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ParallelFetcher fetches multiple artifacts concurrently.
type ParallelFetcher struct {
	fetcher    *Fetcher
	maxWorkers int
	verbose    bool
}

// FetchJob represents a single fetch task.
type FetchJob struct {
	GAV        string
	Classifier string // Optional classifier (for artifacts)
	Type       string // "pom" or "artifact"
	Result     chan *FetchJobResult
}

// FetchJobResult contains the result of a fetch operation.
type FetchJobResult struct {
	GAV      string
	Data     []byte
	Error    error
	Cached   bool
	Duration time.Duration
	Attempt  int
}

// FetchProgress tracks fetching progress.
type FetchProgress struct {
	Total     int64
	Completed int64
	Failed    int64
	Cached    int64
	StartTime time.Time
}

// NewParallelFetcher creates a parallel fetcher.
func NewParallelFetcher(fetcher *Fetcher, maxWorkers int, verbose bool) *ParallelFetcher {
	if maxWorkers <= 0 {
		maxWorkers = 4 // Default
	}
	return &ParallelFetcher{
		fetcher:    fetcher,
		maxWorkers: maxWorkers,
		verbose:    verbose,
	}
}

// FetchMany fetches multiple POMs or artifacts in parallel.
func (pf *ParallelFetcher) FetchMany(ctx context.Context, gavs []string, fetchType string) (map[string][]byte, error) {
	if len(gavs) == 0 {
		return make(map[string][]byte), nil
	}

	progress := &FetchProgress{
		Total:     int64(len(gavs)),
		StartTime: time.Now(),
	}

	// Create job queue and result map
	jobs := make(chan *FetchJob, len(gavs))
	results := make(map[string]*FetchJobResult)
	resultsMu := sync.Mutex{}
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < pf.maxWorkers; i++ {
		wg.Add(1)
		go pf.worker(ctx, i, jobs, &results, &resultsMu, progress, &wg)
	}

	// Enqueue jobs
	go func() {
		for _, gav := range gavs {
			select {
			case <-ctx.Done():
				return
			case jobs <- &FetchJob{
				GAV:  gav,
				Type: fetchType,
			}:
			}
		}
		close(jobs)
	}()

	// Wait for completion
	wg.Wait()

	// Collect results
	data := make(map[string][]byte)
	for gav, result := range results {
		if result.Error != nil {
			return nil, fmt.Errorf("fetch %s failed: %w", gav, result.Error)
		}
		data[gav] = result.Data
	}

	duration := time.Since(progress.StartTime)
	if pf.verbose {
		fmt.Printf("✓ Fetched %d artifacts in %v (%d cached)\n",
			atomic.LoadInt64(&progress.Completed),
			duration,
			atomic.LoadInt64(&progress.Cached))
	}

	return data, nil
}

// worker processes jobs from the queue.
func (pf *ParallelFetcher) worker(ctx context.Context, id int, jobs <-chan *FetchJob, results *map[string]*FetchJobResult, mu *sync.Mutex, progress *FetchProgress, wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
		if r := recover(); r != nil {
			fmt.Printf("Worker %d panicked: %v\n", id, r)
		}
	}()

	for job := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		startTime := time.Now()
		var data []byte
		var err error

		// Fetch based on type
		if job.Type == "pom" {
			data, err = pf.fetcher.FetchPOM(ctx, job.GAV)
		} else {
			data, err = pf.fetcher.FetchArtifact(ctx, job.GAV, job.Classifier)
		}

		// Determine if cached
		cached := err == nil && len(data) > 0

		duration := time.Since(startTime)

		// Record result
		result := &FetchJobResult{
			GAV:      job.GAV,
			Data:     data,
			Error:    err,
			Cached:   cached,
			Duration: duration,
		}

		mu.Lock()
		(*results)[job.GAV] = result
		mu.Unlock()

		// Update progress
		if err == nil {
			atomic.AddInt64(&progress.Completed, 1)
			if cached {
				atomic.AddInt64(&progress.Cached, 1)
			}
		} else {
			atomic.AddInt64(&progress.Failed, 1)
		}

		if pf.verbose && err == nil {
			fmt.Printf("Worker %d: %s (%.2fs, cached=%v)\n", id, job.GAV, duration.Seconds(), cached)
		}
	}
}

// FetchPOMsParallel fetches multiple POMs concurrently.
func (pf *ParallelFetcher) FetchPOMsParallel(ctx context.Context, gavs []string) (map[string][]byte, error) {
	return pf.FetchMany(ctx, gavs, "pom")
}

// FetchArtifactsParallel fetches multiple artifacts concurrently.
func (pf *ParallelFetcher) FetchArtifactsParallel(ctx context.Context, gavs []string) (map[string][]byte, error) {
	return pf.FetchMany(ctx, gavs, "artifact")
}
