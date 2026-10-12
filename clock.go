// Package clock provides narrow time capabilities backed by the standard
// library and deterministic implementations for tests. Accept the narrowest
// capability interface a consumer needs; System's zero value is the normal
// production implementation, while manual supplies explicit test-time control.
// Observe adds synchronous hooks without installing an exporter or registry.
//
// The package does not own calendars, cron scheduling, distributed ordering,
// or process-global time. All implementations use Go time values; no cgo or
// external service is required.
//
// Wall-clock timestamps and monotonic elapsed time are deliberately distinct.
// A time.Time returned by System.Now retains the process-local monotonic reading
// supplied by time.Now. Serialization removes that reading, so persisted values
// must never be used as a substitute for monotonic elapsed measurement.
package clock

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrInvalidDuration reports a duration that is not valid for an operation.
	ErrInvalidDuration = errors.New("clock: invalid duration")
	// ErrInvalidCallback reports a nil callback function.
	ErrInvalidCallback = errors.New("clock: invalid callback")
	// ErrOverflow reports unrepresentable duration arithmetic or exhaustion of
	// the manual clock's event registration sequence.
	ErrOverflow = errors.New("clock: duration overflow")
)

// Clock obtains the current wall-clock time.
type Clock interface {
	// Now returns the implementation's current wall timestamp.
	Now() time.Time
}

// ElapsedClock measures implementation-local elapsed time. Since accepts a time.Time
// with time.Time.Sub semantics, including wall subtraction when no compatible
// monotonic readings exist. Measure returns a closure tied to the implementation's
// monotonic source and is the safe choice across explicit manual wall-clock jumps.
type ElapsedClock interface {
	// Since measures from start using the implementation's timestamp semantics.
	// Manual clocks cannot recover elapsed progress from a wall timestamp.
	Since(time.Time) time.Duration
	// Measure captures elapsed progress and returns a reusable measurement.
	// Fixed clocks always report zero; manual wall jumps do not affect it.
	Measure() func() time.Duration
}

// Sleeper waits for a duration or until its context is canceled.
type Sleeper interface {
	// Sleep accepts a non-nil context and returns nil on completion or an error
	// on cancellation or resource failure. Non-positive durations need no wait.
	// Context cancellation does not stop other resources owned by the clock.
	Sleep(context.Context, time.Duration) error
}

// Timer is an owned, one-shot time event.
//
// The owner must stop a timer it no longer needs. System uses the synchronous
// channel semantics of Go 1.23 and later; manual timers buffer one value and
// retain an already-delivered value after Stop. Implementations document their
// delivery and reset rules. A timer channel is not closed by Stop.
type Timer interface {
	// C returns the caller-owned receive side; callers must not close it.
	C() <-chan time.Time
	// Stop reports whether it stopped an active timer. It does not close C.
	Stop() bool
	// Reset reschedules from the current instant and reports prior activity.
	// Non-positive durations are immediately due. Rejected manual resets leave
	// the prior schedule/value intact; successful manual resets drain old values.
	Reset(time.Duration) (bool, error)
}

// TimerFactory creates owned one-shot timers.
type TimerFactory interface {
	// NewTimer returns an owned timer, or nil and an error on failure.
	// Non-positive durations are immediately due rather than invalid.
	NewTimer(time.Duration) (Timer, error)
}

// Ticker is an owned periodic time event.
//
// The owner must call Stop. Ticks may be dropped for a slow receiver, matching
// time.Ticker. Reset returns ErrInvalidDuration for a non-positive duration
// instead of exposing the standard library's panic across an interface seam.
type Ticker interface {
	// C returns tick timestamps; slow receivers may miss ticks.
	C() <-chan time.Time
	// Stop idempotently prevents future ticks without closing C.
	Stop()
	// Reset starts a new period from the current instant. Non-positive periods
	// return ErrInvalidDuration without changing the existing schedule.
	Reset(time.Duration) error
}

// TickerFactory creates owned periodic tickers.
type TickerFactory interface {
	// NewTicker requires a positive period and transfers ownership on success.
	// Invalid periods return nil and ErrInvalidDuration.
	NewTicker(time.Duration) (Ticker, error)
}

// Callback is an owned timer callback.
//
// Stop reports whether it prevented the callback from starting. It does not
// wait for an already-started callback to finish.
type Callback interface {
	// Stop reports whether it prevented a scheduled invocation from starting.
	// It does not wait for an invocation already running.
	Stop() bool
	// Reset reports prior scheduling activity and schedules another invocation.
	// It does not wait for a running invocation, so calls can overlap.
	Reset(time.Duration) (bool, error)
}

// CallbackClock creates owned timer callbacks.
type CallbackClock interface {
	// AfterFunc schedules a non-nil function and transfers callback ownership.
	// A nil function returns nil and ErrInvalidCallback; non-positive durations
	// are immediately due. Panic and execution policy depend on the backend.
	AfterFunc(time.Duration, func()) (Callback, error)
}

// FullClock is a convenience composition for consumers that genuinely need
// every capability. Consumers should normally accept a narrower interface.
type FullClock interface {
	Clock
	ElapsedClock
	Sleeper
	TimerFactory
	TickerFactory
	CallbackClock
}
