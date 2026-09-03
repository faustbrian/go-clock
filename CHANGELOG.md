# Changelog

All notable changes follow [Keep a Changelog](https://keepachangelog.com/) and
the module follows semantic versioning.

## [Unreleased]

### Changed

- Publish the module's ecosystem family, API, lifecycle, ownership,
  compatibility, documentation, and delivery metadata, and enforce the
  cohesion contract locally and in hosted CI.
- Adopt the versioned shared `golib` repository contract for local and hosted
  verification while retaining package-owned API and mutation evidence.

### Documentation

- Replace archived monorepo and hardening terminology with package-owned
  documentation and reproducible verification guidance.

## [1.0.0] - 2026-08-25

### Changed

- Exclude intentional nested modules from root local-proxy archives so local,
  bootstrap, CI, and public module checksums describe the same source
  boundary.

- Track the pinned documentation-tool lockfile so clean CI checkouts install
  the exact validated cspell dependency.

- Reconcile standalone dependency checksums against deterministic current
  module archives so CI, local verification, and release consumers resolve
  identical content.

- Harden standalone documentation validation with deterministic spelling and
  link checks, package-specific documentation gates, and repository-local
  contributor guidance.

### Documentation

- Replace obsolete standalone-repository links and workflow claims with
  monorepo-canonical targets and current release guidance.
- Document the package's initial stable `v1.0.0` scope and security policy.

- Link the package README to package-owned documentation.

### Changed

- Publish the module from its standalone `github.com/faustbrian/go-clock` identity while preserving its documented API and behavior.
- Delegate local mutation checks to the canonical exact-100 repository runner
  instead of accepting package-local survivors and timeouts.

### Added

- Differential system lifecycle, persistence monotonic-loss, synctest timer,
  ticker, callback, and independent wall/monotonic audit coverage.
- Blocking repeated race stress plus cold and contended benchmark baselines.
- Complete drained, canceled, failed, completed, and released state tables.

### Fixed

- Use deterministic execution counts for default fuzz smoke campaigns to avoid
  reporting the fuzz harness deadline as a clock failure.
- Keep future manual time frozen during a running callback until code
  explicitly waits on a nested or concurrent advancement, preventing reset
  deadlines from depending on goroutine scheduling.

### v1.0.0 scope

The following initial scope is included in `v1.0.0`.

#### Added

- Narrow wall-time, elapsed-time, sleep, timer, ticker, and callback contracts.
- Standard-library-backed `System` behavior for Go 1.26.
- Immutable fixed and bounded concurrency-safe manual clocks.
- Deterministic advancement waiters, registration ordering, wall jumps,
  callback synchronization, panic containment, and shutdown.
- Bounded advancement waiters and immediate release of superseded schedules.
- `testing/synctest` composition helpers and bounded lifecycle observations.
- Race, fuzz, leak, mutation, benchmark, security, compatibility, and release
  automation.

[Unreleased]: https://github.com/faustbrian/go-clock/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/faustbrian/go-clock/releases/tag/v1.0.0
