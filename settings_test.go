package workgroup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConsumeCancelGuard exercises the in-loop cancellation guard of consume
// directly and deterministically: the work channel already holds an item when
// the context is cancelled, so the loop body observes ctx.Err() and returns
// without invoking the worker again. A white-box test is used because, by
// design, Run never delivers an item to a worker after its context is
// cancelled (its source honours ctx.Done), making the guard unreachable
// through Run's public surface without a data race.
func TestConsumeCancelGuard(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before any item is consumed

	work := make(chan int, 1)
	work <- 42
	close(work)

	var calls atomic.Int64
	worker := Worker[int](func(context.Context, WorkerID, int) error {
		calls.Add(1)
		return nil
	})

	consume(ctx, func(error) {}, FailFast, worker, 0, work, func(error) {})
	assert.Zero(t, calls.Load(), "guard returns before invoking the worker")
}

func TestRunContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var processed atomic.Int64

	// A long context-honouring source feeds a single worker that cancels after
	// the first item. Run returns context.Canceled and stops well short of the
	// full stream. Determinism: the source's context-aware send unwinds on
	// ctx.Done(), so Run cannot hang regardless of scheduling.
	const items = 1000
	worker := Worker[int](func(context.Context, WorkerID, int) error {
		if processed.Add(1) == 1 {
			cancel()
		}
		return nil
	})

	err := Run(ctx, counterSource(items), worker, Workers(1))
	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, processed.Load(), int64(items))
}

func TestRunPreCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var called atomic.Bool
	worker := Worker[int](func(context.Context, WorkerID, int) error {
		called.Store(true)
		return nil
	})

	err := Run(ctx, counterSource(1), worker, Workers(1))
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, called.Load(), "worker not called under cancelled context")
}

func TestRunFailFast(t *testing.T) {
	var processed atomic.Int64
	worker := Worker[int](func(_ context.Context, _ WorkerID, item int) error {
		processed.Add(1)
		if item == 5 {
			return errWorker
		}
		return nil
	})

	err := Run(context.Background(), counterSource(1000), worker, Workers(1))
	assert.ErrorIs(t, err, errWorker)
	assert.Less(t, processed.Load(), int64(1000), "fail-fast skips remaining items")
}

func TestRunDrainsNonCooperativeSource(t *testing.T) {
	// A non-cooperative source ignores ctx and uses raw sends. After the worker
	// fails fast and every worker exits, the source's next raw send has no
	// reader. Run drains the work channel so that pending send unblocks, the
	// source finishes and closes work, and Run returns instead of hanging on the
	// source's wg.Wait. Without the drain this test deadlocks (and times out).
	const items = 3
	source := Source[int](func(_ context.Context, out chan<- int) error {
		for i := range items {
			out <- i // raw send: deliberately ignores ctx
		}
		return nil
	})
	worker := Worker[int](func(context.Context, WorkerID, int) error { return errWorker })

	err := Run(context.Background(), source, worker, Workers(1))
	assert.ErrorIs(t, err, errWorker)
}

func TestRunSourceError(t *testing.T) {
	srcErr := errors.New("source failed")
	worker := countWorker(&atomic.Int64{})

	tests := []struct {
		name string
		opts []Optional
	}{
		{name: "fail-fast", opts: nil},
		{name: "collect-all", opts: []Optional{CollectAll}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Run(context.Background(), errSource(srcErr), worker, tt.opts...)
			assert.ErrorIs(t, err, srcErr)
		})
	}
}

func TestRunLogging(t *testing.T) {
	tests := []struct {
		name      string
		groupName string
		contains  []string
	}{
		{name: "completion logs", groupName: "", contains: []string{"workgroup starting", "workgroup completed"}},
		{name: "name in logs", groupName: "processor", contains: []string{"processor"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			opts := []Optional{captureLogger(&buf)}
			if tt.groupName != "" {
				opts = append(opts, Name(tt.groupName))
			}
			require.NoError(t, Run(context.Background(), counterSource(0), countWorker(&atomic.Int64{}), opts...))
			for _, want := range tt.contains {
				assert.Contains(t, buf.String(), want)
			}
		})
	}
}

func TestRunErrorLogging(t *testing.T) {
	var buf bytes.Buffer
	worker := Worker[int](func(context.Context, WorkerID, int) error { return errWorker })

	err := Run(context.Background(), counterSource(1), worker, Workers(1), captureLogger(&buf))
	assert.ErrorIs(t, err, errWorker)
	assert.Contains(t, buf.String(), "workgroup completed with errors")
}

func TestRunSourceErrorLogging(t *testing.T) {
	var buf bytes.Buffer
	srcErr := errors.New("source failed")

	err := Run(context.Background(), errSource(srcErr), countWorker(&atomic.Int64{}), captureLogger(&buf))
	assert.ErrorIs(t, err, srcErr)
	assert.Contains(t, buf.String(), "workgroup source error")
}

// TestMustClampsWorkersAndLoggerToUsableDefaults pins the two corrections
// settings.must applies after options run. Both exist because an option may
// leave a field unusable and nothing downstream re-checks it.
//
// A worker count below one would start NO workers, so Run would hang forever on
// a source that never drains — the worst failure shape available, because it
// looks like slow work. A nil logger would panic on the first structured log
// line, turning a diagnostic into a crash. Neither is reachable through the
// public options today, which is exactly why the clamps are asserted directly:
// an incidental integration test that happens to graze them is not a guarantee.
func TestMustClampsWorkersAndLoggerToUsableDefaults(t *testing.T) {
	t.Parallel()

	for _, workers := range []int{0, -1, -100} {
		got := settings{workers: workers}.must()

		assert.Equal(t, runtime.NumCPU(), got.workers,
			"a worker count of %d would start no workers and hang forever", workers)
		assert.NotNil(t, got.logger, "a nil logger would panic on the first log line")
	}

	explicit := slog.New(slog.NewTextHandler(io.Discard, nil))
	got := settings{workers: 3, logger: explicit}.must()

	assert.Equal(t, 3, got.workers, "a usable worker count is left alone")
	assert.Same(t, explicit, got.logger, "and so is an explicitly supplied logger")
}
