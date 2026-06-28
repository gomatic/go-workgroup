# Quickstart: workgroup

**Package**: `github.com/gomatic/go-workgroup`

## Install

```bash
go get github.com/gomatic/go-workgroup
```

## Basic Usage — Fan-Out

Distribute 100 work items across 8 workers:

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

worker := workgroup.Worker[int](func(ctx context.Context, id int, item int) error {
    fmt.Printf("worker %d processed item %d\n", id, item)
    return nil
})

err := workgroup.FanOut(ctx, 8, source, worker)
```

## Fan-In (Single Worker)

```go
err := workgroup.FanIn(ctx, source, worker)
```

## Pipeline — Fan-Out then Fan-In

```go
doubled := workgroup.Pipe(ctx, 8, source,
    func(ctx context.Context, id int, item int) (int, error) {
        return item * 2, nil
    },
)

err := workgroup.FanIn(ctx, doubled, aggregator)
```

## Options

```go
err := workgroup.Run(ctx, source, worker,
    workgroup.Workers(16),
    workgroup.Name("processor"),
    workgroup.Log{Logger: slog.Default()},
    workgroup.CollectAll,
)
```

## Error Handling

```go
// Fail-fast (default): first error cancels all workers
err := workgroup.Run(ctx, source, riskyWorker)

// Collect-all: continue processing, aggregate errors
err := workgroup.Run(ctx, source, riskyWorker, workgroup.CollectAll)
// err contains all worker errors via errors.Join
```

## Validation Scenarios

1. `workgroup.FanOut(ctx, 8, source, worker)` — processes all items, returns nil
2. Cancel `ctx` mid-processing — returns context error, workers stop
3. Worker returns error with `FailFast` — remaining items skipped
4. Worker returns error with `CollectAll` — all items attempted, errors joined
5. `Pipe` into `FanIn` — all transformed items reach downstream worker
