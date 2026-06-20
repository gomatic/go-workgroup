<!--
Sync Impact Report
- Version change: N/A → 1.0.0 (initial ratification)
- Added principles: I–VII (all new)
- Added sections: Technical Constraints, Development Workflow, Governance
- Removed sections: none (initial)
- Templates requiring updates:
  - .specify/templates/plan-template.md ✅ no changes needed (Constitution Check section is generic)
  - .specify/templates/spec-template.md ✅ no changes needed (technology-agnostic by design)
  - .specify/templates/tasks-template.md ✅ no changes needed (structure-agnostic by design)
  - .specify/templates/checklist-template.md ✅ no changes needed (generated per-feature)
- Follow-up TODOs: none
-->

# workgroup Constitution

## Core Principles

### I. Type Safety via Generics

The public API MUST use Go generics (`[T any]`) for all work-item types.
`interface{}`/`any` MUST NOT appear in any public function signature,
type definition, or return value where a generic type parameter provides
compile-time safety. Callers MUST NOT need type assertions to use the
library.

### II. Stateless Public API

Public API functions MUST NOT require callers to construct or mutate
struct state. Configuration is passed as function parameters and
options. There MUST be no builder pattern, no multi-step construction,
and no mutable intermediate state. Every public function call MUST be
self-contained and independently invocable.

### III. Context-Driven Lifecycle

All execution entry points MUST accept `context.Context` as the first
parameter. Cancellation, deadlines, and timeouts MUST be handled
exclusively through context — no custom timeout fields, no
`time.After` loops, no ad-hoc shutdown mechanisms. Workers MUST
respect context cancellation and drain cleanly.

### IV. Errors Over Fatals

The library MUST NOT call `log.Fatal`, `log.Panic`, `os.Exit`, or
`panic` for any condition a caller could recover from. All failures
MUST be returned as `error` values. Invalid configuration (nil worker,
nil source, invalid worker count) MUST be reported via returned errors,
not process termination.

### V. Deterministic Testability

All code MUST be testable without timing dependencies (`time.Sleep`),
random inputs (`math/rand`), or external state. Tests MUST use
controlled inputs and synchronization primitives. The test suite MUST
achieve 100% meaningful line and branch coverage — every error path,
every option, every public function. Tests MUST pass deterministically
on every run with `-race` enabled.

### VI. Stdlib Only

The library MUST depend exclusively on the Go standard library. Zero
external module dependencies. This ensures minimal import footprint,
no transitive dependency risk, and maximum compatibility.

### VII. Embedded Options Pattern

Configuration MUST use named types implementing `Apply(*settings)`,
following the fmt.alt `Optional` / `Apply` pattern embedded directly
in the library. The `Optional` interface and all option types MUST be
non-generic. The pattern MUST be self-contained — no imports from or
references to external options libraries.

## Technical Constraints

- **Go version**: 1.26 minimum (set in `go.mod`)
- **Module path**: `github.com/gomatic/workgroup`
- **Logging**: `log/slog` for all structured logging — injectable
  via the `Log` option type
- **Concurrency**: `sync.WaitGroup` for goroutine coordination,
  channels for work distribution, `context.Context` for lifecycle
- **Error aggregation**: `errors.Join` for collecting multiple errors

## Development Workflow

- **TDD mandatory**: tests written before implementation, red-green-refactor
- **Test command**: `go test -race -cover ./...` MUST pass with zero failures
- **Coverage gate**: 100% meaningful line coverage — no untested public paths
- **No timing tests**: tests MUST NOT depend on wall-clock time or `time.Sleep`
- **Deterministic**: tests MUST produce identical results on every run
- **Vet and lint**: `go vet ./...` MUST pass with zero findings

## Governance

This constitution supersedes all other practices for the workgroup
project. Amendments require:

1. Explicit documentation of what changed and why
2. Version bump following semantic versioning:
   - MAJOR: principle removal or backward-incompatible redefinition
   - MINOR: new principle added or existing principle materially expanded
   - PATCH: clarification, wording, or non-semantic refinement
3. Propagation check across all dependent Spec Kit templates

All code changes MUST be verified against these principles before merge.
Violations MUST be justified in the plan's Complexity Tracking table or
rejected.

**Version**: 1.0.0 | **Ratified**: 2015-03-14 | **Last Amended**: 2026-02-22
