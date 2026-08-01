package workgroup

import (
	"context"
	"log/slog"
	"sync"
)

// WorkerID is the 0-based index identifying a worker goroutine within a group.
type WorkerID int

// Source generates work items by sending them to the channel.
type Source[T any] func(context.Context, chan<- T) error

// Worker processes a single work item. id is the 0-based worker index.
type Worker[T any] func(context.Context, WorkerID, T) error

// Transformer maps an input item to an output item within a Pipe stage.
type Transformer[In, Out any] func(context.Context, WorkerID, In) (Out, error)

// Run distributes work from source across N workers (default: NumCPU).
// Blocks until all work is processed or context is cancelled.
// Returns nil on success, or the first/joined error(s) on failure.
func Run[T any](ctx context.Context, source Source[T], worker Worker[T], opts ...Optional) error {
	if source == nil {
		return ErrNilSource
	}
	if worker == nil {
		return ErrNilWorker
	}

	cfg := newSettings(opts)
	attrs := cfg.logAttrs()
	cfg.logger.LogAttrs(ctx, slog.LevelInfo, "workgroup starting", attrs...)

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	work := make(chan T)
	produced := runSource(ctx, cancel, source, work)
	errs := runWorkers(ctx, cancel, cfg, worker, work)
	drain(work)
	sourceErr := produced()

	return cfg.outcome(ctx, attrs, errs, sourceErr)
}

// runSource starts the source goroutine that feeds the work channel. It
// cancels the group with the source error on failure and always closes the
// channel so workers terminate. The returned function blocks until the source
// goroutine has returned and yields its error.
func runSource[T any](
	ctx context.Context,
	cancel context.CancelCauseFunc,
	source Source[T],
	work chan<- T,
) func() error {
	var wg sync.WaitGroup
	var sourceErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(work)
		if err := source(ctx, work); err != nil {
			sourceErr = err
			cancel(err)
		}
	}()
	return func() error {
		wg.Wait()
		return sourceErr
	}
}

// runWorkers starts cfg.workers consumer goroutines and returns their
// collected errors once every worker has drained the channel.
func runWorkers[T any](
	ctx context.Context,
	cancel context.CancelCauseFunc,
	cfg settings,
	worker Worker[T],
	work <-chan T,
) []error {
	var mu sync.Mutex
	var errs []error
	record := func(err error) {
		mu.Lock()
		errs = append(errs, err)
		mu.Unlock()
	}

	var wg sync.WaitGroup
	wg.Add(cfg.workers)
	for i := range cfg.workers {
		go func() {
			defer wg.Done()
			consume(ctx, cancel, cfg.onError, worker, WorkerID(i), work, record)
		}()
	}
	wg.Wait()
	return errs
}

// consume drains the work channel for a single worker, invoking worker on
// each item and recording any error. In FailFast mode the group is cancelled
// and the worker stops on the first error.
func consume[T any](
	ctx context.Context,
	cancel context.CancelCauseFunc,
	mode onError,
	worker Worker[T],
	id WorkerID,
	work <-chan T,
	record func(error),
) {
	for item := range work {
		if ctx.Err() != nil {
			return
		}
		err := worker(ctx, id, item)
		if err == nil {
			continue
		}
		record(err)
		if mode == FailFast {
			cancel(err)
			return
		}
	}
}
