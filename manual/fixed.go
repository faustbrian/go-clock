// Package manual provides fixed timestamps and explicitly advanced test clocks.
// NewFixed is an immutable wall-time seam; New constructs a concurrency-safe
// Clock whose elapsed progress is independent of Jump's wall-time corrections.
// The package does not replace process time or automatically advance blocked
// goroutines, and it requires no cgo or external service.
//
// Clock owns bounded schedules and advancement waiters. Call Close to release
// scheduled work. Advancements may start callback goroutines and wait for them;
// callbacks must cooperate and use a nested advancement waiter to permit future
// progress. Closing the clock does not wait for running callbacks.
package manual

import "time"

// Fixed is an immutable, concurrency-safe wall clock. Its zero value returns
// time.Time{}; it has no scheduled resources or release obligation.
type Fixed struct {
	now time.Time
}

// NewFixed returns an immutable clock fixed at now, preserving its location
// and any process-local monotonic reading. It accepts time.Time{}.
func NewFixed(now time.Time) Fixed {
	return Fixed{now: now}
}

// Now returns the fixed timestamp without changing its location.
func (fixed Fixed) Now() time.Time {
	return fixed.now
}

// Since returns the duration from start to the fixed timestamp. If both values
// contain compatible monotonic readings, time.Time.Sub uses them.
func (fixed Fixed) Since(start time.Time) time.Duration {
	return fixed.now.Sub(start)
}

// Measure returns an elapsed measurement that remains zero for a fixed clock.
func (Fixed) Measure() func() time.Duration {
	return func() time.Duration { return 0 }
}
