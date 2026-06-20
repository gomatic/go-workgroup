# Data Model: Implement workgroup

**Branch**: `001-implement-workgroup` | **Date**: 2026-02-22

## Public Types

### Source[T any]

Function type that generates work items by sending them to a channel.

- **Signature**: `func(context.Context, chan<- T) error`
- **Lifecycle**: Called once per Run invocation. Runs in its own goroutine.
- **Contract**: Send items to channel. Return nil on success. Return error
  on failure (always fatal — cancels the workgroup).
- **Context**: MUST stop sending when `ctx.Done()` is signalled.

### Worker[T any]

Function type that processes a single work item.

- **Signature**: `func(context.Context, int, T) error`
- **Parameters**: context, worker ID (0-based), work item
- **Lifecycle**: N instances run concurrently. Each processes items from
  shared work channel until channel closes or context cancels.
- **Contract**: Return nil on success. Return error on failure (behavior
  depends on error mode: fail-fast or collect-all).

### Optional

Interface for configuring workgroup behavior.

- **Method**: `Apply(*settings)`
- **Contract**: Mutates the settings struct. Nil-safe — nil options are
  skipped during application.

## Public Option Types

All implement `Optional`.

| Type | Underlying | Exported | Apply Behavior |
|------|-----------|----------|----------------|
| `Workers` | `int` | yes | Sets worker goroutine count |
| `Name` | `string` | yes | Sets workgroup name for log output |
| `Log` | `struct{ *slog.Logger }` | yes | Sets structured logger |
| `onError` | `int` | no | Sets error handling mode |

### onError

Unexported integer type controlling error handling behavior. Only the
exported constants `FailFast` and `CollectAll` are usable by callers.
This prevents construction of arbitrary values.

- **Constants**: `FailFast` (0, default), `CollectAll` (1)
- **Implements**: `Optional` via `Apply(*settings)`

## Internal Types

### settings

Private struct holding resolved configuration.

| Field | Type | Default | Source |
|-------|------|---------|--------|
| `workers` | `int` | `runtime.NumCPU()` | `Workers` option |
| `name` | `string` | `""` | `Name` option |
| `logger` | `*slog.Logger` | `slog.Default()` | `Log` option |
| `onError` | `onError` | `FailFast` | `FailFast` / `CollectAll` constants |

### Validation Rules

- `workers < 1` → clamped to `runtime.NumCPU()`
- `logger == nil` after options → set to `slog.Default()`
- Nil options in slice → skipped silently
- Nil source → return error immediately
- Nil worker → return error immediately

## Relationships

```text
Optional ──Apply──▶ settings
Source[T] ──feeds──▶ chan T ──consumed by──▶ Worker[T]
Pipe[In,Out] ──wraps──▶ Source[In] + transform ──produces──▶ Source[Out]
Run ──coordinates──▶ Source + Workers + settings + context
FanOut ──delegates──▶ Run (with Workers(n))
FanIn ──delegates──▶ Run (with Workers(1))
```
