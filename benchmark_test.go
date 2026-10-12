package clock_test

import (
	"context"
	"testing"
	"time"

	clock "github.com/faustbrian/go-clock"
	"github.com/faustbrian/go-clock/manual"
)

// BenchmarkSystemNow measures the stateless System.Now call, including the
// underlying time.Now operation, without per-iteration construction.
func BenchmarkSystemNow(b *testing.B) {
	system := clock.System{}
	b.ReportAllocs()
	for b.Loop() {
		_ = system.Now()
	}
}

// BenchmarkSystemMeasure measures calls to one pre-captured elapsed closure;
// capturing the measurement origin is excluded from the measured loop.
func BenchmarkSystemMeasure(b *testing.B) {
	system := clock.System{}
	measure := system.Measure()
	b.ReportAllocs()
	for b.Loop() {
		_ = measure()
	}
}

// BenchmarkManualTimerFanout measures construction, equal-deadline registration,
// advancement and waiting for 1, 100 or 10,000 undrained timers. All setup is
// included; subtest names encode the timer count as a time.Duration string.
func BenchmarkManualTimerFanout(b *testing.B) {
	for _, count := range []int{1, 100, 10_000} {
		b.Run(time.Duration(count).String(), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				c, _ := manual.New(time.Unix(1, 0), manual.WithLimits(manual.Limits{MaxActive: count, MaxWorkPerAdvance: count}))
				for range count {
					_, _ = c.NewTimer(time.Second)
				}
				waiter, _ := c.Advance(time.Second)
				_, _ = waiter.Wait(context.Background())
			}
		})
	}
}

// BenchmarkManualColdStart measures clock construction at a fixed timestamp
// with default limits; no schedules or advancement are included.
func BenchmarkManualColdStart(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, _ = manual.New(time.Unix(1, 0))
	}
}

// BenchmarkManualNowContended measures parallel reads of one shared manual
// clock, including mutex contention. Construction happens once before the
// parallel reads and is not explicitly excluded from timing.
func BenchmarkManualNowContended(b *testing.B) {
	c, _ := manual.New(time.Unix(1, 0))
	b.ReportAllocs()
	b.RunParallel(func(parallel *testing.PB) {
		for parallel.Next() {
			_ = c.Now()
		}
	})
}
