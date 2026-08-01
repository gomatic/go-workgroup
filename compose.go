package workgroup

import (
	"context"
	"errors"
)

// The composition helpers: FanOut, FanIn and Pipe. Each is a thin, exact
// spelling of a Run configuration, and "exact" is the contract — a caller
// reaches for FanIn precisely because the work is not safe to run
// concurrently, so a helper that quietly widened it would introduce the race
// the caller chose it to avoid.

// FanOut distributes work across n workers. Equivalent to Run with Workers(n).
func FanOut[T any](ctx context.Context, n Workers, source Source[T], worker Worker[T], opts ...Optional) error {
	return Run(ctx, source, worker, append([]Optional{n}, opts...)...)
}

// FanIn processes work with exactly 1 worker. Equivalent to Run with Workers(1).
func FanIn[T any](ctx context.Context, source Source[T], worker Worker[T], opts ...Optional) error {
	return Run(ctx, source, worker, append([]Optional{Workers(1)}, opts...)...)
}

// Pipe creates a Source from a transformation, enabling stage chaining.
// The returned Source, when consumed by a downstream Run, executes the
// upstream source with n workers applying transform to each item.
func Pipe[In, Out any](n Workers, source Source[In], transform Transformer[In, Out], opts ...Optional) Source[Out] {
	return func(ctx context.Context, out chan<- Out) error {
		worker := Worker[In](func(ctx context.Context, id WorkerID, item In) error {
			result, err := transform(ctx, id, item)
			if err != nil {
				return err
			}
			return send(ctx, out, result)
		})
		return Run(ctx, source, worker, append([]Optional{n}, opts...)...)
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
