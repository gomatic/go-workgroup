package workgroup

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// TestFanInProcessesWithExactlyOneWorker names FanIn's claim. "Exactly 1" is
// the whole contract — it is what a caller reaches for when the work is NOT
// safe to run concurrently — so a FanIn that started two workers would
// introduce the very concurrency the caller chose it to avoid, and the symptom
// would be a data race in the caller's code rather than a failure here.
//
// Concurrency is measured directly rather than by counting distinct worker IDs:
// with a fast worker one goroutine can win every receive, so observing a single
// ID would pass even if four workers were running.
func TestFanInProcessesWithExactlyOneWorker(t *testing.T) {
	var live, peak atomic.Int64
	var processed atomic.Int64

	worker := func(_ context.Context, _ WorkerID, _ int) error {
		n := live.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		processed.Add(1)
		live.Add(-1)
		return nil
	}

	require.NoError(t, FanIn(context.Background(), counting(50), Worker[int](worker)))

	assert.Equal(t, int64(1), peak.Load(), "no two items may ever be in flight at once")
	assert.Equal(t, int64(50), processed.Load(), "and every item must still be processed")
}

// TestFanOutRunsTheWorkerCountItWasGivenConcurrently is FanIn's counterpart,
// included so the assertion above cannot be satisfied by an implementation that
// always runs a single worker regardless of what was asked for. Each worker
// holds its item until every worker has arrived, so the peak is the real
// concurrency rather than whatever the scheduler happened to produce.
func TestFanOutRunsTheWorkerCountItWasGivenConcurrently(t *testing.T) {
	const want = 4
	var live, peak atomic.Int64
	arrived := make(chan struct{}, want)
	release := make(chan struct{})

	worker := func(_ context.Context, _ WorkerID, _ int) error {
		n := live.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		select {
		case arrived <- struct{}{}:
			<-release // hold until every worker has been seen
		default:
		}
		live.Add(-1)
		return nil
	}

	go func() {
		for range want {
			<-arrived
		}
		close(release)
	}()

	require.NoError(t, FanOut(context.Background(), Workers(want), counting(200), Worker[int](worker)))

	assert.Equal(t, int64(want), peak.Load(), "FanOut must run the worker count it was given")
}

// counting is a Source emitting 0..n-1.
func counting(n int) Source[int] {
	return func(ctx context.Context, out chan<- int) error {
		for i := range n {
			if err := send(ctx, out, i); err != nil {
				return err
			}
		}
		return nil
	}
}
