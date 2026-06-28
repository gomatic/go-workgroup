package workgroup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errWorker = errors.New("worker error")

// counterSource returns a Source that emits the integers [0, n), stopping
// early if the context is cancelled.
func counterSource(n int) Source[int] {
	return func(ctx context.Context, out chan<- int) error {
		for i := range n {
			if err := send(ctx, out, i); err != nil {
				return err
			}
		}
		return nil
	}
}

// errSource returns a Source that immediately fails with err.
func errSource(err error) Source[int] {
	return func(context.Context, chan<- int) error { return err }
}

// countWorker returns a Worker that increments counter for every item.
func countWorker(counter *atomic.Int64) Worker[int] {
	return func(context.Context, WorkerID, int) error {
		counter.Add(1)
		return nil
	}
}

// captureLogger returns a logger writing text to buf for assertion.
func captureLogger(buf *bytes.Buffer) Log {
	return Log{slog.New(slog.NewTextHandler(buf, nil))}
}

func TestRunValidation(t *testing.T) {
	worker := countWorker(&atomic.Int64{})
	source := counterSource(0)

	tests := []struct {
		wantIs error
		source Source[int]
		worker Worker[int]
		name   string
	}{
		{name: "nil source", source: nil, worker: worker, wantIs: ErrNilSource},
		{name: "nil worker", source: source, worker: nil, wantIs: ErrNilWorker},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Run(context.Background(), tt.source, tt.worker)
			assert.ErrorIs(t, err, tt.wantIs)
		})
	}
}

func TestRunProcessesAllItems(t *testing.T) {
	tests := []struct {
		name    string
		items   int
		workers int
	}{
		{name: "many items many workers", items: 100, workers: 8},
		{name: "empty source", items: 0, workers: 4},
		{name: "single worker", items: 20, workers: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var count atomic.Int64
			err := Run(context.Background(), counterSource(tt.items), countWorker(&count), Workers(tt.workers))
			require.NoError(t, err)
			assert.Equal(t, int64(tt.items), count.Load())
		})
	}
}

// barrierWorker returns a Worker that records its id, then blocks until all n
// workers have received an item. Paired with a source of exactly n items, it
// forces every worker to consume exactly one — no worker can loop back for a
// second item until the barrier releases — so the observed IDs deterministically
// span [0, n). Work distribution across goroutines is otherwise non-deterministic:
// a subset of workers can drain the channel, leaving others (including id 0) idle.
func barrierWorker(n int, seen *sync.Map) Worker[int] {
	var arrived sync.WaitGroup
	arrived.Add(n)
	return func(_ context.Context, id WorkerID, _ int) error {
		seen.Store(id, true)
		arrived.Done()
		arrived.Wait()
		return nil
	}
}

func TestRunDefaultWorkers(t *testing.T) {
	// With no Workers option, Run must spawn exactly runtime.NumCPU() workers.
	// The barrier deadlocks (and the test times out) if it spawns fewer.
	n := runtime.NumCPU()
	var seen sync.Map
	require.NoError(t, Run(context.Background(), counterSource(n), barrierWorker(n, &seen)))
	assert.Equal(t, n, countKeys(&seen))
}

func TestRunDistinctWorkerIDs(t *testing.T) {
	tests := []struct {
		run  func(Source[int], Worker[int]) error
		name string
		want int
	}{
		{name: "Run with Workers(4)", run: runWith(Workers(4)), want: 4},
		{name: "FanIn single worker", run: fanIn, want: 1},
		{name: "FanOut with 8", run: fanOut(8), want: 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen sync.Map
			require.NoError(t, tt.run(counterSource(tt.want), barrierWorker(tt.want, &seen)))
			assert.Equal(t, tt.want, countKeys(&seen))
			// Every worker id in [0, want) is observed — distinct and 0-based.
			for id := range tt.want {
				_, ok := seen.Load(WorkerID(id))
				assert.Truef(t, ok, "worker ID %d observed", id)
			}
		})
	}
}

func runWith(opt Optional) func(Source[int], Worker[int]) error {
	return func(s Source[int], w Worker[int]) error {
		return Run(context.Background(), s, w, opt)
	}
}

func fanIn(s Source[int], w Worker[int]) error {
	return FanIn(context.Background(), s, w)
}

func fanOut(n Workers) func(Source[int], Worker[int]) error {
	return func(s Source[int], w Worker[int]) error {
		return FanOut(context.Background(), n, s, w)
	}
}

func countKeys(m *sync.Map) int {
	count := 0
	m.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

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

func TestRunCollectAll(t *testing.T) {
	const items = 5
	worker := Worker[int](func(context.Context, WorkerID, int) error { return errWorker })

	err := Run(context.Background(), counterSource(items), worker, Workers(1), CollectAll)
	require.Error(t, err)

	joined, ok := err.(interface{ Unwrap() []error })
	require.True(t, ok, "collect-all joins errors")
	assert.Len(t, joined.Unwrap(), items)
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

func TestRunNilOptionInSlice(t *testing.T) {
	var count atomic.Int64
	err := Run(context.Background(), counterSource(3), countWorker(&count), nil, Workers(2), nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count.Load())
}

func TestPipeTransform(t *testing.T) {
	const n = 50
	var sum atomic.Int64

	ctx := context.Background()
	doubled := Pipe(4, counterSource(n), func(_ context.Context, _ WorkerID, item int) (int, error) {
		return item * 2, nil
	})
	worker := Worker[int](func(_ context.Context, _ WorkerID, item int) error {
		sum.Add(int64(item))
		return nil
	})

	require.NoError(t, FanIn(ctx, doubled, worker))
	assert.Equal(t, int64(n*(n-1)), sum.Load())
}

func TestPipeTransformError(t *testing.T) {
	transformErr := errors.New("transform failed")
	ctx := context.Background()
	piped := Pipe(1, counterSource(10), func(_ context.Context, _ WorkerID, item int) (int, error) {
		if item == 3 {
			return 0, transformErr
		}
		return item, nil
	})

	err := FanIn(ctx, piped, countWorker(&atomic.Int64{}))
	assert.ErrorIs(t, err, transformErr)
}

func TestPipeContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	doubled := Pipe(1, counterSource(1000), func(_ context.Context, _ WorkerID, item int) (int, error) {
		return item * 2, nil
	})

	err := FanIn(ctx, doubled, countWorker(&atomic.Int64{}))
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSend(t *testing.T) {
	t.Run("delivers item", func(t *testing.T) {
		out := make(chan int, 1)
		assert.NoError(t, send(context.Background(), out, 42))
		assert.Equal(t, 42, <-out)
	})

	t.Run("honours cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		out := make(chan int) // unbuffered: send blocks, ctx.Done wins
		assert.ErrorIs(t, send(ctx, out, 1), context.Canceled)
	})
}

func TestContextError(t *testing.T) {
	tests := []struct {
		err  error
		name string
		want bool
	}{
		{name: "canceled", err: context.Canceled, want: true},
		{name: "deadline", err: context.DeadlineExceeded, want: true},
		{name: "other", err: errWorker, want: false},
		{name: "nil", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, contextError(tt.err))
		})
	}
}

func ExampleRun() {
	source := counterSource(10)
	worker := Worker[int](func(context.Context, WorkerID, int) error { return nil })

	err := Run(context.Background(), source, worker, Workers(4))
	fmt.Println(err)
	// Output: <nil>
}

func ExampleFanOut() {
	source := Source[string](func(ctx context.Context, out chan<- string) error {
		for _, s := range []string{"a", "b", "c"} {
			if err := send(ctx, out, s); err != nil {
				return err
			}
		}
		return nil
	})
	worker := Worker[string](func(context.Context, WorkerID, string) error { return nil })

	err := FanOut(context.Background(), 2, source, worker)
	fmt.Println(err)
	// Output: <nil>
}

func ExamplePipe() {
	ctx := context.Background()
	doubled := Pipe(2, counterSource(6), func(_ context.Context, _ WorkerID, item int) (int, error) {
		return item * 2, nil
	})

	var sum atomic.Int64
	err := FanIn(ctx, doubled, func(_ context.Context, _ WorkerID, item int) error {
		sum.Add(int64(item))
		return nil
	})
	fmt.Println(err, sum.Load())
	// Output: <nil> 30
}
