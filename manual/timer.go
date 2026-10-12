package manual

import (
	"time"

	clockpkg "github.com/faustbrian/go-clock"
)

// Timer is a concurrency-safe manually advanced one-shot timer created by
// Clock.NewTimer. Its zero value is not usable; the clock owns its schedule.
type Timer struct {
	clock   *Clock
	state   eventState
	channel chan time.Time
}

// C returns the timer's one-value buffered channel. It is never closed by
// Stop or Clock.Close. A delivered timestamp includes the current wall offset.
func (timer *Timer) C() <-chan time.Time { return timer.channel }

// Stop prevents an active timer from firing and releases its schedule. It
// returns false after firing or a prior stop and does not drain a buffered value.
func (timer *Timer) Stop() bool {
	timer.clock.mu.Lock()
	defer timer.clock.mu.Unlock()
	if !timer.state.active {
		return false
	}
	timer.state.active = false
	timer.clock.removeScheduledLocked(&timer.state)
	timer.clock.active--
	return true
}

// Reset reschedules the timer and reports whether it was active beforehand.
// Non-positive durations become due at the current instant. A successful reset
// drains any old buffered timestamp; rejected resets preserve schedule/value.
// Errors are ErrClosed, ErrActiveLimit when reactivating, or clock.ErrOverflow.
func (timer *Timer) Reset(duration time.Duration) (bool, error) {
	timer.clock.mu.Lock()
	defer timer.clock.mu.Unlock()
	if timer.clock.closed {
		return false, ErrClosed
	}
	wasActive := timer.state.active
	if !wasActive && timer.clock.active >= timer.clock.limits.MaxActive {
		return false, ErrActiveLimit
	}
	deadline, ok := deadlineAfter(timer.clock.elapsed, duration)
	if !ok {
		return wasActive, clockpkg.ErrOverflow
	}
	if timer.clock.sequence == ^uint64(0) {
		return wasActive, clockpkg.ErrOverflow
	}
	select {
	case <-timer.channel:
	default:
	}
	if !wasActive {
		timer.clock.active++
	}
	timer.state.active = true
	timer.clock.replaceScheduledLocked(&timer.state, deadline, timer)
	timer.clock.signalLocked()
	return wasActive, nil
}

// Ticker is a concurrency-safe manually advanced periodic timer created by
// Clock.NewTicker. Its zero value is not usable; the owner must Stop it.
type Ticker struct {
	clock    *Clock
	state    eventState
	channel  chan time.Time
	interval time.Duration
}

// C returns the ticker's one-value buffered channel; undrained later ticks are
// dropped. Stop and Clock.Close never close or drain it.
func (ticker *Ticker) C() <-chan time.Time { return ticker.channel }

// Stop prevents future ticks and releases its schedule. It is idempotent and
// does not remove a timestamp already buffered in C.
func (ticker *Ticker) Stop() {
	ticker.clock.mu.Lock()
	defer ticker.clock.mu.Unlock()
	if ticker.state.active {
		ticker.state.active = false
		ticker.clock.removeScheduledLocked(&ticker.state)
		ticker.clock.active--
	}
}

// Reset changes the period and schedules the next tick from current elapsed
// progress without draining an old buffered tick. Non-positive periods return
// clock.ErrInvalidDuration. ErrClosed, ErrActiveLimit when reactivating, and
// clock.ErrOverflow also reject the reset without changing the prior schedule.
func (ticker *Ticker) Reset(duration time.Duration) error {
	if duration <= 0 {
		return clockpkg.ErrInvalidDuration
	}
	ticker.clock.mu.Lock()
	defer ticker.clock.mu.Unlock()
	if ticker.clock.closed {
		return ErrClosed
	}
	wasActive := ticker.state.active
	if !wasActive && ticker.clock.active >= ticker.clock.limits.MaxActive {
		return ErrActiveLimit
	}
	if !wasActive {
		ticker.clock.active++
	}
	ticker.state.active = true
	if err := ticker.clock.rescheduleLocked(&ticker.state, duration, ticker); err != nil {
		if !wasActive {
			ticker.clock.active--
		}
		ticker.state.active = wasActive
		return err
	}
	ticker.interval = duration
	return nil
}

// Callback is a concurrency-safe scheduled function created by Clock.AfterFunc.
// Its zero value is not usable. Clock advancement owns started invocations;
// stopping or closing the clock cannot interrupt a function already running.
type Callback struct {
	clock    *Clock
	state    eventState
	function func()
}

// Stop prevents an active callback from starting and releases its schedule. It
// returns false for an already-started or stopped invocation and does not wait.
func (callback *Callback) Stop() bool {
	callback.clock.mu.Lock()
	defer callback.clock.mu.Unlock()
	if !callback.state.active {
		return false
	}
	callback.state.active = false
	callback.clock.removeScheduledLocked(&callback.state)
	callback.clock.active--
	return true
}

// Reset reschedules the callback and reports prior scheduling activity, not
// whether an invocation is running. Non-positive durations are immediately due.
// A running invocation is not awaited; explicit reentrant/concurrent progress
// may start an overlapping invocation. ErrClosed, ErrActiveLimit when
// reactivating, and clock.ErrOverflow preserve the prior schedule on rejection.
func (callback *Callback) Reset(duration time.Duration) (bool, error) {
	callback.clock.mu.Lock()
	defer callback.clock.mu.Unlock()
	if callback.clock.closed {
		return false, ErrClosed
	}
	wasActive := callback.state.active
	if !wasActive && callback.clock.active >= callback.clock.limits.MaxActive {
		return false, ErrActiveLimit
	}
	if !wasActive {
		callback.clock.active++
	}
	callback.state.active = true
	if err := callback.clock.rescheduleLocked(&callback.state, duration, callback); err != nil {
		if !wasActive {
			callback.clock.active--
		}
		callback.state.active = wasActive
		return wasActive, err
	}
	return wasActive, nil
}
