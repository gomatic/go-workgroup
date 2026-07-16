# Implementation Plan: Implement workgroup

**Branch**: `001-implement-workgroup` | **Date**: 2026-02-22 | **Spec**: [spec.md](spec.md) **Input**: Feature specification from `specs/001-implement-workgroup/spec.md`

## Summary

Implement a Go 1.26 library for concurrent work distribution using generics, context.Context, slog, and an embedded fmt.alt options pattern. The library replaces sync.WaitGroup boilerplate with type-safe top-level functions (Run, FanOut, FanIn, Pipe). All errors are returned, not fatal. Tests are deterministic with 100% meaningful coverage.

## Technical Context

**Language/Version**: Go 1.26 **Primary Dependencies**: Go standard library only (sync, context, log/slog, errors, runtime) **Storage**: N/A **Testing**: `go test -race -cover ./...` **Target Platform**: All platforms supported by Go **Project Type**: Library **Performance Goals**: Minimal overhead over raw goroutines + sync.WaitGroup **Constraints**: Zero external dependencies, stdlib only **Scale/Scope**: ~300 LOC, 4 source files, 1 test file

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

| Principle | Status | Evidence |
| --- | --- | --- |
| I. Type Safety via Generics | PASS | `Run[T]`, `Source[T]`, `Worker[T]` — no `interface{}`/`any` in public API |
| II. Stateless Public API | PASS | Top-level functions only, no builder, no mutable struct |
| III. Context-Driven Lifecycle | PASS | `context.Context` first parameter on Run, FanOut, FanIn, Pipe |
| IV. Errors Over Fatals | PASS | All failures returned as `error`, no log.Fatal/panic/os.Exit |
| V. Deterministic Testability | PASS | No time.Sleep, no rand, controlled inputs, -race enabled |
| VI. Stdlib Only | PASS | Zero external module dependencies |
| VII. Embedded Options Pattern | PASS | `Optional` interface, named types with `apply(settings) settings`, self-contained |

**Result**: All gates pass. No violations.

## Design Decisions

### Work Channel Buffering

**Decision**: Unbuffered channel for work distribution.

Unbuffered channels provide natural backpressure — the source blocks until a worker is ready. This prevents unbounded memory growth when the source produces faster than workers consume.

_Rejected_: Buffered channel (size = workers) — slightly higher throughput but buffered items are lost on cancellation and backpressure is less predictable.

### Error Collection Strategy

**Decision**: Mutex-protected slice, `errors.Join` for aggregation.

Workers run concurrently and may return errors simultaneously. A `sync.Mutex` guarding a `[]error` slice is the simplest correct approach. After all workers complete, `errors.Join` produces a single combined error. For fail-fast mode, the first error calls `context.CancelCauseFunc` and no further errors are collected.

_Rejected_: Error channel (requires separate drain goroutine, unnecessary complexity). `sync.Once` for first error (only captures one, insufficient for collect-all mode).

### Context Cancellation Propagation

**Decision**: `context.WithCancelCause` inside Run. Source and workers check `ctx.Done()` via select.

`context.WithCancelCause` (Go 1.20+) attaches the first worker error as the cancellation cause. Flow:

1. Run creates child context with cancel.
2. Source goroutine sends to work channel via `select` with `ctx.Done()`.
3. Workers receive from work channel via `select` with `ctx.Done()`.
4. On cancellation: source stops, workers drain and exit, channels close, WaitGroup unblocks, Run returns.

_Rejected_: `context.WithCancel` without cause (loses originating error). Manual done channel (duplicates context semantics).

### Pipe Execution Model

**Decision**: Pipe returns a `Source[Out]` closure. Lazy execution — the upstream doesn't start until the downstream calls the returned Source.

When the downstream Run invokes the returned Source:

1. Creates internal work channel for upstream items.
2. Starts N workers that call the transform and send results to the downstream channel.
3. Starts the upstream source goroutine feeding the internal channel.
4. Blocks until upstream is fully processed.
5. Returns any errors from transform or upstream source.

The downstream Run's context governs the entire pipeline.

_Rejected_: Eager execution (goroutine leaks if Source never used). Intermediate buffer channel (memory overhead, complicates backpressure).

### Source Error Handling

**Decision**: Source errors are always fatal regardless of error mode. Collect-all applies only to worker errors.

The source is the single producer. If it fails, no more work items can be generated — there is nothing to "continue" with. Worker errors are independent (each processes different items), so continuing makes sense. Source failure means the input stream is broken.

_Rejected_: Collecting source error alongside worker errors (misleading — remaining items are never produced).

### Worker Lifecycle on Cancellation

**Decision**: Workers finish their current item but do not pick up new items after context cancellation.

The worker function receives the context and can check it internally if it needs to abort mid-item. The library stops _dispatching_ new items, not forcefully interrupting in-progress work.

_Rejected_: Force-kill workers mid-item (Go has no goroutine preemption; would require wrapping worker calls for no practical benefit).

## Project Structure

### Documentation (this feature)

```text
specs/001-implement-workgroup/
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── api.md
└── checklists/
    └── requirements.md
```

### Source Code (repository root)

```text
go.mod                 # module github.com/gomatic/go-workgroup, go 1.26
options.go             # Optional interface, named option types (Workers, Name, Log, OnError)
settings.go            # settings struct, must(), newSettings(), defaults
workgroup.go           # Source[T], Worker[T], Run, FanOut, FanIn, Pipe (public API)
workgroup_test.go      # all tests — deterministic, 100% coverage
```

**Structure Decision**: Flat package at repository root. Go library convention — no `src/` or `lib/` subdirectories. Four source files organized by concern: public options, private settings, and public API. Single test file covers all paths.

## Complexity Tracking

> No constitution violations. Table intentionally empty.

| Violation | Why Needed | Simpler Alternative Rejected Because |
| --------- | ---------- | ------------------------------------ |
| (none)    | —          | —                                    |
