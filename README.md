# clock

[![CI](https://github.com/faustbrian/go-clock/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/faustbrian/go-clock/actions/workflows/ci.yml)
[![CodeQL](https://img.shields.io/badge/CodeQL-required-blue)](https://github.com/faustbrian/go-clock/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Mutation](https://img.shields.io/badge/mutation-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Documentation](https://img.shields.io/badge/docs-checked_in_CI-blue)](docs/)
[![Go Reference](https://pkg.go.dev/badge/github.com/faustbrian/go-clock.svg)](https://pkg.go.dev/github.com/faustbrian/go-clock)
[![Release](https://img.shields.io/github/v/release/faustbrian/go-clock?sort=semver)](https://github.com/faustbrian/go-clock/releases)
[![Go](https://img.shields.io/badge/go-1.27.0-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`clock` is a small, production-oriented clock foundation for Go 1.26.6 and
later. It keeps `time.Time` and `time.Duration` as public values, separates wall
time from elapsed time, and provides deterministic timers, tickers, sleeps, and
callbacks without changing the process-wide clock.

The module is a stable v1 public library. It owns explicit process-local time
capabilities and deterministic test clocks; it does not own calendars,
scheduling, distributed ordering, or a process-global clock.

Use the standard `time` package directly when no dependency seam is needed. Use
`testing/synctest` when a complete test can live inside one fake-time bubble.
Use this module when business timestamps, explicit wall jumps, package
contracts, or selectively controlled time require dependency injection.

## Install

```sh
go get github.com/faustbrian/go-clock@v1
```

The module has no runtime dependencies.

## Five-minute quickstarts

Current-`main` variants are compiler-checked in
[`example_test.go`](example_test.go). The installed-v1 manual-clock quick start
below is separately verified in a clean external module pinned to `v1.0.0`;
it handles every construction and wait error and releases the clock explicitly.

### System clock

Depend on only the capability an operation needs:

```go
func stamp(clock interface{ Now() time.Time }) time.Time {
    return clock.Now()
}

createdAt := stamp(clock.System{})
```

`System.Now` returns `time.Now()` unchanged, including its location and
process-local monotonic reading. `System.Sleep` owns and releases its timer when
the context is canceled.

### Fixed clock

```go
fixed := manual.NewFixed(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
fmt.Println(fixed.Now().Format(time.RFC3339))
// 2026-01-02T03:04:05Z
```

### Manual clock

```go
start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
manualClock, err := manual.New(start)
if err != nil {
    panic(err)
}
timer, err := manualClock.NewTimer(time.Minute)
if err != nil {
    panic(err)
}

waiter, err := manualClock.Advance(time.Minute)
if err != nil {
    panic(err)
}
if _, err := waiter.Wait(context.Background()); err != nil {
    panic(err)
}
fmt.Println((<-timer.C()).Format(time.RFC3339))
if err := manualClock.Shutdown(); err != nil {
    panic(err)
}
// 2026-01-02T03:05:05Z
```

Events fire by deadline and then registration order. A ticker has a one-value
buffer and drops backpressured ticks. Always stop resources that remain active,
and release the manual clock when its owner is done. The versioned quick start
uses `Shutdown` because it is available across the complete v1 line; `Close` is
the preferred additive name on current `main`, and `Shutdown` remains its
deprecated exact delegation.

### `testing/synctest`

```go
clocktest.SystemBubble(t, func(t *testing.T, system clock.System) {
    started := system.Now()
    require.NoError(t, system.Sleep(t.Context(), time.Hour))
    require.Equal(t, time.Hour, system.Since(started))
})
```

The helper delegates fake time and goroutine quiescence to the standard
library. It does not install another scheduler.

## Capability map

| Need | Interface |
| --- | --- |
| Business timestamp | `Clock` |
| Monotonic elapsed measurement | `ElapsedClock` |
| Cancelable bounded delay | `Sleeper` |
| Owned one-shot event | `TimerFactory` and `Timer` |
| Owned periodic event | `TickerFactory` and `Ticker` |
| Owned callback | `CallbackClock` and `Callback` |

`FullClock` is a convenience only. Libraries should accept the narrowest row
that meets their contract.

## Package map

All packages are released together from the root module and use root
`v<version>` tags.

| Import path | Package | Role |
| --- | --- | --- |
| `github.com/faustbrian/go-clock` | `clock` | Public capabilities, standard-library implementation, and bounded observations |
| `github.com/faustbrian/go-clock/manual` | `manual` | Public fixed and explicitly advanced deterministic clocks |
| `github.com/faustbrian/go-clock/clocktest` | `clocktest` | Test-support bridge to `testing/synctest` |

## Construction, defaults, and validation

`clock.System{}` is ready without construction and delegates to the standard
library. `manual.NewFixed` returns an immutable fixed wall clock.
`manual.New(start, options...)` strips the start value's process-local
monotonic reading and applies explicit resource limits before returning a
concurrency-safe clock. Its defaults permit 65,536 scheduled objects, 65,536
outstanding advancement waiters, and 1,000,000 triggered events per advance;
`manual.WithLimits` replaces those limits and rejects zero or negative values.

`clock.Observe` rejects nil clock and observer interfaces, a nil
`ObserverFunc`, and invalid tags before creating its wrapper. Other non-nil
dynamic values remain caller-owned collaborators. `WithTags` copies at most 16
non-empty-key tags with keys and values no longer than 64 bytes. A later
`WithTags` option replaces an earlier one, and nil options are ignored.
Constructors read no environment or filesystem configuration.

## Semantics at a glance

- `Advance` never accepts negative elapsed movement; use `Jump` for wall-clock
  rollback or forward correction.
- `Mark`, `SinceMark`, and `Measure` use manual monotonic progress and are not
  affected by `Jump`.
- Callbacks never run while an internal lock is held. They may create, stop, or
  reset work. A callback waiting for future work must issue and wait on a nested
  `Advance`; same-instant work wakes the active coordinator automatically.
- Callback panics are recovered by the manual clock and counted without keeping
  the payload. The system clock retains standard `time.AfterFunc` panic policy.
- Active objects and work per advancement are bounded. Invalid durations,
  overflow, closure, and exhausted budgets return documented errors.
- Observers receive bounded lifecycle metadata, never callback values, panic
  payloads, contexts, or timestamps.
- Sleep observations distinguish completed, deadline, canceled, and other
  failed outcomes while returning the exact base error.

## Documentation

- [Documentation index](docs/README.md)
- [API and ownership](docs/api.md)
- [Integration and adoption](docs/integration.md)
- [Concurrency and callback ownership](docs/concurrency.md)
- [Security model](docs/security-model.md)
- [Compatibility](COMPATIBILITY.md) and [migration](docs/migration.md)
- [Performance](docs/performance.md) and [operations troubleshooting](docs/troubleshooting.md)
- [Current-main executable examples](example_test.go) and [`testing/synctest` helpers](docs/synctest.md)
- [FAQ](docs/faq.md), [support](SUPPORT.md), and [release history](CHANGELOG.md)
- [API reference](https://pkg.go.dev/github.com/faustbrian/go-clock)
- [Private vulnerability reporting](SECURITY.md)

Use the versioned
[Golib ecosystem catalog](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/README.md)
and its [Foundations family guidance](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/design-language.md#package-families-and-selection)
to compare this module with related foundations and composition packages.

## Development

Run `make cohesion` for the repository's design-language contract and
`make check` for its package gates. See [CONTRIBUTING.md](CONTRIBUTING.md) for
focused and release verification.

## Scope

This module does not implement calendars, date-only values, timezone data,
interval algebra, cron, scheduling, distributed ordering, or a timestamp
oracle. `calendar`, `temporal`, `scheduler`, and `lease` own those
concerns.

## License

MIT. See [LICENSE](LICENSE).
