package clock_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	clock "github.com/faustbrian/go-clock"
	"github.com/faustbrian/go-clock/manual"
)

type sleepErrorClock struct {
	clock.System
	err      error
	returned bool
}

type identityError struct {
	message string
	cause   error
}

func (failure *identityError) Error() string { return failure.message }
func (failure *identityError) Unwrap() error { return failure.cause }

func (base *sleepErrorClock) Sleep(context.Context, time.Duration) error {
	base.returned = true
	return base.err
}

type callbackCaptureClock struct {
	clock.System
	function func()
}

func (base *callbackCaptureClock) AfterFunc(_ time.Duration, function func()) (clock.Callback, error) {
	base.function = function
	return callbackStub{}, nil
}

type callbackStub struct{}

func (callbackStub) Stop() bool                        { return true }
func (callbackStub) Reset(time.Duration) (bool, error) { return true, nil }

func TestObservedClockReportsBoundedLifecycleData(t *testing.T) {
	t.Parallel()

	base, err := manual.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	observations := make([]clock.Observation, 0, 4)
	observed, err := clock.Observe(base, clock.ObserverFunc(func(observation clock.Observation) {
		mu.Lock()
		defer mu.Unlock()
		observations = append(observations, observation)
	}), clock.WithTags(map[string]string{"component": "test"}))
	if err != nil {
		t.Fatal(err)
	}

	timer, err := observed.NewTimer(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !timer.Stop() {
		t.Fatal("Stop() = false")
	}
	callback, err := observed.AfterFunc(2*time.Second, func() {})
	if err != nil {
		t.Fatal(err)
	}
	_ = callback
	waiter, err := base.Advance(2 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := waiter.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(observations) != 4 {
		t.Fatalf("observations = %+v, want four", observations)
	}
	want := []struct {
		kind    clock.Kind
		outcome clock.Outcome
	}{{clock.KindTimer, clock.OutcomeCreated}, {clock.KindTimer, clock.OutcomeStopped},
		{clock.KindCallback, clock.OutcomeCreated}, {clock.KindCallback, clock.OutcomeFired}}
	for index, expected := range want {
		got := observations[index]
		if got.Kind != expected.kind || got.Outcome != expected.outcome {
			t.Fatalf("observation %d = %+v", index, got)
		}
		if got.Tags["component"] != "test" || len(got.Tags) != 1 {
			t.Fatalf("tags = %v", got.Tags)
		}
	}
	if observations[3].Requested != 2*time.Second || observations[3].Elapsed != 2*time.Second {
		t.Fatalf("callback observation = %+v", observations[3])
	}
}

func TestObserveValidatesInputsAndContainsObserverPanics(t *testing.T) {
	t.Parallel()

	if _, err := clock.Observe(nil, clock.ObserverFunc(func(clock.Observation) {})); !errors.Is(err, clock.ErrInvalidClock) {
		t.Fatalf("Observe(nil) error = %v", err)
	}
	if _, err := clock.Observe(clock.System{}, nil); !errors.Is(err, clock.ErrInvalidObserver) {
		t.Fatalf("Observe(nil observer) error = %v", err)
	}
	if _, err := clock.Observe(clock.System{}, clock.ObserverFunc(nil)); !errors.Is(err, clock.ErrInvalidObserver) {
		t.Fatalf("Observe(nil function) error = %v", err)
	}
	tags := make(map[string]string, clock.MaxObservationTags+1)
	for index := 0; index <= clock.MaxObservationTags; index++ {
		tags[string(rune('a'+index))] = "value"
	}
	if _, err := clock.Observe(clock.System{}, clock.ObserverFunc(func(clock.Observation) {}), clock.WithTags(tags)); !errors.Is(err, clock.ErrObservationTags) {
		t.Fatalf("Observe(tags) error = %v", err)
	}
	for _, invalid := range []map[string]string{{"": "value"}, {"key": string(make([]byte, clock.MaxObservationTagBytes+1))}} {
		if _, err := clock.Observe(clock.System{}, clock.ObserverFunc(func(clock.Observation) {}), clock.WithTags(invalid)); !errors.Is(err, clock.ErrObservationTags) {
			t.Fatalf("Observe(invalid tags) error = %v", err)
		}
	}
	boundaryTags := make(map[string]string, clock.MaxObservationTags)
	for index := range clock.MaxObservationTags {
		key := string(rune('a'+index)) + strings.Repeat("k", clock.MaxObservationTagBytes-1)
		boundaryTags[key] = strings.Repeat("v", clock.MaxObservationTagBytes)
	}
	if _, err := clock.Observe(clock.System{}, clock.ObserverFunc(func(clock.Observation) {}), clock.WithTags(boundaryTags)); err != nil {
		t.Fatalf("Observe(maximum tags) error = %v", err)
	}

	observed, err := clock.Observe(clock.System{}, clock.ObserverFunc(func(clock.Observation) { panic("observer") }))
	if err != nil {
		t.Fatal(err)
	}
	timer, err := observed.NewTimer(time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !timer.Stop() {
		t.Fatal("observer panic corrupted timer creation")
	}
}

func TestObserveSkipsNilOptionsWithoutSkippingLaterValidation(t *testing.T) {
	t.Parallel()

	_, err := clock.Observe(
		clock.System{},
		clock.ObserverFunc(func(clock.Observation) {}),
		nil,
		clock.WithTags(map[string]string{"": "invalid"}),
	)
	if !errors.Is(err, clock.ErrObservationTags) {
		t.Fatalf("Observe(nil, invalid option) error = %v, want ErrObservationTags", err)
	}
}

func TestObservedClockDelegatesCapabilitiesAndAllLifecycleTransitions(t *testing.T) {
	t.Parallel()

	base, err := manual.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	observations := make([]clock.Observation, 0, 16)
	observed, err := clock.Observe(base, clock.ObserverFunc(func(observation clock.Observation) {
		mu.Lock()
		observations = append(observations, observation)
		mu.Unlock()
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !observed.Now().Equal(base.Now()) || observed.Since(base.Now()) != 0 || observed.Measure()() != 0 {
		t.Fatal("wall or elapsed capability did not delegate")
	}
	if err := observed.Sleep(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := observed.Sleep(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("Sleep() error = %v", err)
	}
	mu.Lock()
	if len(observations) != 2 || observations[0].Kind != clock.KindSleep ||
		observations[0].Outcome != clock.OutcomeCompleted ||
		observations[1].Kind != clock.KindSleep ||
		observations[1].Outcome != clock.OutcomeCanceled {
		mu.Unlock()
		t.Fatalf("sleep observations = %+v", observations)
	}
	mu.Unlock()

	timer, err := observed.NewTimer(time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if active, err := timer.Reset(time.Minute); err != nil || !active {
		t.Fatalf("timer Reset() = (%v, %v)", active, err)
	}
	timer.Stop()
	timer.Stop()

	ticker, err := observed.NewTicker(time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := ticker.Reset(time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := ticker.Reset(0); !errors.Is(err, clock.ErrInvalidDuration) {
		t.Fatalf("ticker Reset() error = %v", err)
	}
	ticker.Stop()
	if _, err := observed.NewTicker(0); !errors.Is(err, clock.ErrInvalidDuration) {
		t.Fatalf("NewTicker() error = %v", err)
	}

	callback, err := observed.AfterFunc(time.Hour, func() {})
	if err != nil {
		t.Fatal(err)
	}
	if active, err := callback.Reset(time.Minute); err != nil || !active {
		t.Fatalf("callback Reset() = (%v, %v)", active, err)
	}
	callback.Stop()
	callback.Stop()
	if _, err := observed.AfterFunc(time.Second, nil); !errors.Is(err, clock.ErrInvalidCallback) {
		t.Fatalf("AfterFunc(nil) error = %v", err)
	}

	panicking, err := observed.AfterFunc(0, func() { panic("payload") })
	if err != nil {
		t.Fatal(err)
	}
	_ = panicking
	waiter, err := base.Advance(0)
	if err != nil {
		t.Fatal(err)
	}
	result, err := waiter.Wait(context.Background())
	if err != nil || result.Panics != 1 {
		t.Fatalf("panic advancement = (%+v, %v)", result, err)
	}

	mu.Lock()
	defer mu.Unlock()
	foundPanic := false
	foundRejected := false
	foundCompleted := false
	foundCanceled := false
	foundReset := false
	foundInactive := false
	for _, observation := range observations {
		foundPanic = foundPanic || observation.Outcome == clock.OutcomePanicked
		foundRejected = foundRejected || observation.Outcome == clock.OutcomeRejected
		foundCompleted = foundCompleted || observation.Outcome == clock.OutcomeCompleted
		foundCanceled = foundCanceled || observation.Outcome == clock.OutcomeCanceled
		foundReset = foundReset || observation.Outcome == clock.OutcomeReset
		foundInactive = foundInactive || observation.Outcome == clock.OutcomeInactive
	}
	if !foundPanic || !foundRejected || !foundCompleted || !foundCanceled || !foundReset || !foundInactive {
		t.Fatalf("observations lack required outcomes: %+v", observations)
	}
}

func TestObservedClockReportsFactoryAndResetErrors(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	observations := make([]clock.Observation, 0, 12)
	record := clock.ObserverFunc(func(observation clock.Observation) {
		mu.Lock()
		defer mu.Unlock()
		observations = append(observations, observation)
	})

	limited, err := manual.New(time.Unix(1, 0), manual.WithLimits(manual.Limits{MaxActive: 1, MaxWorkPerAdvance: 10}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := limited.NewTimer(time.Hour); err != nil {
		t.Fatal(err)
	}
	observed, err := clock.Observe(limited, record)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := observed.NewTimer(time.Hour); !errors.Is(err, manual.ErrActiveLimit) {
		t.Fatalf("NewTimer() error = %v", err)
	}
	if _, err := observed.AfterFunc(time.Hour, func() {}); !errors.Is(err, manual.ErrActiveLimit) {
		t.Fatalf("AfterFunc() error = %v", err)
	}

	base, err := manual.New(time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	observed, err = clock.Observe(base, record)
	if err != nil {
		t.Fatal(err)
	}
	timer, err := observed.NewTimer(time.Duration(1<<63 - 1))
	if err != nil {
		t.Fatal(err)
	}
	callback, err := observed.AfterFunc(time.Duration(1<<63-1), func() {})
	if err != nil {
		t.Fatal(err)
	}
	ticker, err := observed.NewTicker(time.Duration(1<<63 - 1))
	if err != nil {
		t.Fatal(err)
	}
	waiter, err := base.Advance(time.Duration(1<<63 - 1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := waiter.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := timer.Reset(1); !errors.Is(err, clock.ErrOverflow) {
		t.Fatalf("timer Reset() error = %v", err)
	}
	if _, err := callback.Reset(1); !errors.Is(err, clock.ErrOverflow) {
		t.Fatalf("callback Reset() error = %v", err)
	}
	if err := ticker.Reset(1); !errors.Is(err, clock.ErrOverflow) {
		t.Fatalf("ticker Reset() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	rejected := map[clock.Kind]int{}
	for _, observation := range observations {
		if observation.Outcome == clock.OutcomeRejected {
			rejected[observation.Kind]++
		}
	}
	if rejected[clock.KindTimer] != 2 || rejected[clock.KindCallback] != 2 || rejected[clock.KindTicker] != 1 {
		t.Fatalf("rejected observations = %v; all factory/reset errors must be classified", rejected)
	}
}

func TestObservedSleepClassifiesExactReturnedError(t *testing.T) {
	t.Parallel()

	failed := &identityError{message: "sleep failed"}
	deadline := &identityError{message: "sleep deadline", cause: context.DeadlineExceeded}
	canceled := &identityError{message: "sleep canceled", cause: context.Canceled}
	wrappedFailed := &identityError{message: "wrapped failure", cause: failed}
	for _, test := range []struct {
		name    string
		err     error
		exact   *identityError
		outcome clock.Outcome
	}{
		{name: "completed", outcome: clock.OutcomeCompleted},
		{name: "direct deadline", err: context.DeadlineExceeded, outcome: clock.OutcomeDeadline},
		{name: "wrapped deadline", err: deadline, exact: deadline, outcome: clock.OutcomeDeadline},
		{name: "canceled", err: canceled, exact: canceled, outcome: clock.OutcomeCanceled},
		{name: "direct failure", err: failed, exact: failed, outcome: clock.OutcomeFailed},
		{name: "wrapped failure", err: wrappedFailed, exact: wrappedFailed, outcome: clock.OutcomeFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			var observations []clock.Observation
			base := &sleepErrorClock{err: test.err}
			observed, err := clock.Observe(base, clock.ObserverFunc(func(observation clock.Observation) {
				if !base.returned {
					t.Fatal("sleep observation emitted before base return")
				}
				observations = append(observations, observation)
				panic("observer panic")
			}))
			if err != nil {
				t.Fatal(err)
			}
			got := observed.Sleep(context.Background(), time.Second)
			if test.err == nil && got != nil {
				t.Fatalf("Sleep() error = %v, want exact %v", got, test.err)
			}
			if test.exact != nil {
				var gotExact *identityError
				if reflect.TypeOf(got) != reflect.TypeOf(test.exact) ||
					!errors.As(got, &gotExact) || gotExact != test.exact {
					t.Fatalf("Sleep() error = %#v, want exact %#v", got, test.exact)
				}
			} else if test.err != nil && !errors.Is(got, test.err) {
				t.Fatalf("Sleep() error = %v, want %v", got, test.err)
			}
			if len(observations) != 1 || observations[0].Kind != clock.KindSleep || observations[0].Outcome != test.outcome {
				t.Fatalf("observations = %+v", observations)
			}
		})
	}
}

func TestObservedCallbackStopLosingToStartReportsInactiveOnce(t *testing.T) {
	t.Parallel()

	base, err := manual.New(time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var terminal []clock.Outcome
	observed, err := clock.Observe(base, clock.ObserverFunc(func(observation clock.Observation) {
		if observation.Kind == clock.KindCallback && observation.Outcome != clock.OutcomeCreated {
			mu.Lock()
			terminal = append(terminal, observation.Outcome)
			mu.Unlock()
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	callback, err := observed.AfterFunc(0, func() {
		close(started)
		<-release
	})
	if err != nil {
		t.Fatal(err)
	}
	type advanceResult struct {
		waiter *manual.Waiter
		err    error
	}
	advanced := make(chan advanceResult, 1)
	go func() {
		waiter, advanceErr := base.Advance(0)
		advanced <- advanceResult{waiter: waiter, err: advanceErr}
	}()
	<-started
	if callback.Stop() {
		t.Fatal("Stop() prevented a callback that had already started")
	}
	close(release)
	result := <-advanced
	if result.err != nil {
		t.Fatal(result.err)
	}
	if _, err := result.waiter.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(terminal) != 2 || terminal[0] != clock.OutcomeInactive || terminal[1] != clock.OutcomeFired {
		t.Fatalf("terminal outcomes = %v", terminal)
	}
}

func TestObservedTickerReportsOnlyActiveStopTransitions(t *testing.T) {
	t.Parallel()

	base, err := manual.New(time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var outcomes []clock.Outcome
	observed, err := clock.Observe(base, clock.ObserverFunc(func(observation clock.Observation) {
		if observation.Kind == clock.KindTicker && (observation.Outcome == clock.OutcomeStopped || observation.Outcome == clock.OutcomeInactive) {
			mu.Lock()
			outcomes = append(outcomes, observation.Outcome)
			mu.Unlock()
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	ticker, err := observed.NewTicker(time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	var workers sync.WaitGroup
	for range 32 {
		workers.Go(ticker.Stop)
	}
	workers.Wait()
	if err := ticker.Reset(0); !errors.Is(err, clock.ErrInvalidDuration) {
		t.Fatalf("inactive Reset(0) error = %v", err)
	}
	ticker.Stop()
	if err := ticker.Reset(time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := ticker.Reset(0); !errors.Is(err, clock.ErrInvalidDuration) {
		t.Fatalf("active Reset(0) error = %v", err)
	}
	ticker.Stop()
	ticker.Stop()

	mu.Lock()
	defer mu.Unlock()
	stopped := 0
	inactive := 0
	for _, outcome := range outcomes {
		if outcome == clock.OutcomeStopped {
			stopped++
		} else {
			inactive++
		}
	}
	if stopped != 2 || inactive != 33 {
		t.Fatalf("stop outcomes = %v, want 2 stopped and 33 inactive", outcomes)
	}
}

func TestObservedCallbackReportsTerminalOutcomeBeforeReturningOrRepanicking(t *testing.T) {
	t.Parallel()

	base := &callbackCaptureClock{}
	var outcomes []clock.Outcome
	observed, err := clock.Observe(base, clock.ObserverFunc(func(observation clock.Observation) {
		if observation.Kind == clock.KindCallback && (observation.Outcome == clock.OutcomeFired || observation.Outcome == clock.OutcomePanicked) {
			outcomes = append(outcomes, observation.Outcome)
			if observation.Outcome == clock.OutcomePanicked {
				panic("observer panic")
			}
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	returned := false
	_, err = observed.AfterFunc(0, func() {
		if len(outcomes) != 0 {
			t.Fatalf("terminal outcome emitted before callback return: %v", outcomes)
		}
		returned = true
	})
	if err != nil {
		t.Fatal(err)
	}
	base.function()
	if !returned || len(outcomes) != 1 || outcomes[0] != clock.OutcomeFired {
		t.Fatalf("normal callback = (%v, %v)", returned, outcomes)
	}

	payload := &struct{ value string }{"exact payload"}
	_, err = observed.AfterFunc(0, func() {
		if len(outcomes) != 1 {
			t.Fatalf("panic outcome emitted before callback panic: %v", outcomes)
		}
		panic(payload)
	})
	if err != nil {
		t.Fatal(err)
	}
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		base.function()
	}()
	if recovered != payload {
		t.Fatalf("recovered payload = %#v, want exact %#v", recovered, payload)
	}
	if len(outcomes) != 2 || outcomes[1] != clock.OutcomePanicked {
		t.Fatalf("panic outcomes = %v", outcomes)
	}
}
