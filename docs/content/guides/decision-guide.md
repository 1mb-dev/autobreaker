---
title: "Decision Guide"
weight: 6
---

# Decision Guide: When and How to Use AutoBreaker

## When to Use a Circuit Breaker

Use one around calls to something that can fail or slow down on its own: another service, a third-party API, a database, network storage. When the dependency is unhealthy, the breaker fails calls fast instead of letting them pile up, and probes it again after `Timeout`.

Skip it for in-process work (computation, pure functions): there is no dependency to protect, and an open breaker only turns a bug into `ErrOpenState`.

Whatever calls the breaker needs a plan for `ErrOpenState` and `ErrTooManyRequests`: a fallback, a cached value, or a clear error to the caller.

## Static or Adaptive

The default is static: the breaker trips when `ConsecutiveFailures > 5`, the same rule as sony/gobreaker. It needs no tuning. At low traffic, leave `Interval` at 0, or make each `Interval` long enough to hold six failures in a row: every `Interval` resets the counts.

Set `AdaptiveThreshold: true` to trip on the failure rate instead: the breaker opens when more than `FailureRateThreshold` of the requests in the current window fail, once the window holds at least `MinimumObservations` requests.

| Your traffic | Choose |
|--------------|--------|
| Varies a lot over the day, or between environments | Adaptive, with an `Interval` |
| Steady, and a run of failures is the signal you care about | Static (default) |
| Very low, and you cannot make one window hold `MinimumObservations` requests | Static (default) |
| Bursty batch jobs | Static, or adaptive with a high threshold |

Adaptive mode has no consecutive-failure fallback: a window with fewer than `MinimumObservations` requests never trips, even if every request fails. Always set `Interval` with adaptive mode; with `Interval: 0`, counts reset only on state changes, and a long healthy run dilutes later failures. See [How It Works](https://github.com/1mb-dev/autobreaker#how-it-works).

## Recommended Configurations

Starting points for adaptive mode. Tune them against what `Metrics()` shows in production.

| Dependency | `FailureRateThreshold` | `Interval` | `Timeout` |
|------------|------------------------|------------|-----------|
| Internal service | 0.05 | 1m | 30s |
| External API | 0.10 | 1m | 60s |
| Database | 0.03 | 30s | 10s |
| Network storage | 0.05 | 1m | 15s |

```go
breaker := autobreaker.New(autobreaker.Settings{
    Name:                 "payments-api",
    Timeout:              60 * time.Second,
    AdaptiveThreshold:    true,
    Interval:             time.Minute,
    FailureRateThreshold: 0.10,
    MinimumObservations:  20,
})
```

A lower threshold trips sooner and on smaller blips; a higher one tolerates more failures before protecting the dependency. A shorter `Timeout` probes the dependency sooner; a longer one gives it more time to recover before traffic returns.

## Sizing the Window

One `Interval` must see at least `MinimumObservations` requests, or the rate is never checked:

```
requests per window = request rate × Interval  ≥  MinimumObservations
```

| Request rate | `Interval` | Requests per window | `MinimumObservations` |
|--------------|------------|---------------------|-----------------------|
| 100 RPS | 10s | 1,000 | 20 (default) |
| 2 RPS | 1m | 120 | 20 (default) |
| 5 per minute | 2m | 10 | 5 |

Below a few requests per minute, keep the static default.

A larger `MinimumObservations` filters noise but delays the first check in each window; a smaller one reacts to fewer requests and trips on smaller samples.

## Migrating from sony/gobreaker

AutoBreaker follows the gobreaker v1 API shape, and with the default `Settings` it uses the same trip rule: `ConsecutiveFailures > 5`. Two differences to check:

- `MaxRequests` caps concurrent HalfOpen probes, and the first successful probe closes the breaker. gobreaker needs `MaxRequests` consecutive successes.
- The constructor is `New`, and the numeric values of `StateOpen` and `StateHalfOpen` are swapped relative to gobreaker. Compare states by name, not by number.

To move to rate-based tripping, replace your trip rule with:

```go
// Trip when more than 5% of the requests in a 1-minute window fail,
// once the window holds at least 20 requests
AdaptiveThreshold:    true,
Interval:             time.Minute,
FailureRateThreshold: 0.05,
MinimumObservations:  20,
```

Run it beside the old breaker first and compare when each would have tripped. See the [Migration Guide](/migration/) for the full mapping.

## Changing Settings Later

`UpdateSettings` changes `MaxRequests`, `Interval`, `Timeout`, `FailureRateThreshold` and `MinimumObservations` on a live breaker, so you can tune thresholds without a restart. It cannot switch `AdaptiveThreshold` on or off; build a new breaker for that.
