import importlib.util
import pathlib
import subprocess
import tempfile
import unittest
import os

SCRIPT = pathlib.Path(__file__).with_name('qa005_memory_overlay.py')
spec = importlib.util.spec_from_file_location('qa005_memory_overlay', SCRIPT)
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)


class OverlayTest(unittest.TestCase):
    def test_rewrites_functions_without_touching_source(self):
        fixture = '''package neta
import ("os"; "testing"; "time")
type ownerResourceUsage struct { retainedApplicationBytes uint64; retainedCompletionBytes uint64 }
type ownerResourceLedger struct {}
func (*ownerResourceLedger) snapshot() ownerResourceUsage { return ownerResourceUsage{10, 5} }
func checkedAddUint64(a, b uint64) (uint64, bool) { return a+b, true }
type Server struct { resources *ownerResourceLedger }
type Client struct { resources *ownerResourceLedger }
type benchmarkEnv struct { server *Server; clients []*Client }
func benchmarkRunRequestResponse(b *testing.B, env *benchmarkEnv) {
 b.ResetTimer()
 b.StopTimer()
}
func benchmarkRunSendNoWait(b *testing.B, env *benchmarkEnv) {
 b.ResetTimer()
 b.StopTimer()
}
func TestNoop(t *testing.T) {}
func BenchmarkCheck(b *testing.B) {
    env := &benchmarkEnv{server:&Server{resources:&ownerResourceLedger{}}}
    benchmarkRunRequestResponse(b,env)
}
'''
        fixture = fixture.replace(' b.ResetTimer()', '\tb.ResetTimer()').replace(' b.StopTimer()', '\tb.StopTimer()')
        with tempfile.TemporaryDirectory() as d:
            root = pathlib.Path(d) / 'repo'
            root.mkdir()
            (root / 'go.mod').write_text('module example.com/overlay\n\ngo 1.23\n')
            (root / 'benchmark_test.go').write_text(fixture)
            (root / 'qa005_qualification_benchmark_test.go').write_text('package neta\n')
            original = (root / 'benchmark_test.go').read_bytes()
            out = pathlib.Path(d) / 'adapter'
            metadata = mod.prepare(root, out)
            self.assertFalse(metadata['measurement_qualifies_retained_attributed_memory'])
            self.assertEqual(original, (root / 'benchmark_test.go').read_bytes())
            replacement = (out / 'benchmark_test.go').read_text()
            self.assertEqual(replacement.count('probe := qa005StartChargeProbe(env)'), 2)
            self.assertIn('b.ResetTimer()\n\tprobe := qa005StartChargeProbe(env)', replacement)
            self.assertNotIn('probe := qa005StartChargeProbe(env)\n\tb.ResetTimer()', replacement)
            self.assertNotIn('b.ReportMetric(float64(p.peak), "retained-attributed-B")', replacement)
            # The toy fixture has otherwise-unused imports; disable vet only for this synthetic check.
            result = subprocess.run(['go', 'test', '-vet=off', '-overlay', str(out / 'overlay.json'), '-run', '^TestNoop$', './...'],
                                    cwd=root, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            bench = subprocess.run(['go', 'test', '-vet=off', '-overlay', str(out / 'overlay.json'),
                                    '-run', '^$', '-bench', '^BenchmarkCheck$', '-benchtime=1x'],
                                   cwd=root, capture_output=True, text=True,
                                   env={**os.environ, 'NETA_QA005_MEMORY_PROBE': '1'})
            self.assertEqual(bench.returncode, 0, bench.stdout + bench.stderr)
            self.assertIn('probe-owner-charge-peak-B', bench.stdout)
            self.assertNotIn('retained-attributed-B', bench.stdout)
            with self.assertRaises(FileExistsError):
                mod.prepare(root, out)

    def test_rejects_modified_timer_fixture_and_inside_repo(self):
        fixture = 'func benchmarkRunRequestResponse(){\n\tb.ResetTimer()\n\tb.StopTimer()\n}\nfunc benchmarkRunSendNoWait(){\n\tb.StopTimer()\n}\nfunc TestX(){}\n'
        with self.assertRaisesRegex(ValueError, 'unexpected benchmark timer contract'):
            mod.patch_benchmark(fixture)
        with tempfile.TemporaryDirectory() as d:
            root = pathlib.Path(d)
            (root/'benchmark_test.go').write_text('')
            (root/'qa005_qualification_benchmark_test.go').write_text('')
            with self.assertRaisesRegex(ValueError, 'outside'):
                mod.prepare(root, root / 'inside')


if __name__ == '__main__':
    unittest.main()
