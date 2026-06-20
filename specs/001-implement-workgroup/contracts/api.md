# Public API Contract: workgroup

**Package**: `github.com/gomatic/workgroup`

## Types

```go
// Source generates work items by sending them to the channel.
type Source[T any] func(context.Context, chan<- T) error

// Worker processes a single work item. id is the 0-based worker index.
type Worker[T any] func(context.Context, int, T) error

// Optional configures workgroup behavior.
type Optional interface {
    Apply(*settings)
}

// onError controls error handling mode (unexported type — only
// FailFast and CollectAll are usable by callers).
type onError int

const (
    FailFast   onError = iota // cancel all on first error (default)
    CollectAll                 // continue, join all errors at end
)

// Workers sets the number of concurrent worker goroutines.
type Workers int

// Name identifies the workgroup in log output.
type Name string

// Log sets the structured logger.
type Log struct{ *slog.Logger }
```

## Functions

```go
// Run distributes work from source across N workers (default: NumCPU).
// Blocks until all work is processed or context is cancelled.
// Returns nil on success, or the first/joined error(s) on failure.
func Run[T any](ctx context.Context, source Source[T], worker Worker[T], opts ...Optional) error

// FanOut distributes work across n workers. Equivalent to Run with Workers(n).
func FanOut[T any](ctx context.Context, n int, source Source[T], worker Worker[T], opts ...Optional) error

// FanIn processes work with exactly 1 worker. Equivalent to Run with Workers(1).
func FanIn[T any](ctx context.Context, source Source[T], worker Worker[T], opts ...Optional) error

// Pipe creates a Source from a transformation, enabling stage chaining.
// The returned Source, when consumed by a downstream Run, executes the
// upstream source with n workers applying transform to each item.
func Pipe[In, Out any](ctx context.Context, n int, source Source[In], transform func(context.Context, int, In) (Out, error), opts ...Optional) Source[Out]
```

## Error Behavior

| Condition | FailFast (default) | CollectAll |
|-----------|-------------------|------------|
| Worker returns error | Cancel context, return first error | Continue, join all errors |
| Source returns error | Cancel context, return source error | Cancel context, return source error |
| Context cancelled | Return context error | Return context error |
| Nil source | Return error immediately | Return error immediately |
| Nil worker | Return error immediately | Return error immediately |

## Option Defaults

| Option | Default |
|--------|---------|
| Workers | `runtime.NumCPU()` |
| Name | `""` (empty) |
| Log | `slog.Default()` |
| onError | `FailFast` |
