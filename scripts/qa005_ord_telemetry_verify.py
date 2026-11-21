"""Strict fail-closed validation of ORD-EVID process and host telemetry."""
from __future__ import annotations

import math
from qa005_ord_evidence import SENSORS


EXTRA_SENSOR_KEYS = {
    "vmstat", "governors", "frequencies_khz", "thermal_millidegrees",
}
EXPECTED_SENSOR_KEYS = set(SENSORS) | EXTRA_SENSOR_KEYS
SCALAR_SENSOR_KEYS = set(SENSORS) - {"boot_id"}
MAP_SENSOR_KEYS = {"governors", "frequencies_khz", "thermal_millidegrees"}
VMSTAT_KEYS = {"pswpin", "pswpout", "pgmajfault"}


def _clock(value, label):
    if type(value) is not dict or set(value) != {"utc_ns", "monotonic_ns"}:
        raise ValueError("Malformed clock record: " + label)
    monotonic = value.get("monotonic_ns")
    utc = value.get("utc_ns")
    if (type(monotonic) is not int or type(utc) is not int or
            monotonic <= 0 or utc <= 0):
        raise ValueError("Invalid clock integer: " + label)
    return monotonic, utc


def _sensor_payload(sample, label):
    sensors = sample.get("sensors")
    if type(sensors) is not dict or set(sensors) != EXPECTED_SENSOR_KEYS:
        raise ValueError("Missing or unexpected sensor keys: " + label)

    missing = []
    for key in SCALAR_SENSOR_KEYS:
        value = sensors[key]
        if value is None:
            missing.append(key)
        elif type(value) is not str or not value:
            raise ValueError("Malformed scalar sensor value: " + key)

    boot_id = sensors.get("boot_id")
    if type(boot_id) is not str or not boot_id:
        raise ValueError("Missing Linux boot identity: " + label)

    vmstat = sensors.get("vmstat")
    if vmstat is None:
        missing.append("vmstat")
    elif (type(vmstat) is not dict or set(vmstat) != VMSTAT_KEYS or
          any(type(value) is not str or not value for value in vmstat.values())):
        raise ValueError("Malformed vmstat sensor payload: " + label)

    for key in MAP_SENSOR_KEYS:
        value = sensors[key]
        if value is None:
            missing.append(key)
            continue
        if type(value) is not dict or not value:
            raise ValueError("Malformed sensor map: " + key)
        for path, reading in value.items():
            if (type(path) is not str or not path or
                    (reading is not None and
                     (type(reading) is not str or not reading))):
                raise ValueError("Malformed sensor map entry: " + key)
        if any(reading is None for reading in value.values()):
            missing.append(key)

    declared = sample.get("missing_sensors")
    expected = sorted(missing)
    if (type(declared) is not list or
            any(type(value) is not str for value in declared) or
            declared != sorted(set(declared)) or declared != expected):
        raise ValueError("Sensor absence declaration contradicts sensor payload: " + label)
    return boot_id, expected


def verify_process_trace(record, samples):
    """Validate exact process clocks, snapshot sequence, sensor coverage and summaries."""
    if type(record) is not dict:
        raise ValueError("Invalid process evidence record")
    if record.get("complete") is not True:
        raise ValueError("Process evidence is incomplete")
    if type(record.get("exit_code")) is not int or record["exit_code"] != 0:
        raise ValueError("Process exit code is not an exact integer zero")

    sample_count = record.get("sample_count")
    max_gap = record.get("observed_max_sample_gap_ns")
    if (type(sample_count) is not int or sample_count < 3 or
            type(max_gap) is not int or max_gap < 0):
        raise ValueError("Invalid sample count or maximum sampling gap")
    sampling_interval_ms = record.get("sampling_interval_ms")
    if (type(sampling_interval_ms) is not int or
            not 100 <= sampling_interval_ms <= 5000):
        raise ValueError("Invalid sampling interval metadata")
    if type(samples) is not list or len(samples) != sample_count:
        raise ValueError("Sample count differs from the raw telemetry trace")

    declared_union = record.get("missing_sensors")
    if (type(declared_union) is not list or
            any(type(value) is not str for value in declared_union) or
            declared_union != sorted(set(declared_union))):
        raise ValueError("Malformed process-level missing sensor summary")

    start_mono, start_utc = _clock(record.get("start_before_spawn"), "start_before_spawn")
    launch_mono, launch_utc = _clock(record.get("start_after_spawn"), "start_after_spawn")
    end_mono, end_utc = _clock(record.get("exit_after_wait"), "exit_after_wait")
    if not start_mono < launch_mono < end_mono:
        raise ValueError("Invalid process monotonic chronology")
    if not start_utc <= launch_utc <= end_utc:
        raise ValueError("Invalid process UTC chronology")

    expected_kinds = ["before_spawn", "after_spawn"] + ["during"] * (
        sample_count - 3
    ) + ["after_exit"]
    observed_gaps = []
    union = set()
    boot_id = None
    previous_end_mono = None
    previous_end_utc = None

    for index, (sample, expected_kind) in enumerate(zip(samples, expected_kinds)):
        label = "snapshot %d" % index
        if type(sample) is not dict or sample.get("kind") != expected_kind:
            raise ValueError("Unexpected telemetry snapshot kind: " + label)
        t0, u0 = _clock(sample.get("sample_start"), label + " start")
        t1, u1 = _clock(sample.get("sample_end"), label + " end")
        if t0 > t1 or u0 > u1:
            raise ValueError("Reversed telemetry snapshot window: " + label)
        if (previous_end_mono is not None and
                (t0 < previous_end_mono or u0 < previous_end_utc)):
            raise ValueError("Overlapping or regressing telemetry snapshots: " + label)

        if expected_kind == "before_spawn":
            if t1 > start_mono or u1 > start_utc:
                raise ValueError("Pre-spawn snapshot does not precede process start")
        elif expected_kind == "after_spawn":
            if (t0 < launch_mono or t1 > end_mono or
                    u0 < launch_utc or u1 > end_utc):
                raise ValueError("Post-spawn snapshot outside process bracket")
        elif expected_kind == "during":
            if (t0 < launch_mono or t1 > end_mono or
                    u0 < launch_utc or u1 > end_utc):
                raise ValueError("During-process snapshot outside process bracket")
        elif expected_kind == "after_exit":
            if t0 < end_mono or u0 < end_utc:
                raise ValueError("Post-exit snapshot precedes process exit")

        current_boot_id, absent = _sensor_payload(sample, label)
        if boot_id is None:
            boot_id = current_boot_id
        elif current_boot_id != boot_id:
            raise ValueError("Linux boot identity changed within process trace")
        union.update(absent)

        if index:
            observed_gaps.append(t0 - samples[index - 1]["sample_start"]["monotonic_ns"])
        previous_end_mono, previous_end_utc = t1, u1

    if not observed_gaps or max(observed_gaps) != max_gap:
        raise ValueError("Maximum sampling gap inconsistent with raw telemetry trace")
    if sorted(union) != declared_union:
        raise ValueError("Process missing-sensor summary differs from snapshot union")

    ordered_gaps = sorted(observed_gaps)
    middle = len(ordered_gaps) // 2
    median_gap = (
        ordered_gaps[middle] if len(ordered_gaps) % 2 else
        (ordered_gaps[middle - 1] + ordered_gaps[middle]) / 2
    )
    p95_gap = ordered_gaps[math.ceil(0.95 * len(ordered_gaps)) - 1]

    return {
        "start_monotonic_ns": start_mono,
        "launch_monotonic_ns": launch_mono,
        "end_monotonic_ns": end_mono,
        "start_utc_ns": start_utc,
        "launch_utc_ns": launch_utc,
        "end_utc_ns": end_utc,
        "observed_max_sample_gap_ns": max_gap,
        "sampling_interval_ms": sampling_interval_ms,
        "sample_gap_count": len(ordered_gaps),
        "median_sample_gap_ns": median_gap,
        "p95_sample_gap_ns": p95_gap,
        "max_gap_to_interval_ratio": max_gap / (sampling_interval_ms * 1_000_000),
        "missing_sensors": declared_union,
        "boot_id": boot_id,
    }
