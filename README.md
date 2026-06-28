# workgroup

[![CI](https://github.com/gomatic/go-workgroup/actions/workflows/ci.yml/badge.svg)](https://github.com/gomatic/go-workgroup/actions/workflows/ci.yml)

Package `workgroup` distributes work across concurrent goroutines with type-safe
generics, structured logging, and configurable error handling.

## Install

```bash
go get github.com/gomatic/go-workgroup
```

Requires Go 1.26+.

## Usage

### Fan-Out: Distribute Work Across Workers

```go
source := workgroup.Source[int](func(ctx context.Context, out chan<- int) error {
    for i := range 100 {
        select {
        case out <- i:
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    return nil
})

worker := workgroup.Worker[int](func(ctx context.Context, id workgroup.WorkerID, item int) error {
    fmt.Printf("worker %d processed item %d\n", id, item)
    return nil
})

err := workgroup.FanOut(ctx, 8, source, worker)
```

### Fan-In: Single Worker

```go
err := workgroup.FanIn(ctx, source, worker)
```

### Pipeline: Fan-Out Transform into Fan-In

```go
doubled := workgroup.Pipe(4, source,
    func(ctx context.Context, id workgroup.WorkerID, item int) (int, error) {
        return item * 2, nil
    },
)

err := workgroup.FanIn(ctx, doubled, aggregator)
```

### Options

```go
err := workgroup.Run(ctx, source, worker,
    workgroup.Workers(16),
    workgroup.Name("processor"),
    workgroup.Log{Logger: slog.Default()},
    workgroup.CollectAll,
)
```

### Error Handling

```go
// Fail-fast (default): first error cancels all workers
err := workgroup.Run(ctx, source, riskyWorker)

// Collect-all: continue processing, aggregate errors
err := workgroup.Run(ctx, source, riskyWorker, workgroup.CollectAll)
// err contains all worker errors via errors.Join
```

## API

| Function | Description |
|----------|-------------|
| `Run[T]` | Distribute work from source across N workers (default: NumCPU) |
| `FanOut[T]` | Run with Workers(n) |
| `FanIn[T]` | Run with Workers(1) |
| `Pipe[In, Out]` | Create a Source from a transformation for stage chaining |

| Option | Default | Description |
|--------|---------|-------------|
| `Workers` | `runtime.NumCPU()` | Number of concurrent worker goroutines |
| `Name` | `""` | Workgroup name for log output |
| `Log` | `slog.Default()` | Structured logger |
| `FailFast` | yes | Cancel all on first error |
| `CollectAll` | no | Continue, join all errors at end |

## License

See [LICENSE](LICENSE).
