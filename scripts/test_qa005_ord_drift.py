"""Fast synthetic tests: fixed Option-A formula only, never physical qualification."""
import importlib.util
import json
import math
from pathlib import Path
import sys
import tempfile
import unittest

HERE = Path(__file__).parent
sys.path.insert(0, str(HERE))
from qa005_ord_drift import analyze, fit_one, inverse3, MODE_COUNTS

ORDER = [1 if i % 2 == 0 else -1 for i in range(10)]
TIME = [-1 + 2 * i / 9 for i in range(10)]


class OrdDriftTests(unittest.TestCase):
    def test_zero_log_ratio_fits_original_mean_and_all_four_corners(self):
        for family in (5, 48, 144):
            model = fit_one([0.] * 10, ORDER, TIME, .075, family)
            self.assertEqual(model["fixed_bounds_result"], "WITHIN_FIXED_BOUNDS")
            self.assertEqual(model["joint_5m_family_size"], 5 * family)
            self.assertTrue(model["original_m_family_mean"]["inside_fixed_bound"])
            self.assertTrue(model["joint_5m_mean"]["inside_fixed_bound"])
            self.assertEqual(model["residual_df"], 7)
            self.assertEqual(len(model["joint_5m_corners"]), 4)
            self.assertEqual(
                {(v["first_side_order"], v["scaled_time"])
                 for v in model["joint_5m_corners"]},
                {(-1, -1.), (-1, 1.), (1, -1.), (1, 1.)},
            )

    def test_constant_ratio_outside_prospective_margin(self):
        model = fit_one([math.log(1.09)] * 10, ORDER, TIME, .05, 48)
        self.assertEqual(model["fixed_bounds_result"], "OUTSIDE_FIXED_BOUNDS")
        self.assertFalse(model["joint_5m_mean"]["inside_fixed_bound"])

    def test_balanced_mean_cannot_hide_order_bias(self):
        y = [0.07 if o == 1 else -0.07 for o in ORDER]
        model = fit_one(y, ORDER, TIME, .05, 5)
        self.assertAlmostEqual(model["original_m_family_mean"]["estimate_log"], 0.)
        self.assertEqual(model["fixed_bounds_result"], "OUTSIDE_FIXED_BOUNDS")
        self.assertTrue(any(not c["inside_fixed_bound"]
                            for c in model["joint_5m_corners"]))

    def test_balanced_mean_cannot_hide_time_drift(self):
        y = [t * .09 for t in TIME]
        model = fit_one(y, ORDER, TIME, .075, 5)
        self.assertAlmostEqual(model["original_m_family_mean"]["estimate_log"], 0.)
        self.assertTrue(model["original_m_family_mean"]["inside_fixed_bound"])
        self.assertEqual(model["fixed_bounds_result"], "OUTSIDE_FIXED_BOUNDS")

    def test_bonferroni_joint_mean_more_conservative_than_original(self):
        y = [.005 * t + (.004 if i % 2 else -.004)
             for i, t in enumerate(TIME)]
        for m in MODE_COUNTS.values():
            model = fit_one(y, ORDER, TIME, .05, m)
            original = model["original_m_family_mean"]
            joint = model["joint_5m_mean"]
            self.assertGreaterEqual(
                joint["ci_log"][1] - joint["ci_log"][0],
                original["ci_log"][1] - original["ci_log"][0],
            )
            self.assertAlmostEqual(
                model["alpha_two_sided_per_contrast"], .05 / (5 * m)
            )

    def test_zero_missing_nonfinite_and_rank_deficiency_fail_closed(self):
        for bad in ([0.] * 9, [float("nan")] * 10):
            with self.assertRaises(ValueError):
                fit_one(bad, ORDER, TIME, .05, 5)
        with self.assertRaises(ValueError):
            fit_one([0.] * 10, ORDER, [float(i) for i in ORDER], .05, 5)
        with self.assertRaises(ValueError):
            fit_one([0.] * 10, ORDER, TIME, .20, 6)
        with self.assertRaises(ValueError):
            inverse3([[1., 0, 0], [1., 0, 0], [0, 0, 1.]])

    def test_missing_or_substituted_evidence_never_claims_admission(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for mode in MODE_COUNTS:
                report = analyze(root, mode)
                self.assertEqual(report["qualification"], "DRIFT-INCONCLUSIVE")
                self.assertEqual(report["environment_validity"], "E-UNKNOWN")
                self.assertEqual(report["statistical_diagnostics"], "INCONCLUSIVE")
                self.assertEqual(report["scenarios"], {})
                self.assertTrue(report["reasons"])

    def test_process_midpoint_is_not_case_level_timestamp(self):
        with tempfile.TemporaryDirectory() as tmp:
            report = analyze(Path(tmp), "qa004")
            self.assertIn("PER_CASE_UNPROVEN", report["time_precision"])


if __name__ == "__main__":
    unittest.main()
