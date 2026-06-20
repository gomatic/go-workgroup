# Tasks: Implement workgroup

**Input**: Design documents from `specs/001-implement-workgroup/`
**Prerequisites**: plan.md, spec.md, data-model.md, contracts/api.md, quickstart.md

**Tests**: TDD mandatory per constitution. Tests written first, verified failing, then implementation.

**Organization**: Tasks grouped by user story. US1-US3 are P1 (core Run function), US4 is P2 (options verification), US5-US6 are P3 (Pipe and convenience constructors).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1-US6 mapping from spec.md
- Exact file paths at repository root (flat Go package)

---

## Phase 1: Setup

**Purpose**: Initialize module and remove legacy code

- [x] T001 Create go.mod with module github.com/gomatic/workgroup and go 1.26 in go.mod
- [x] T002 Remove legacy files: .travis.yml, ex/, TODO.md, wg.go, wg_test.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Options and settings infrastructure required by all user stories

- [x] T003 [P] Implement Optional interface and option types (Workers, Name, Log, onError, FailFast, CollectAll) in options.go
- [x] T004 [P] Implement settings struct, must(), newSettings() with defaults (workers=NumCPU, logger=slog.Default, onError=FailFast) in settings.go
- [x] T005 Write tests for option application, settings defaults, nil option skip, and invalid worker clamping in workgroup_test.go

**Checkpoint**: Foundation ready — user story implementation can begin

---

## Phase 3: User Story 1 — Distribute Work Across Concurrent Workers (Priority: P1)

**Goal**: Run distributes work items from a source across N concurrent workers and blocks until complete

**Independent Test**: Send 100 items through source, confirm all 100 processed exactly once, Run returns nil

### Tests for User Story 1

> Write tests FIRST, ensure they FAIL before implementation

- [x] T006 [US1] Write test: Run with 8 workers processes all 100 items exactly once in workgroup_test.go
- [x] T007 [US1] Write test: Run with empty source (0 items) returns nil immediately in workgroup_test.go
- [x] T008 [US1] Write test: Run with no Workers option defaults to NumCPU workers in workgroup_test.go
- [x] T009 [P] [US1] Write test: Run with nil source returns error in workgroup_test.go
- [x] T010 [P] [US1] Write test: Run with nil worker returns error in workgroup_test.go

### Implementation for User Story 1

- [x] T011 [US1] Define Source[T] and Worker[T] generic function types in workgroup.go
- [x] T012 [US1] Implement Run[T] in workgroup.go: context.WithCancelCause, unbuffered work channel, sync.WaitGroup, source goroutine with ctx select, N worker goroutines with ctx select, channel close on source completion, WaitGroup.Wait, fail-fast error return

**Checkpoint**: Run works for happy path — all items distributed, nil source/worker rejected, defaults applied

---

## Phase 4: User Story 2 — Cancel Work via Context (Priority: P1)

**Goal**: Context cancellation stops workers and Run returns the context error

**Independent Test**: Cancel context mid-processing, confirm Run returns context error and no new work dispatched

### Tests for User Story 2

- [x] T013 [US2] Write test: cancelling context mid-processing stops Run and returns context.Canceled in workgroup_test.go
- [x] T014 [US2] Write test: pre-cancelled context returns error immediately with zero worker invocations in workgroup_test.go

### Implementation for User Story 2

No new code — context cancellation is inherent in Run from T012 (select on ctx.Done in source and worker loops).

**Checkpoint**: Context cancellation verified — no goroutine leaks after Run returns

---

## Phase 5: User Story 3 — Handle Worker Errors (Priority: P1)

**Goal**: Worker errors propagate to caller in fail-fast (default) and collect-all modes

**Independent Test**: Return errors from workers, verify Run returns expected error(s) under both modes

### Tests for User Story 3

- [x] T015 [US3] Write test: fail-fast mode returns first worker error and skips remaining items in workgroup_test.go
- [x] T016 [US3] Write test: collect-all mode returns all worker errors via errors.Join in workgroup_test.go
- [x] T017 [US3] Write test: source error propagates regardless of error mode in workgroup_test.go

### Implementation for User Story 3

- [x] T018 [US3] Add collect-all error handling to Run in workgroup.go: mutex-protected []error slice, conditional cancel on FailFast only, errors.Join aggregation after WaitGroup.Wait

**Checkpoint**: Both error modes verified — fail-fast cancels early, collect-all aggregates all

---

## Phase 6: User Story 4 — Configure via Options (Priority: P2)

**Goal**: Each named option type affects Run behavior as specified

**Independent Test**: Pass each option and verify the corresponding behavior change

### Tests for User Story 4

- [x] T019 [P] [US4] Write test: Workers(4) starts exactly 4 worker goroutines in workgroup_test.go
- [x] T020 [P] [US4] Write test: Name("processor") appears in slog output captured via custom handler in workgroup_test.go
- [x] T021 [P] [US4] Write test: Log option directs all output to custom slog.Logger in workgroup_test.go
- [x] T022 [P] [US4] Write test: nil option in slice is skipped without panic in workgroup_test.go

### Implementation for User Story 4

No new code — options implemented in Phase 2, consumed by Run from Phase 3. This phase validates integration.

**Checkpoint**: All option types verified — Workers count, Name in logs, custom Logger, nil safety

---

## Phase 7: User Story 5 — Chain Stages via Pipe (Priority: P3)

**Goal**: Pipe creates a Source from a transformation, enabling fan-out to fan-in chaining

**Independent Test**: Pipe integers through a doubling transform, consume via FanIn, verify all doubled values arrive

### Tests for User Story 5

- [x] T023 [US5] Write test: Pipe transforms all source items and delivers to downstream FanIn in workgroup_test.go
- [x] T024 [US5] Write test: Pipe transform error propagates to downstream Run in workgroup_test.go

### Implementation for User Story 5

- [x] T025 [US5] Implement Pipe[In, Out] in workgroup.go: returns Source[Out] closure that internally runs upstream source with N transform workers, sends results to downstream channel, respects context

**Checkpoint**: Pipeline chaining works — fan-out transform feeds fan-in consumer

---

## Phase 8: User Story 6 — Convenience Constructors (Priority: P3)

**Goal**: FanOut and FanIn are semantic shortcuts for common Run patterns

**Independent Test**: FanOut with n=8 observes 8 distinct worker IDs, FanIn observes only worker 0

### Tests for User Story 6

- [x] T026 [P] [US6] Write test: FanOut with n=8 observes 8 distinct worker IDs in workgroup_test.go
- [x] T027 [P] [US6] Write test: FanIn observes only worker ID 0 in workgroup_test.go

### Implementation for User Story 6

- [x] T028 [US6] Implement FanOut[T] and FanIn[T] in workgroup.go: FanOut prepends Workers(n), FanIn prepends Workers(1), both delegate to Run

**Checkpoint**: Convenience functions work — clear semantic shortcuts

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Coverage verification, documentation, cleanup

- [x] T029 [P] Write Example functions for godoc (ExampleRun, ExampleFanOut, ExamplePipe) in workgroup_test.go
- [x] T030 Run `go test -race -cover ./...` and verify 100% meaningful line coverage
- [x] T031 Run `go vet ./...` and fix any findings
- [x] T032 Update README.md with current API, install instructions, and usage examples

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Phase 2 — BLOCKS US2, US3, US4, US5, US6
- **US2 (Phase 4)**: Depends on Phase 3 (Run must exist for cancellation tests)
- **US3 (Phase 5)**: Depends on Phase 3 (Run must exist for error tests)
- **US4 (Phase 6)**: Depends on Phase 3 (Run must exist for option behavior tests)
- **US5 (Phase 7)**: Depends on Phase 3 + Phase 8 (Pipe uses Run internally, FanIn needed for test)
- **US6 (Phase 8)**: Depends on Phase 3 (FanOut/FanIn wrap Run)
- **Polish (Phase 9)**: Depends on all user story phases

### Parallel Opportunities

- T003 and T004 can run in parallel (different files)
- T009 and T010 can run in parallel (independent test cases)
- US2 (Phase 4), US3 (Phase 5), US4 (Phase 6), US6 (Phase 8) can start in parallel after US1 completes
- T019-T022 can all run in parallel (independent option tests)
- T026 and T027 can run in parallel (independent convenience function tests)
- T029, T031 can run in parallel with T030

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: `go test -race ./...` passes for Run happy path

### Incremental Delivery

1. Setup + Foundational → options and settings work
2. US1 → Run distributes work (MVP)
3. US2 → context cancellation verified
4. US3 → error handling complete (fail-fast + collect-all)
5. US4 → all options verified
6. US6 → FanOut/FanIn available
7. US5 → Pipe chaining available
8. Polish → 100% coverage, docs, vet

---

## Notes

- [P] tasks = different files or independent test cases, no dependencies
- [Story] label maps task to specific user story for traceability
- Tests MUST be written and FAIL before implementation (TDD per constitution)
- All tests MUST be deterministic — no time.Sleep, no rand, no timing dependencies
- Commit after each phase checkpoint
