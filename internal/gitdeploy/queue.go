package gitdeploy

import (
	"context"
	"log/slog"
	"sync"
)

// maxConcurrentBuilds is the maximum number of builds that can run simultaneously.
const maxConcurrentBuilds = 2

// queueBufferSize is the size of the build job channel buffer.
const queueBufferSize = 100

// BuildJob represents a build to be processed.
type BuildJob struct {
	BuildID   int64
	RepoURL   string
	CommitSHA string
	Branch    string
}

// Queue manages a buffered channel of build jobs with a semaphore-based
// concurrency limit. At most maxConcurrentBuilds run simultaneously.
type Queue struct {
	builder  *Builder
	jobs     chan BuildJob
	sem      chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once
	done     chan struct{}
}

// NewQueue creates a new build queue.
func NewQueue(builder *Builder) *Queue {
	return &Queue{
		builder: builder,
		jobs:    make(chan BuildJob, queueBufferSize),
		sem:     make(chan struct{}, maxConcurrentBuilds),
		done:    make(chan struct{}),
	}
}

// Start begins processing jobs from the queue. It spawns a goroutine that
// reads jobs and dispatches them with bounded concurrency.
func (q *Queue) Start() {
	go q.processLoop()
	slog.Info("build queue started",
		"max_concurrent", maxConcurrentBuilds,
		"buffer_size", queueBufferSize,
	)
}

// Stop gracefully shuts down the queue. It stops accepting new jobs and
// waits for all in-progress builds to complete.
func (q *Queue) Stop() {
	q.stopOnce.Do(func() {
		close(q.done)
		q.wg.Wait()
		slog.Info("build queue stopped")
	})
}

// Enqueue adds a build job to the queue. If the queue is full, the job
// is dropped and a warning is logged.
func (q *Queue) Enqueue(job BuildJob) {
	select {
	case q.jobs <- job:
		slog.Debug("job enqueued", "build_id", job.BuildID)
	default:
		slog.Warn("build queue full, dropping job",
			"build_id", job.BuildID,
			"repo", job.RepoURL,
		)
	}
}

// Len returns the number of jobs waiting in the queue.
func (q *Queue) Len() int {
	return len(q.jobs)
}

// processLoop reads jobs from the channel and dispatches them with
// bounded concurrency via the semaphore.
func (q *Queue) processLoop() {
	for {
		select {
		case <-q.done:
			return
		case job := <-q.jobs:
			// Acquire semaphore slot.
			select {
			case <-q.done:
				return
			case q.sem <- struct{}{}:
			}

			q.wg.Add(1)
			go func(j BuildJob) {
				defer q.wg.Done()
				defer func() { <-q.sem }()

				ctx := context.Background()
				if err := q.builder.Build(ctx, j); err != nil {
					slog.Error("build failed",
						"build_id", j.BuildID,
						"repo", j.RepoURL,
						"error", err,
					)
				}
			}(job)
		}
	}
}
