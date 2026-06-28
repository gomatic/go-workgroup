package workgroup

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

// Source generates work items by sending them to the channel.
type Source[T any] func(context.Context, chan<- T) error

// Worker processes a single work item. id is the 0-based worker index.
type Worker[T any] func(context.Context, int, T) error

// Transformer maps an input item to an output item within a Pipe stage.
type Transformer[In, Out any] func(context.Context, int, In) (Out, error)

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
func runSource[T any](ctx context.Context, cancel context.CancelCauseFunc, source Source[T], work chan<- T) func() error {
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
func runWorkers[T any](ctx context.Context, cancel context.CancelCauseFunc, cfg settings, worker Worker[T], work <-chan T) []error {
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
			consume(ctx, cancel, cfg.onError, worker, i, work, record)
		}()
	}
	wg.Wait()
	return errs
}

// consume drains the work channel for a single worker, invoking worker on
// each item and recording any error. In FailFast mode the group is cancelled
// and the worker stops on the first error.
func consume[T any](ctx context.Context, cancel context.CancelCauseFunc, mode onError, worker Worker[T], id int, work <-chan T, record func(error)) {
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

// FanOut distributes work across n workers. Equivalent to Run with Workers(n).
func FanOut[T any](ctx context.Context, n int, source Source[T], worker Worker[T], opts ...Optional) error {
	return Run(ctx, source, worker, append([]Optional{Workers(n)}, opts...)...)
}

// FanIn processes work with exactly 1 worker. Equivalent to Run with Workers(1).
func FanIn[T any](ctx context.Context, source Source[T], worker Worker[T], opts ...Optional) error {
	return Run(ctx, source, worker, append([]Optional{Workers(1)}, opts...)...)
}

// Pipe creates a Source from a transformation, enabling stage chaining.
// The returned Source, when consumed by a downstream Run, executes the
// upstream source with n workers applying transform to each item.
func Pipe[In, Out any](n int, source Source[In], transform Transformer[In, Out], opts ...Optional) Source[Out] {
	return func(ctx context.Context, out chan<- Out) error {
		worker := Worker[In](func(ctx context.Context, id int, item In) error {
			result, err := transform(ctx, id, item)
			if err != nil {
				return err
			}
			return send(ctx, out, result)
		})
		return Run(ctx, source, worker, append([]Optional{Workers(n)}, opts...)...)
	}
}

// drain discards any items still pending on work until it is closed. It runs
// after every worker has returned, so a non-cooperative source — one that
// ignores ctx and blocks on a raw send — has its pending send unblocked,
// letting the source observe cancellation, return, and close work. Without it,
// such a send would deadlock and Run would hang on the source's wg.Wait. On the
// happy path the channel is already closed and drained, so this returns at once.
func drain[T any](work <-chan T) {
	for {
		if _, ok := <-work; !ok {
			return
		}
	}
}

// send delivers item downstream, honouring context cancellation.
func send[T any](ctx context.Context, out chan<- T, item T) error {
	select {
	case out <- item:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// contextError reports whether err is a context cancellation/deadline error.
func contextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
