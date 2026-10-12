package clock

import (
	"context"
	"errors"
	"maps"
	"sync"
	"time"
)

const (
	// MaxObservationTags bounds labels attached to one observation.
	MaxObservationTags = 16
	// MaxObservationTagBytes bounds each tag key and value.
	MaxObservationTagBytes = 64
)

var (
	// ErrInvalidClock reports a nil clock passed to Observe.
	ErrInvalidClock = errors.New("clock: invalid clock")
	// ErrInvalidObserver reports a nil observer passed to Observe.
	ErrInvalidObserver = errors.New("clock: invalid observer")
	// ErrObservationTags reports tags outside the documented bounds.
	ErrObservationTags = errors.New("clock: invalid observation tags")
)

// Kind identifies an observed time resource.
type Kind string

const (
	// KindSleep identifies a context-aware sleep.
	KindSleep Kind = "sleep"
	// KindTimer identifies a one-shot channel timer.
	KindTimer Kind = "timer"
	// KindTicker identifies a periodic channel ticker.
	KindTicker Kind = "ticker"
	// KindCallback identifies an AfterFunc callback.
	KindCallback Kind = "callback"
)

// Outcome identifies an observed lifecycle transition.
type Outcome string

const (
	// OutcomeCreated reports successful resource creation.
	OutcomeCreated Outcome = "created"
	// OutcomeCompleted reports successful synchronous completion.
	OutcomeCompleted Outcome = "completed"
	// OutcomeCanceled reports context cancellation.
	OutcomeCanceled Outcome = "canceled"
	// OutcomeDeadline reports context deadline expiration.
	OutcomeDeadline Outcome = "deadline"
	// OutcomeFailed reports another synchronous operation failure.
	OutcomeFailed Outcome = "failed"
	// OutcomeStopped reports a successful active-to-stopped transition.
	OutcomeStopped Outcome = "stopped"
	// OutcomeInactive reports an operation on an inactive resource.
	OutcomeInactive Outcome = "inactive"
	// OutcomeReset reports successful rescheduling.
	OutcomeReset Outcome = "reset"
	// OutcomeFired reports normal callback return.
	OutcomeFired Outcome = "fired"
	// OutcomePanicked reports a callback panic without its payload.
	OutcomePanicked Outcome = "panicked"
	// OutcomeRejected reports validation or resource rejection.
	OutcomeRejected Outcome = "rejected"
)

// Observation contains bounded lifecycle metadata. Clock-generated metadata
// excludes callback functions, panic payloads, timestamps and contexts. Tag
// contents are caller-supplied and are not redacted; never attach secrets.
type Observation struct {
	// Kind identifies the operation or resource being observed.
	Kind Kind
	// Outcome identifies the transition, completion or rejection.
	Outcome Outcome
	// Requested is the duration supplied to creation, sleep or reset.
	// Stop observations have no requested duration and use zero.
	Requested time.Duration
	// Elapsed measures sleep or callback progress from the original call.
	// Resource creation, stop and reset observations leave it zero.
	Elapsed time.Duration
	// Tags is a fresh map per observation, or nil when no tags are configured.
	// The observer may retain or mutate it without affecting other observations.
	Tags map[string]string
}

// Observer consumes lifecycle metadata synchronously. Implementations must
// return promptly and synchronize their own state: concurrent clock operations
// and callbacks may call Observe concurrently. Calls happen outside wrapper
// locks; reentrant observed operations produce nested notifications. Panics are
// isolated, but blocking an observer blocks the corresponding clock operation.
type Observer interface {
	// Observe consumes one observation and may retain its independently owned tags.
	Observe(Observation)
}

// ObserverFunc adapts a function to Observer.
type ObserverFunc func(Observation)

// Observe calls function with observation synchronously. A nil function panics
// when called directly; the clock decorator recovers observer panics.
func (function ObserverFunc) Observe(observation Observation) {
	function(observation)
}

// ObserveOption configures an observed clock.
type ObserveOption func(*observeConfig) error

type observeConfig struct {
	tags map[string]string
}

// WithTags validates and copies tags when Observe applies the option, then
// supplies a separate copy to each observation. Keys must be non-empty; at most
// MaxObservationTags entries and MaxObservationTagBytes bytes per key/value are
// accepted. Nil tags are valid. A later WithTags replaces an earlier one.
// Do not mutate the input concurrently with construction or put secrets in tags.
func WithTags(tags map[string]string) ObserveOption {
	return func(config *observeConfig) error {
		if len(tags) > MaxObservationTags {
			return ErrObservationTags
		}
		config.tags = make(map[string]string, len(tags))
		for key, value := range tags {
			if key == "" || len(key) > MaxObservationTagBytes || len(value) > MaxObservationTagBytes {
				return ErrObservationTags
			}
			config.tags[key] = value
		}
		return nil
	}
}

// Observe decorates a FullClock with bounded synchronous lifecycle hooks. It
// starts no goroutine and owns no exporter or global registry. The caller still
// owns the base clock and its release; the wrapper does not expose base-specific
// methods such as manual.Clock.Close.
//
// Nil interfaces and a nil ObserverFunc are rejected; other typed-nil dynamic
// values are not detected. Nil options are ignored. Clock validation failures
// return nil with ErrInvalidClock, ErrInvalidObserver or the option's error.
// Concurrent use requires a concurrency-safe base and observer.
//
// Now, Since and Measure delegate without notifications. Timer/ticker channel
// receives are not observed. Sleep reports after return without changing its
// error; callbacks report after return or during panic unwinding and re-panic
// the original value. Callback observations retain the initial duration and
// measurement origin even after Reset; resets have their own observations.
func Observe(base FullClock, observer Observer, options ...ObserveOption) (FullClock, error) {
	if base == nil {
		return nil, ErrInvalidClock
	}
	if observer == nil {
		return nil, ErrInvalidObserver
	}
	if function, ok := observer.(ObserverFunc); ok && function == nil {
		return nil, ErrInvalidObserver
	}
	configuration := observeConfig{}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&configuration); err != nil {
			return nil, err
		}
	}
	return &observedClock{base: base, observer: observer, tags: configuration.tags}, nil
}

type observedClock struct {
	base     FullClock
	observer Observer
	tags     map[string]string
}

func (clock *observedClock) Now() time.Time { return clock.base.Now() }
func (clock *observedClock) Since(start time.Time) time.Duration {
	return clock.base.Since(start)
}
func (clock *observedClock) Measure() func() time.Duration { return clock.base.Measure() }

func (clock *observedClock) Sleep(ctx context.Context, duration time.Duration) error {
	elapsed := clock.base.Measure()
	err := clock.base.Sleep(ctx, duration)
	var outcome Outcome
	switch {
	case err == nil:
		outcome = OutcomeCompleted
	case errors.Is(err, context.DeadlineExceeded):
		outcome = OutcomeDeadline
	case errors.Is(err, context.Canceled):
		outcome = OutcomeCanceled
	default:
		outcome = OutcomeFailed
	}
	clock.report(Observation{Kind: KindSleep, Outcome: outcome, Requested: duration, Elapsed: elapsed()})
	return err
}

func (clock *observedClock) NewTimer(duration time.Duration) (Timer, error) {
	timer, err := clock.base.NewTimer(duration)
	if err != nil {
		clock.report(Observation{Kind: KindTimer, Outcome: OutcomeRejected, Requested: duration})
		return nil, err
	}
	clock.report(Observation{Kind: KindTimer, Outcome: OutcomeCreated, Requested: duration})
	return &observedTimer{Timer: timer, clock: clock}, nil
}

func (clock *observedClock) NewTicker(duration time.Duration) (Ticker, error) {
	ticker, err := clock.base.NewTicker(duration)
	if err != nil {
		clock.report(Observation{Kind: KindTicker, Outcome: OutcomeRejected, Requested: duration})
		return nil, err
	}
	clock.report(Observation{Kind: KindTicker, Outcome: OutcomeCreated, Requested: duration})
	return &observedTicker{Ticker: ticker, clock: clock, active: true}, nil
}

func (clock *observedClock) AfterFunc(duration time.Duration, function func()) (Callback, error) {
	if function == nil {
		clock.report(Observation{Kind: KindCallback, Outcome: OutcomeRejected, Requested: duration})
		return nil, ErrInvalidCallback
	}
	elapsed := clock.base.Measure()
	wrapper := func() {
		defer func() {
			if payload := recover(); payload != nil {
				clock.report(Observation{Kind: KindCallback, Outcome: OutcomePanicked, Requested: duration, Elapsed: elapsed()})
				panic(payload)
			}
			clock.report(Observation{Kind: KindCallback, Outcome: OutcomeFired, Requested: duration, Elapsed: elapsed()})
		}()
		function()
	}
	callback, err := clock.base.AfterFunc(duration, wrapper)
	if err != nil {
		clock.report(Observation{Kind: KindCallback, Outcome: OutcomeRejected, Requested: duration})
		return nil, err
	}
	clock.report(Observation{Kind: KindCallback, Outcome: OutcomeCreated, Requested: duration})
	return &observedCallback{Callback: callback, clock: clock}, nil
}

func (clock *observedClock) report(observation Observation) {
	observation.Tags = cloneTags(clock.tags)
	defer func() { _ = recover() }()
	clock.observer.Observe(observation)
}

func cloneTags(tags map[string]string) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	clone := make(map[string]string, len(tags))
	maps.Copy(clone, tags)
	return clone
}

type observedTimer struct {
	Timer
	clock *observedClock
}

func (timer *observedTimer) Stop() bool {
	stopped := timer.Timer.Stop()
	outcome := OutcomeInactive
	if stopped {
		outcome = OutcomeStopped
	}
	timer.clock.report(Observation{Kind: KindTimer, Outcome: outcome})
	return stopped
}

func (timer *observedTimer) Reset(duration time.Duration) (bool, error) {
	active, err := timer.Timer.Reset(duration)
	outcome := OutcomeReset
	if err != nil {
		outcome = OutcomeRejected
	}
	timer.clock.report(Observation{Kind: KindTimer, Outcome: outcome, Requested: duration})
	return active, err
}

type observedTicker struct {
	Ticker
	clock  *observedClock
	mu     sync.Mutex
	active bool
}

func (ticker *observedTicker) Stop() {
	ticker.mu.Lock()
	ticker.Ticker.Stop()
	outcome := OutcomeInactive
	if ticker.active {
		ticker.active = false
		outcome = OutcomeStopped
	}
	// Release transition ownership before invoking caller code; reentrant
	// observer operations must not deadlock on the lifecycle mutex.
	ticker.mu.Unlock()
	ticker.clock.report(Observation{Kind: KindTicker, Outcome: outcome})
}

func (ticker *observedTicker) Reset(duration time.Duration) error {
	ticker.mu.Lock()
	err := ticker.Ticker.Reset(duration)
	outcome := OutcomeReset
	if err != nil {
		outcome = OutcomeRejected
	} else {
		ticker.active = true
	}
	ticker.mu.Unlock()
	ticker.clock.report(Observation{Kind: KindTicker, Outcome: outcome, Requested: duration})
	return err
}

type observedCallback struct {
	Callback
	clock *observedClock
}

func (callback *observedCallback) Stop() bool {
	stopped := callback.Callback.Stop()
	outcome := OutcomeInactive
	if stopped {
		outcome = OutcomeStopped
	}
	callback.clock.report(Observation{Kind: KindCallback, Outcome: outcome})
	return stopped
}

func (callback *observedCallback) Reset(duration time.Duration) (bool, error) {
	active, err := callback.Callback.Reset(duration)
	outcome := OutcomeReset
	if err != nil {
		outcome = OutcomeRejected
	}
	callback.clock.report(Observation{Kind: KindCallback, Outcome: outcome, Requested: duration})
	return active, err
}
