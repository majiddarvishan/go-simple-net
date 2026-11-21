"""Synthetic fail-closed tests for case-time uncertainty diagnostics."""
from pathlib import Path
import sys
import tempfile
import unittest

HERE = Path(__file__).parent
sys.path.insert(0, str(HERE))

from qa005_ord_drift import fit_one
from qa005_ord_time_uncertainty import (
    endpoint_sensitivity,
    pair_case_time_intervals,
)

ORDER = [1 if i % 2 == 0 else -1 for i in range(10)]
TIME = [-1 + 2 * i / 9 for i in range(10)]
CASE = "BenchmarkSyntheticCase"


def make_intervals(width_ns=200_000_000):
    intervals = []
    for index in range(10):
        lower = 2 * (1_000_000_000 + index * 1_000_000_000)
        upper = lower + 2 * width_ns
        intervals.append({
            "pair": index + 1,
            "lower_pair_midpoint_twice_ns": lower,
            "upper_pair_midpoint_twice_ns": upper,
            "width_pair_midpoint_ns": width_ns,
        })
    return intervals


def make_runs(width_ns=100_000):
    runs = []
    for pair in range(1, 11):
        for side_index, side in enumerate(("baseline", "candidate")):
            pre_spawn = (pair - 1) * 2_000_000_000 + side_index * 1_000_000_000 + 1_000_000_000
            arrival = pre_spawn + width_ns
            runs.append({
                "pair": pair,
                "side": side,
                "case_line_observation": {
                    "case_observation_intervals": {
                        CASE: {
                            "earliest_possible_case_ns": pre_spawn,
                            "latest_possible_case_ns": arrival,
                            "arrival_monotonic_ns": arrival,
                            "interval_width_ns": width_ns,
                        }
                    }
                },
            })
    return runs


class CaseTimeUncertaintyTests(unittest.TestCase):
    def test_pair_intervals_combine_both_side_bounds_exactly(self):
        intervals = pair_case_time_intervals(make_runs(), CASE)
        self.assertEqual(len(intervals), 10)
        self.assertEqual(
            intervals[0]["lower_pair_midpoint_twice_ns"],
            3_000_000_000,
        )
        self.assertEqual(
            intervals[0]["upper_pair_midpoint_twice_ns"],
            3_000_200_000,
        )
        self.assertEqual(intervals[0]["width_pair_midpoint_ns"], 100_000)

    def test_missing_duplicate_and_contradictory_side_evidence_fail_closed(self):
        with self.assertRaisesRegex(ValueError, "20 side runs"):
            pair_case_time_intervals(make_runs()[:-1], CASE)
        duplicate = make_runs()
        duplicate[-1] = dict(duplicate[0])
        with self.assertRaisesRegex(ValueError, "Duplicate side"):
            pair_case_time_intervals(duplicate, CASE)
        contradictory = make_runs()
        contradictory[0]["case_line_observation"]["case_observation_intervals"][CASE]["interval_width_ns"] = True
        with self.assertRaisesRegex(ValueError, "Contradictory"):
            pair_case_time_intervals(contradictory, CASE)

    def test_overlapping_pair_intervals_fail_closed(self):
        intervals = make_intervals()
        intervals[1]["lower_pair_midpoint_twice_ns"] = (
            intervals[0]["upper_pair_midpoint_twice_ns"] - 1
        )
        with self.assertRaisesRegex(ValueError, "not strictly chronological"):
            endpoint_sensitivity([0.0] * 10, ORDER, intervals, .05, 5, fit_one)

    def test_exact_case_times_produce_one_unique_time_vector(self):
        intervals = make_intervals(width_ns=0)
        result = endpoint_sensitivity(
            [0.0] * 10, ORDER, intervals, .075, 5, fit_one
        )
        self.assertEqual(result["endpoint_combinations"], 1024)
        self.assertEqual(result["unique_endpoint_time_vectors"], 1)
        self.assertEqual(result["valid_endpoint_fits"], 1)
        self.assertEqual(result["invalid_endpoint_fits"], 0)
        self.assertEqual(
            result["time_coefficient_log_ratio_per_normalized_window_range"],
            [0.0, 0.0],
        )
        self.assertFalse(result["continuous_interval_bound_proved"])
        self.assertFalse(result["case_time_precision_proved"])
        self.assertEqual(result["qualification"], "DRIFT-INCONCLUSIVE")

    def test_wide_case_time_intervals_change_fitted_time_slope(self):
        intervals = make_intervals(width_ns=200_000_000)
        result = endpoint_sensitivity(
            [0.015 * value for value in TIME],
            ORDER, intervals, .075, 5, fit_one
        )
        slope_range = result[
            "time_coefficient_log_ratio_per_normalized_window_range"
        ]
        self.assertIsNotNone(slope_range)
        self.assertGreater(slope_range[1] - slope_range[0], 0.0005)
        self.assertEqual(result["unique_endpoint_time_vectors"], 1024)
        self.assertEqual(result["valid_endpoint_fits"], 1024)
        self.assertEqual(result["invalid_endpoint_fits"], 0)
        self.assertFalse(result["continuous_interval_bound_proved"])
        self.assertEqual(result["qualification"], "DRIFT-INCONCLUSIVE")

    def test_nonfinite_or_boolean_inputs_fail_closed(self):
        intervals = make_intervals()
        with self.assertRaisesRegex(ValueError, "Nonfinite"):
            endpoint_sensitivity([float("nan")] + [0.0] * 9,
                                 ORDER, intervals, .05, 5, fit_one)
        with self.assertRaisesRegex(ValueError, "first-side order"):
            endpoint_sensitivity([0.0] * 10, [True] + ORDER[1:],
                                 intervals, .05, 5, fit_one)
        intervals[0]["lower_pair_midpoint_twice_ns"] = True
        with self.assertRaisesRegex(ValueError, "endpoints"):
            endpoint_sensitivity([0.0] * 10, ORDER, intervals, .05, 5, fit_one)


if __name__ == "__main__":
    unittest.main()
