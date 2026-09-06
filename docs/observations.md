# Observations

`Observe` wraps a `FullClock` without a global registry, exporter, or background
goroutine. The observer is called synchronously after lifecycle transitions and
must return promptly. Observer panics are recovered and cannot corrupt clock
behavior.

An observation contains only kind, outcome, requested duration, monotonic
elapsed duration, and a defensive copy of bounded tags. It never contains a
callback, panic payload, context, channel value, or timestamp.

At most 16 tags are accepted. Keys must be non-empty; keys and values are at
most 64 bytes. Invalid tags reject wrapper construction. Tags should describe a
bounded component or operation, never a user, request, token, or raw error.

Channel consumption is caller-owned, so the generic wrapper observes resource
creation, stop, and reset but does not start a proxy goroutine merely to report
timer/ticker delivery. Callback and sleep completion outcomes are observable
without changing channel semantics.

Sleep emits exactly one observation after the base call returns. A nil error is
`completed`; an error matching `context.DeadlineExceeded` is `deadline`; one
matching `context.Canceled` is `canceled`; every other error is `failed`. The
wrapper returns the exact original error.

Ticker stop observations are transition-based: the first active stop is
`stopped`, repeated or concurrent stops are `inactive`, and a successful reset
reactivates the transition. Callback observations are `fired` only after normal
return and `panicked` while unwinding before the original panic payload is
re-panicked. Observer panics remain isolated on both paths.
