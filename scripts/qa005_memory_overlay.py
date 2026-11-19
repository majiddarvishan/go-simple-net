#!/usr/bin/env python3
"""Generate an exploratory QA-005 ledger-charge sampler as an identical go test overlay.

This deliberately does NOT emit retained-attributed-B: the existing owner charge
includes pre-allocation reservations and excludes library metadata.
"""
import argparse
import hashlib
import json
from pathlib import Path

ADAPTER_ID = 'QA005-CHARGE-OVERLAY-PROBE-V1-UNQUALIFIED'
FIXTURES = ('qa005_qualification_benchmark_test.go', 'benchmark_test.go')
START = '\tb.ResetTimer()\n'
STOP = '\tb.StopTimer()\n'
FUNCTIONS = ('func benchmarkRunRequestResponse(', 'func benchmarkRunSendNoWait(')

HELPER = '''
// QA005 exploratory diagnostic only: this is policy-charge sampling, NOT attributable heap.
type qa005ChargeProbe struct {
    stop chan struct{}
    done chan struct{}
    peak uint64
    samples uint64
    invalid bool
}

func qa005StartChargeProbe(env *benchmarkEnv) *qa005ChargeProbe {
    if os.Getenv("NETA_QA005_MEMORY_PROBE") != "1" { return nil }
    p := &qa005ChargeProbe{stop: make(chan struct{}), done: make(chan struct{})}
    observe := func() {
        var total uint64
        accumulate := func(owner *ownerResourceLedger) {
            if p.invalid { return }
            usage := owner.snapshot()
            next, ok := checkedAddUint64(total, usage.retainedApplicationBytes)
            if !ok { p.invalid = true; return }
            total, ok = checkedAddUint64(next, usage.retainedCompletionBytes)
            if !ok { p.invalid = true; return }
        }
        accumulate(env.server.resources)
        for _, client := range env.clients { accumulate(client.resources) }
        if total > p.peak { p.peak = total }
        p.samples++
    }
    go func() {
        defer close(p.done)
        ticker := time.NewTicker(5 * time.Millisecond)
        defer ticker.Stop()
        observe()
        for {
            select {
            case <-p.stop: observe(); return
            case <-ticker.C: observe()
            }
        }
    }()
    return p
}

func (p *qa005ChargeProbe) qa005ReportChargeProbe(b *testing.B) {
    if p == nil { return }
    close(p.stop)
    <-p.done
    if p.invalid || p.samples == 0 {
        b.Fatal("QA005 exploratory charge sampler invalid")
    }
    b.ReportMetric(float64(p.peak), "probe-owner-charge-peak-B")
    b.ReportMetric(float64(p.samples), "probe-owner-charge-samples")
    // Never report retained-attributed-B here: these are reserved full-frame policy bytes.
}
'''


def digest(contents):
    h = hashlib.sha256()
    for data in contents:
        h.update(data)
    return h.hexdigest()


def patch_benchmark(source):
    if 'type qa005ChargeProbe struct {' in source:
        raise ValueError('existing sampler marker; refuse duplicate instrumentation')
    for pos, function in enumerate(FUNCTIONS):
        start = source.find(function)
        if start < 0 or source.find(function, start + len(function)) >= 0:
            raise ValueError('missing or duplicate fixture function: ' + function)
        end = source.find(FUNCTIONS[pos + 1], start + len(function)) if pos + 1 < len(FUNCTIONS) else source.find('\nfunc ', start + len(function))
        if end < 0:
            raise ValueError('missing function boundary: ' + function)
        segment = source[start:end]
        if segment.count(START) != 1 or segment.count(STOP) != 1:
            raise ValueError('unexpected benchmark timer contract: ' + function)
        segment = segment.replace(START, START + '\tprobe := qa005StartChargeProbe(env)\n', 1)
        segment = segment.replace(STOP, STOP + '\tprobe.qa005ReportChargeProbe(b)\n', 1)
        source = source[:start] + segment + source[end:]
    return source.rstrip() + '\n' + HELPER


def prepare(source, output):
    source = source.resolve(strict=True)
    output = output.resolve()
    if source == output or source in output.parents:
        raise ValueError('overlay output must be outside the source checkout')
    original = [(source / name).read_bytes() for name in FIXTURES]
    patched = patch_benchmark(original[1].decode('utf-8')).encode('utf-8')
    # No writing to checkout. The effective digest is identical for the same fixtures.
    output.mkdir(parents=True, exist_ok=False)
    replacement = output / 'benchmark_test.go'
    replacement.write_bytes(patched)
    overlay = {'Replace': {str(source / FIXTURES[1]): str(replacement)}}
    (output / 'overlay.json').write_text(json.dumps(overlay, sort_keys=True) + '\n')
    manifest = {
        'adapter_id': ADAPTER_ID,
        'source_fixture_sha256': digest(original),
        'effective_fixture_sha256': digest([original[0], patched]),
        'effective_benchmark_sha256': hashlib.sha256(patched).hexdigest(),
        'measurement_qualifies_retained_attributed_memory': False,
        'metric': 'probe-owner-charge-peak-B',
        'source_tree_mutated': False,
    }
    (output / 'manifest.json').write_text(json.dumps(manifest, sort_keys=True, indent=2) + '\n')
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    print(json.dumps(prepare(args.source, args.output), sort_keys=True))


if __name__ == '__main__':
    main()
