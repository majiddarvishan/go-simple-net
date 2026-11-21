"""Report-only sensitivity diagnostics for uncertain per-case benchmark time.

Line receipts are upper bounds, not case execution timestamps.  This module
sweeps all endpoint assignments of the ten pair-time intervals and reports how
the existing order/time fit changes.  Endpoint sampling is NOT a continuous
interval bound or a qualification proof; every result remains inconclusive.
"""
from __future__ import annotations

import math


PAIR_COUNT = 10
SIDES = ("baseline", "candidate")


def pair_case_time_intervals(runs, case):
    """Build ten paired-case midpoint intervals from both side receipt bounds.

    Each side interval is [parent pre-spawn snapshot, observer line receipt].
    The paired-case time is the arithmetic mean of the two side case times.
    Doubled nanoseconds preserve exact integer arithmetic for half-ns midpoints.
    """
    if not isinstance(case, str) or not case:
        raise ValueError("A benchmark case identity is required")
    by_pair = {}
    if not isinstance(runs, list) or len(runs) != PAIR_COUNT * len(SIDES):
        raise ValueError("Exactly 20 side runs are required for case-time sensitivity")

    for run in runs:
        if not isinstance(run, dict):
            raise ValueError("Invalid paired run record")
        pair = run.get("pair")
        side = run.get("side")
        if type(pair) is not int or pair not in range(1, PAIR_COUNT + 1):
            raise ValueError("Invalid pair identity in case-time evidence")
        if side not in SIDES:
            raise ValueError("Invalid side identity in case-time evidence")
        sides = by_pair.setdefault(pair, {})
        if side in sides:
            raise ValueError("Duplicate side in case-time evidence")
        observation = run.get("case_line_observation")
        if not isinstance(observation, dict):
            raise ValueError("Missing line-observation evidence")
        intervals = observation.get("case_observation_intervals")
        if not isinstance(intervals, dict) or case not in intervals:
            raise ValueError("Missing case receipt interval: " + case)
        item = intervals[case]
        if not isinstance(item, dict):
            raise ValueError("Malformed case receipt interval: " + case)
        lower = item.get("earliest_possible_case_ns")
        upper = item.get("latest_possible_case_ns")
        arrival = item.get("arrival_monotonic_ns")
        width = item.get("interval_width_ns")
        if (type(lower) is not int or type(upper) is not int or
                type(arrival) is not int or type(width) is not int or
                not 0 < lower <= upper or arrival != upper or
                width != upper - lower):
            raise ValueError("Contradictory or invalid case receipt interval: " + case)
        sides[side] = (lower, upper)

    if set(by_pair) != set(range(1, PAIR_COUNT + 1)):
        raise ValueError("Missing pair in case-time evidence")

    result = []
    for pair in range(1, PAIR_COUNT + 1):
        sides = by_pair[pair]
        if set(sides) != set(SIDES):
            raise ValueError("Missing baseline/candidate side for pair %02d" % pair)
        baseline = sides["baseline"]
        candidate = sides["candidate"]
        lower_twice = baseline[0] + candidate[0]
        upper_twice = baseline[1] + candidate[1]
        if lower_twice > upper_twice:
            raise ValueError("Reversed paired-case time interval")
        result.append({
            "pair": pair,
            "lower_pair_midpoint_twice_ns": lower_twice,
            "upper_pair_midpoint_twice_ns": upper_twice,
            "width_pair_midpoint_ns": (upper_twice - lower_twice) / 2,
        })

    for previous, current in zip(result, result[1:]):
        if (previous["upper_pair_midpoint_twice_ns"] >=
                current["lower_pair_midpoint_twice_ns"]):
            raise ValueError(
                "Paired-case time intervals overlap or contradict run chronology"
            )
    return result


def endpoint_sensitivity(y, order, intervals, d, family_size, fit_fn):
    """Sweep all 2^10 endpoint assignments through the frozen fit function.

    This is an explicit sensitivity diagnostic, not a mathematical bound over
    the continuous interval product.  Invalid endpoint fits are counted, never
    dropped silently, and every returned status is non-qualifying.
    """
    if (len(y) != PAIR_COUNT or len(order) != PAIR_COUNT or
            len(intervals) != PAIR_COUNT):
        raise ValueError("Ten paired values, orders and time intervals are required")
    if any(type(v) not in (int, float) or not math.isfinite(v) for v in y):
        raise ValueError("Nonfinite or invalid paired log ratio")
    if any(type(v) is not int or v not in (-1, +1) for v in order):
        raise ValueError("Invalid first-side order")
    if d not in (.05, .075) or family_size not in (5, 48, 144):
        raise ValueError("Unexpected approved margin or case-metric family")

    lower = []
    upper = []
    for index, item in enumerate(intervals):
        if not isinstance(item, dict) or item.get("pair") != index + 1:
            raise ValueError("Invalid or unordered pair-time interval")
        lo = item.get("lower_pair_midpoint_twice_ns")
        hi = item.get("upper_pair_midpoint_twice_ns")
        if type(lo) is not int or type(hi) is not int or not 0 < lo <= hi:
            raise ValueError("Invalid pair-time interval endpoints")
        lower.append(lo)
        upper.append(hi)
    if any(upper[i] >= lower[i + 1] for i in range(PAIR_COUNT - 1)):
        raise ValueError("Pair-time intervals are not strictly chronological")

    expected_combinations = 1 << PAIR_COUNT
    seen = set()
    slopes = []
    conditions = []
    corner_values = {}
    invalid_reasons = {}
    invalid_count = 0
    sampled_bounds = []

    for mask in range(expected_combinations):
        raw_times = tuple(
            upper[i] if mask & (1 << i) else lower[i]
            for i in range(PAIR_COUNT)
        )
        if raw_times in seen:
            continue
        seen.add(raw_times)
        span = raw_times[-1] - raw_times[0]
        if span <= 0:
            invalid_count += 1
            invalid_reasons["nonpositive normalized time span"] = (
                invalid_reasons.get("nonpositive normalized time span", 0) + 1
            )
            continue
        scaled = [2.0 * (value - raw_times[0]) / span - 1.0
                  for value in raw_times]
        try:
            fitted = fit_fn(y, order, scaled, d, family_size)
        except (ValueError, TypeError, ArithmeticError, OverflowError) as exc:
            reason = str(exc) or type(exc).__name__
            invalid_count += 1
            invalid_reasons[reason] = invalid_reasons.get(reason, 0) + 1
            continue

        coefficients = fitted.get("model_coefficients_mean_order_time")
        if (not isinstance(coefficients, list) or len(coefficients) != 3 or
                any(type(v) not in (int, float) or not math.isfinite(v)
                    for v in coefficients)):
            invalid_count += 1
            invalid_reasons["missing or invalid fitted coefficients"] = (
                invalid_reasons.get("missing or invalid fitted coefficients", 0) + 1
            )
            continue
        slopes.append(coefficients[2])
        condition = fitted.get("xtx_condition_inf")
        if type(condition) not in (int, float) or not math.isfinite(condition):
            invalid_count += 1
            invalid_reasons["missing or invalid design condition"] = (
                invalid_reasons.get("missing or invalid design condition", 0) + 1
            )
            continue
        conditions.append(condition)
        sampled_bounds.append(
            fitted.get("fixed_bounds_result") == "WITHIN_FIXED_BOUNDS"
        )
        for corner in fitted.get("joint_5m_corners", []):
            key = (corner.get("first_side_order"), corner.get("scaled_time"))
            if key not in ((-1, -1.0), (-1, 1.0), (1, -1.0), (1, 1.0)):
                invalid_count += 1
                invalid_reasons["unexpected modeled corner"] = (
                    invalid_reasons.get("unexpected modeled corner", 0) + 1
                )
                continue
            bucket = corner_values.setdefault(key, {
                "estimates": [], "ci_lows": [], "ci_highs": [],
                "inside": [],
            })
            ci = corner.get("ci_log")
            estimate = corner.get("estimate_log")
            if (not isinstance(ci, list) or len(ci) != 2 or
                    any(type(v) not in (int, float) or not math.isfinite(v)
                        for v in ci) or
                    type(estimate) not in (int, float) or
                    not math.isfinite(estimate)):
                invalid_count += 1
                invalid_reasons["missing or invalid corner interval"] = (
                    invalid_reasons.get("missing or invalid corner interval", 0) + 1
                )
                continue
            bucket["estimates"].append(estimate)
            bucket["ci_lows"].append(ci[0])
            bucket["ci_highs"].append(ci[1])
            bucket["inside"].append(corner.get("inside_fixed_bound") is True)

    unique_count = len(seen)
    valid_count = len(slopes)
    complete = (unique_count > 0 and invalid_count == 0 and
                valid_count == unique_count and
                set(corner_values) == {
                    (-1, -1.0), (-1, 1.0), (1, -1.0), (1, 1.0)
                } and all(len(v["inside"]) == valid_count
                          for v in corner_values.values()))
    all_within = (complete and bool(sampled_bounds) and
                  all(sampled_bounds) and
                  all(all(v["inside"]) for v in corner_values.values()))
    any_outside = (any(not value for value in sampled_bounds) or
                   any(any(not x for x in v["inside"])
                       for v in corner_values.values()))

    if not complete:
        status = "INCONCLUSIVE_INVALID_ENDPOINT_FIT"
    elif any_outside:
        status = "ENDPOINT_SWEEP_EXPOSES_OUT_OF_BOUND_ASSIGNMENT"
    else:
        status = "ENDPOINT_SWEEP_WITHIN_FIXED_BOUNDS_UNPROVEN"

    corner_envelope = {}
    for (first_order, scaled_time), values in sorted(corner_values.items()):
        if not values["estimates"]:
            continue
        corner_envelope[
            "order=%+d,time=%+d" % (first_order, int(scaled_time))
        ] = {
            "estimate_log_range": [
                min(values["estimates"]), max(values["estimates"])
            ],
            "ci_log_envelope": [
                min(values["ci_lows"]), max(values["ci_highs"])
            ],
            "inside_fixed_bound_all_sampled_assignments": (
                len(values["inside"]) == valid_count and
                all(values["inside"])
            ),
        }

    return {
        "method": (
            "all unique 2^10 pair-time interval endpoint assignments; each "
            "pair time is the mean of its baseline/candidate case-time values"
        ),
        "endpoint_combinations": expected_combinations,
        "unique_endpoint_time_vectors": unique_count,
        "valid_endpoint_fits": valid_count,
        "invalid_endpoint_fits": invalid_count,
        "invalid_endpoint_reasons": invalid_reasons,
        "time_coefficient_log_ratio_per_normalized_window_range": (
            [min(slopes), max(slopes)] if slopes else None
        ),
        "maximum_design_condition_inf": max(conditions) if conditions else None,
        "pair_time_interval_width_ns": [
            item["width_pair_midpoint_ns"] for item in intervals
        ],
        "pair_time_intervals_twice_ns": [
            [item["lower_pair_midpoint_twice_ns"],
             item["upper_pair_midpoint_twice_ns"]]
            for item in intervals
        ],
        "joint_5m_corner_ci_envelope": corner_envelope,
        "all_sampled_endpoint_fits_within_fixed_bounds": (
            all_within if complete else None
        ),
        "any_sampled_endpoint_fit_outside_fixed_bounds": any_outside,
        "continuous_interval_bound_proved": False,
        "case_time_precision_proved": False,
        "sensitivity_status": status,
        "qualification": "DRIFT-INCONCLUSIVE",
    }
