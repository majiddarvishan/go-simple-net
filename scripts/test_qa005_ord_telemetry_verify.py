"""Synthetic fail-closed tests for process and host telemetry provenance."""
from copy import deepcopy
import sys
from pathlib import Path
import unittest

HERE = Path(__file__).parent
sys.path.insert(0, str(HERE))

from qa005_ord_telemetry_verify import verify_process_trace


SENSORS = {
    "loadavg": "0.00 0.00 0.00 1/1 1",
    "cpu_psi": "some avg10=0.00 avg60=0.00 avg300=0.00 total=0",
    "memory_psi": "some avg10=0.00 avg60=0.00 avg300=0.00 total=0",
    "io_psi": "some avg10=0.00 avg60=0.00 avg300=0.00 total=0",
    "cpu_stat": "usage_usec 1",
    "cpu_max": "max 100000",
    "cpuset": "0-3",
    "memory_events": "low 0",
    "swap_cgroup": "0",
    "boot_id": "synthetic-boot-id",
    "vmstat": {"pswpin": "0", "pswpout": "0", "pgmajfault": "0"},
    "governors": {"/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor": "performance"},
    "frequencies_khz": {"/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq": "2000000"},
    "thermal_millidegrees": {"/sys/class/thermal/thermal_zone0/temp": "42000"},
}


def clock(monotonic, utc):
    return {"monotonic_ns": monotonic, "utc_ns": utc}


def valid_evidence():
    samples = [
        {
            "kind": "before_spawn",
            "sample_start": clock(10, 910),
            "sample_end": clock(20, 920),
            "sensors": deepcopy(SENSORS),
            "missing_sensors": [],
        },
        {
            "kind": "after_spawn",
            "sample_start": clock(210, 1110),
            "sample_end": clock(220, 1120),
            "sensors": deepcopy(SENSORS),
            "missing_sensors": [],
        },
        {
            "kind": "after_exit",
            "sample_start": clock(510, 1410),
            "sample_end": clock(520, 1420),
            "sensors": deepcopy(SENSORS),
            "missing_sensors": [],
        },
    ]
    record = {
        "complete": True,
        "exit_code": 0,
        "sample_count": 3,
        "observed_max_sample_gap_ns": 300,
        "missing_sensors": [],
        "start_before_spawn": clock(100, 1000),
        "start_after_spawn": clock(200, 1100),
        "exit_after_wait": clock(500, 1400),
    }
    return record, samples


class TelemetryFailClosedTests(unittest.TestCase):
    def test_valid_process_trace_is_accepted_with_exact_clocks(self):
        record, samples = valid_evidence()
        result = verify_process_trace(record, samples)
        self.assertEqual(result["start_monotonic_ns"], 100)
        self.assertEqual(result["launch_monotonic_ns"], 200)
        self.assertEqual(result["end_monotonic_ns"], 500)
        self.assertEqual(result["boot_id"], "synthetic-boot-id")
        self.assertEqual(result["missing_sensors"], [])

    def test_boolean_exit_code_and_boolean_clock_are_rejected(self):
        record, samples = valid_evidence()
        record["exit_code"] = False
        with self.assertRaisesRegex(ValueError, "exact integer zero"):
            verify_process_trace(record, samples)
        record, samples = valid_evidence()
        samples[0]["sample_start"]["monotonic_ns"] = True
        with self.assertRaisesRegex(ValueError, "clock integer"):
            verify_process_trace(record, samples)

    def test_boolean_or_float_sample_summaries_are_rejected(self):
        for field, value in (
            ("sample_count", 3.0),
            ("sample_count", True),
            ("observed_max_sample_gap_ns", True),
        ):
            record, samples = valid_evidence()
            record[field] = value
            with self.assertRaisesRegex(ValueError, "sample count or maximum"):
                verify_process_trace(record, samples)

    def test_record_missing_sensor_summary_must_be_sorted_unique_strings(self):
        for value in ("", ["cpu_psi", "cpu_psi"], [True]):
            record, samples = valid_evidence()
            record["missing_sensors"] = value
            with self.assertRaisesRegex(ValueError, "process-level missing sensor"):
                verify_process_trace(record, samples)

    def test_sample_count_and_snapshot_kind_sequence_are_exact(self):
        record, samples = valid_evidence()
        samples[1]["kind"] = "after_exit"
        with self.assertRaisesRegex(ValueError, "snapshot kind"):
            verify_process_trace(record, samples)
        record, samples = valid_evidence()
        record["sample_count"] = 4
        with self.assertRaisesRegex(ValueError, "Sample count differs"):
            verify_process_trace(record, samples)

    def test_overlapping_or_regressing_snapshot_windows_are_rejected(self):
        record, samples = valid_evidence()
        samples[1]["sample_start"] = clock(19, 921)
        with self.assertRaisesRegex(ValueError, "Overlapping or regressing"):
            verify_process_trace(record, samples)
        record, samples = valid_evidence()
        samples[1]["sample_start"] = clock(210, 915)
        with self.assertRaisesRegex(ValueError, "Overlapping or regressing"):
            verify_process_trace(record, samples)

    def test_process_utc_regression_is_rejected(self):
        record, samples = valid_evidence()
        record["start_after_spawn"]["utc_ns"] = 999
        with self.assertRaisesRegex(ValueError, "process UTC chronology"):
            verify_process_trace(record, samples)

    def test_after_spawn_and_after_exit_snapshots_must_respect_process_bounds(self):
        record, samples = valid_evidence()
        samples[1]["sample_start"] = clock(199, 1100)
        with self.assertRaisesRegex(ValueError, "Post-spawn snapshot"):
            verify_process_trace(record, samples)
        record, samples = valid_evidence()
        samples[-1]["sample_start"] = clock(499, 1400)
        with self.assertRaisesRegex(ValueError, "Post-exit snapshot"):
            verify_process_trace(record, samples)

    def test_sensor_key_omission_and_payload_mismatch_fail_closed(self):
        record, samples = valid_evidence()
        samples[1]["sensors"].pop("cpu_psi")
        with self.assertRaisesRegex(ValueError, "sensor keys"):
            verify_process_trace(record, samples)
        record, samples = valid_evidence()
        samples[1]["missing_sensors"] = ["cpu_psi"]
        with self.assertRaisesRegex(ValueError, "absence declaration"):
            verify_process_trace(record, samples)

    def test_missing_scalar_sensor_is_accepted_only_when_declared_in_union(self):
        record, samples = valid_evidence()
        for sample in samples:
            sample["sensors"]["cpu_psi"] = None
            sample["missing_sensors"] = ["cpu_psi"]
        record["missing_sensors"] = ["cpu_psi"]
        result = verify_process_trace(record, samples)
        self.assertEqual(result["missing_sensors"], ["cpu_psi"])

    def test_partial_sensor_map_is_reported_as_missing(self):
        record, samples = valid_evidence()
        for sample in samples:
            sample["sensors"]["governors"]["/sys/devices/system/cpu/cpu1/cpufreq/scaling_governor"] = None
            sample["missing_sensors"] = ["governors"]
        record["missing_sensors"] = ["governors"]
        result = verify_process_trace(record, samples)
        self.assertEqual(result["missing_sensors"], ["governors"])

    def test_missing_boot_identity_is_rejected_even_when_declared(self):
        record, samples = valid_evidence()
        for sample in samples:
            sample["sensors"]["boot_id"] = None
            sample["missing_sensors"] = ["boot_id"]
        record["missing_sensors"] = ["boot_id"]
        with self.assertRaisesRegex(ValueError, "boot identity"):
            verify_process_trace(record, samples)

    def test_sensor_union_and_sampling_gap_must_match_raw_trace(self):
        record, samples = valid_evidence()
        record["missing_sensors"] = ["cpu_psi"]
        with self.assertRaisesRegex(ValueError, "summary differs"):
            verify_process_trace(record, samples)
        record, samples = valid_evidence()
        record["observed_max_sample_gap_ns"] = 299
        with self.assertRaisesRegex(ValueError, "Maximum sampling gap"):
            verify_process_trace(record, samples)

    def test_empty_or_malformed_sensor_maps_are_rejected(self):
        record, samples = valid_evidence()
        samples[0]["sensors"]["governors"] = {}
        with self.assertRaisesRegex(ValueError, "Malformed sensor map"):
            verify_process_trace(record, samples)
        record, samples = valid_evidence()
        samples[0]["sensors"]["frequencies_khz"] = {"/cpu0": ""}
        with self.assertRaisesRegex(ValueError, "Malformed sensor map entry"):
            verify_process_trace(record, samples)

    def test_boot_id_must_remain_constant_within_trace(self):
        record, samples = valid_evidence()
        samples[-1]["sensors"]["boot_id"] = "different-boot"
        with self.assertRaisesRegex(ValueError, "boot identity changed"):
            verify_process_trace(record, samples)


if __name__ == "__main__":
    unittest.main()
