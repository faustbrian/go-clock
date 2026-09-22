# Security model

Version: 1.0

## Assets and trust boundaries

The package protects process availability, deterministic test behavior, clock
state integrity, and observation confidentiality. Durations, callback code,
tags, and concurrent lifecycle operations are caller-controlled inputs.

## Controls

- Active timers, tickers, callbacks, and sleepers are capped by `MaxActive`.
- Outstanding advancement waiters are independently capped by `MaxActive`.
- Reset and stop remove superseded heap entries immediately.
- One advancement is capped by `MaxWorkPerAdvance`.
- Duration arithmetic and sequence allocation reject overflow.
- Wall rollback cannot reverse the manual monotonic counter.
- Callback and observer panics do not corrupt manual clock state.
- Observations bound tag cardinality/size and omit sensitive payloads.
- Close releases scheduled work and wakes owned waiters; deprecated `Shutdown`
  delegates to the same operation.
- Production code is scanned for `unsafe`, cgo, `go:linkname`, runtime patching,
  and global test-clock patterns.

The default limits are 65,536 active objects, 65,536 outstanding advancement
waiters, and 1,000,000 triggered events per advancement. Applications handling
untrusted schedules should configure lower budgets appropriate to their
request and memory limits.

## Accepted residual risks

| Risk | Owner and rationale | Mitigation | Review condition |
| --- | --- | --- | --- |
| A callback can block advancement indefinitely; Go cannot terminate arbitrary caller code. | Callback owner; callback lifetime is outside the clock's control. | Return promptly, honor application cancellation, and avoid waiting for future manual time without a nested advancement. | Revisit if callback execution or cancellation ownership changes. |
| Default active-object and work budgets may be too large for an untrusted schedule. | Application integrator; one package-wide default cannot represent every request and memory budget. | Configure lower `MaxActive` and `MaxWorkPerAdvance` values at the trust boundary. | Revisit if scheduling becomes reachable from untrusted input without caller configuration. |
| Caller-provided observation tags can contain sensitive or high-cardinality values despite size limits. | Application integrator; the package cannot classify caller-supplied tag meaning. | Use bounded, nonsensitive classifications rather than user, request, token, or payload values. | Revisit if tags are generated or exported by the package. |
| A process-local clock cannot establish trustworthy distributed ordering. | Distributed-system integrator; wall time is not a cross-process authority. | Use fencing and version protocols for cross-process decisions. | Revisit if the package adds distributed coordination or persistence. |
