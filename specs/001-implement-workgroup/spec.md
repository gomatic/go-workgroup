# Feature Specification: Implement workgroup

**Feature Branch**: `001-implement-workgroup`
**Created**: 2026-02-22
**Status**: Draft
**Input**: Implement a Go library for concurrent work distribution using generics,
context.Context, slog, embedded fmt.alt options pattern, and error returns.
Achieve 100% meaningful test coverage with deterministic tests.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Distribute Work Across Concurrent Workers (Priority: P1)

A caller distributes a set of work items across N concurrent worker goroutines.
Each worker processes items from a shared channel. The caller blocks until all
items are processed, then receives nil or an error.

**Why this priority**: This is the core value proposition of the library — replacing
sync.WaitGroup boilerplate with a single function call.

**Independent Test**: Verified by sending N items through a source function,
confirming every item reaches a worker exactly once, and confirming Run returns nil.

**Acceptance Scenarios**:

1. **Given** a source producing 100 items and 8 workers, **When** Run is called,
   **Then** all 100 items are processed and Run returns nil.
2. **Given** a source producing 0 items, **When** Run is called,
   **Then** Run returns nil immediately with no worker invocations.
3. **Given** no explicit worker count, **When** Run is called,
   **Then** the number of workers defaults to runtime.NumCPU().

---

### User Story 2 - Cancel Work via Context (Priority: P1)

A caller cancels in-flight work by cancelling the context passed to Run.
Workers stop processing and Run returns promptly.

**Why this priority**: Context cancellation is fundamental to Go concurrency and
prevents goroutine leaks.

**Independent Test**: Verified by cancelling context mid-processing and confirming
Run returns within a bounded time, and that no workers are invoked after cancellation.

**Acceptance Scenarios**:

1. **Given** a long-running source and a context with cancel, **When** the context
   is cancelled, **Then** Run returns the context error and workers stop.
2. **Given** a context with a deadline, **When** the deadline expires during
   processing, **Then** Run returns the context deadline error.

---

### User Story 3 - Handle Worker Errors (Priority: P1)

A caller receives errors from worker functions. The library supports two modes:
fail-fast (cancel on first error) and collect-all (continue, aggregate errors).

**Why this priority**: Error propagation is essential for production use — callers
must know when workers fail.

**Independent Test**: Verified by returning errors from worker functions and
confirming Run returns the expected error(s) under both modes.

**Acceptance Scenarios**:

1. **Given** fail-fast mode (default) and a worker that errors on item 5,
   **When** Run executes, **Then** Run returns the worker error and remaining
   items are skipped.
2. **Given** collect-all mode and 3 workers that each error once, **When** Run
   executes, **Then** Run returns an error containing all 3 worker errors.
3. **Given** a source function that returns an error, **When** Run executes,
   **Then** Run returns the source error.

---

### User Story 4 - Configure via Options (Priority: P2)

A caller customizes workgroup behavior using named option types following the
embedded fmt.alt pattern. Options include worker count, name, logger, and
error mode.

**Why this priority**: Options make the library flexible without complicating
the core API.

**Independent Test**: Verified by passing each option type and confirming the
corresponding behavior change.

**Acceptance Scenarios**:

1. **Given** Workers(4) option, **When** Run executes, **Then** exactly 4
   worker goroutines are started.
2. **Given** Name("processor") option, **When** Run executes, **Then** log
   output includes "processor".
3. **Given** a custom slog.Logger via Log option, **When** Run executes,
   **Then** all log output goes to the custom logger.
4. **Given** CollectAll error mode option, **When** a worker errors,
   **Then** remaining workers continue processing.

---

### User Story 5 - Chain Stages via Pipe (Priority: P3)

A caller chains a fan-out stage into a fan-in stage using Pipe. The output
of one stage becomes the source for the next.

**Why this priority**: Chaining enables pipeline patterns (fan-out then aggregate)
which was a key capability of the original library.

**Independent Test**: Verified by piping a fan-out transform into a fan-in
consumer and confirming all transformed items arrive at the consumer.

**Acceptance Scenarios**:

1. **Given** a source producing integers and a transform doubling each value,
   **When** Pipe creates a new source consumed by FanIn, **Then** the fan-in
   worker receives all doubled values.
2. **Given** a transform that errors on specific items, **When** Pipe executes,
   **Then** the error propagates to the downstream Run call.

---

### User Story 6 - Convenience Constructors (Priority: P3)

A caller uses FanOut and FanIn as semantic shortcuts for common patterns.
FanOut distributes across multiple workers. FanIn processes with exactly one
worker.

**Why this priority**: Convenience functions improve readability and express
intent clearly.

**Independent Test**: Verified by confirming FanOut starts N workers and
FanIn starts exactly 1 worker.

**Acceptance Scenarios**:

1. **Given** FanOut called with n=8, **When** execution completes, **Then**
   8 distinct worker IDs are observed.
2. **Given** FanIn called, **When** execution completes, **Then** only worker
   ID 0 is observed.

---

### Edge Cases

- What happens when a nil source function is passed? Run returns an error.
- What happens when a nil worker function is passed? Run returns an error.
- What happens when Workers(0) is passed? Defaults to runtime.NumCPU().
- What happens when Workers(-1) is passed? Defaults to runtime.NumCPU().
- What happens when a nil option is in the options slice? It is skipped safely.
- What happens when context is already cancelled before Run is called? Run
  returns the context error immediately.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Library MUST provide a `Run` function that distributes work items
  from a source across N concurrent worker goroutines and blocks until complete.
- **FR-002**: Library MUST use Go generics so callers specify their work item
  type at the call site without type assertions.
- **FR-003**: Library MUST accept `context.Context` as the first parameter of
  all execution functions and respect cancellation and deadlines.
- **FR-004**: Library MUST return errors from worker and source functions to the
  caller — never call `log.Fatal`, `panic`, or `os.Exit`.
- **FR-005**: Library MUST support configurable error handling: fail-fast (default)
  and collect-all modes via an option type.
- **FR-006**: Library MUST provide an `Optional` interface and named option types
  (`Workers`, `Name`, `Log`, `OnError`) following the fmt.alt `Apply(*settings)`
  pattern, embedded without external imports.
- **FR-007**: Library MUST provide `FanOut` and `FanIn` convenience functions with
  clear semantic meaning.
- **FR-008**: Library MUST provide a `Pipe` function enabling stage chaining
  (fan-out output as fan-in input).
- **FR-009**: Library MUST default to `runtime.NumCPU()` workers when no worker
  count is specified or an invalid count is provided.
- **FR-010**: Library MUST use `log/slog` for all internal logging, injectable
  via the `Log` option type.
- **FR-011**: Library MUST depend only on the Go standard library with zero
  external module dependencies.
- **FR-012**: Library MUST achieve 100% meaningful line and branch coverage with
  deterministic tests that pass under `-race`.

### Key Entities

- **Source**: A function that generates work items by sending them to a channel.
  Signature: `func(context.Context, chan<- T) error`.
- **Worker**: A function that processes a single work item. Signature:
  `func(context.Context, int, T) error` where int is the worker ID.
- **Optional**: An interface with method `Apply(*settings)` for configuring
  workgroup behavior.
- **settings**: Internal struct holding resolved configuration (worker count,
  name, logger, error mode).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All work items produced by a source are processed exactly once —
  zero items lost, zero items duplicated.
- **SC-002**: Caller receives all worker errors — no errors silently discarded.
- **SC-003**: Context cancellation halts processing within one work-item cycle —
  no goroutine leaks after Run returns.
- **SC-004**: Test suite achieves 100% line coverage with zero test failures
  under race detection.
- **SC-005**: Library compiles with zero external dependencies beyond the Go
  standard library.
- **SC-006**: Every public function and option type is exercised by at least
  one test.
