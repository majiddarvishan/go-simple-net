# ORD-EVID continuous case-time interval enclosure candidate (2026-11-21)

**Status:** report-only mathematical enclosure candidate. It is not yet marked as a proved production-fit bound pending independent equivalence review. `continuous_interval_bound_proved=false`, `case_time_precision_proved=false`, and all parent reports remain `DRIFT-INCONCLUSIVE`.

## Why endpoint sampling is insufficient

The endpoint sweep in [ORD-EVID time uncertainty preparation](NETA_GO_ORD_TIME_UNCERTAINTY_PREP_2026-11-21.md) evaluates every unique one of the (2^{10}) endpoint assignments, but endpoint samples alone do not bound interior assignments of the ten paired-case time intervals.

## Candidate continuous enclosure

`scripts/qa005_ord_time_interval_bound.py` constructs a conservative independent box for each normalized case-time coordinate.

For paired-case midpoint (x_i), with (x_i in [l_i,u_i]), normalization is:

[
t_i = 2\frac{x_i-x_1}{x_{10}-x_1}-1
]

The ten paired intervals are strictly chronological. For interior cases, monotonicity of this expression gives an enclosing coordinate interval by selecting the appropriate extrema of (x_i), (x_1), and (x_{10}). The first and last normalized coordinates are exactly -1 and +1. The independent coordinate box may include infeasible combinations, which widens rather than narrows the result.

For the approved balanced 5/5 order vector, the regression has an orthogonal intercept/order block. The implementation uses the equivalent closed forms for:

- residualized time sum of squares and time/order covariance;
- time, intercept and order coefficient intervals;
- residual sum-of-squares upper bound;
- leverage for each of the four ((order,time)) corners; and
- the four joint 5m corner-CI envelopes.

Every interval arithmetic operation is outward-rounded with `math.nextafter`. If the lower bound of residualized time sum of squares is nonpositive, the helper returns an explicit inconclusive result rather than attempting division or silently dropping the assignment.

## Current report fields

Each case-metric now has `case_time_continuous_enclosure` with:
- `continuous_interval_enclosure_complete`;
- `continuous_interval_bound_proved=false` pending independent review;
- normalized per-case time intervals;
- residualized time sum-of-squares interval;
- time-coefficient interval;
- residual sum-of-squares upper bound;
- four joint 5m corner-CI envelopes; and
- whether every corner envelope remains inside the existing fixed margin.

The report-level statistical diagnostic becomes INCONCLUSIVE if any enclosure cannot establish a positive denominator, and OUTSIDE-FIXED-BOUNDS-UNQUALIFIED if any completed enclosure exposes an out-of-bound possibility. Even a complete in-bound enclosure is not a Qualification PASS.

## Validation and limitations

Synthetic tests compare the enclosure against all 1024 endpoint fits and 256 deterministic interior assignments, including the time coefficient and all four corner CIs. Additional cases cover exact times, a near-collinear order pattern, out-of-bound values, invalid order, malformed intervals and nonfinite inputs.

The implementation is intentionally labelled a candidate because the independent algebraic enclosure has not yet been reviewed for equivalence to every floating-point operation in production `fit_one`. The tests provide strong empirical coverage but are not a formal proof of floating-point implementation error. The line receipt still bounds a possible case time; it is not the actual case start/end. Host E-VALID, observer overhead, A/A parity, nonlinear drift and independent acceptance remain open.

No heavy benchmark, stress, soak or extended fuzz campaign is run. No baseline/SLA, Performance Gate, public API, wire contract, v1.0.0 source or tag is changed.
