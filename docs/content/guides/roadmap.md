---
title: "Roadmap"
weight: 6
---

# Roadmap

## Next: adaptive by default

Percentage-based tripping currently needs `AdaptiveThreshold: true` and a suitable `Interval`. The next release theme is making it work without that setup:

- **Default:** trip on failure rate by default, introduced without silently changing the behavior of existing v1 configurations.
- **Window:** a default `Interval`, or a sliding window in place of the fixed one. A fixed window resets on a boundary, so a burst of failures split across two windows may not trip.
- **Misconfiguration:** warn or reject when `FailureRateThreshold` or `MinimumObservations` is set without `AdaptiveThreshold`, since they are ignored today.

No dates are committed.

## Not scheduled

**HTTP/gRPC middleware package.** The [examples](https://github.com/1mb-dev/autobreaker/tree/main/examples) show HTTP client and server wrappers. A reusable `middleware` sub-package would follow only if those prove insufficient.

## Proposing changes

Open a [GitHub issue](https://github.com/1mb-dev/autobreaker/issues) with your scenario, the current workaround and the behavior you expect.

## Versioning

AutoBreaker follows [Semantic Versioning](https://semver.org/):

- **Major (X.0.0)**: Breaking API changes
- **Minor (1.X.0)**: New features, backward compatible
- **Patch (1.0.X)**: Bug fixes, documentation
