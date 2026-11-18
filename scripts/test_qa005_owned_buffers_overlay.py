"""Strict anchored patch checks for QA-005 owned-buffer overlay.

No source tree mutation and no claim of qualified attributable memory.
"""
import importlib.util
import pathlib
import tempfile
import unittest

SCRIPT = pathlib.Path(__file__).with_name("qa005_owned_buffers_overlay.py")
spec = importlib.util.spec_from_file_location("qa005_owned_buffers_overlay", SCRIPT)
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)


class OwnedBuffersOverlayTest(unittest.TestCase):
    def test_fails_closed_on_wrong_function_or_missing_anchor(self):
        with self.assertRaisesRegex(ValueError, "outbound frame identity"):
            mod.patch_internal("package neta\n")
        with self.assertRaisesRegex(ValueError, "inbound application charge"):
            mod.patch_charges("package neta\n")
        with self.assertRaisesRegex(ValueError, "inbound body allocation"):
            mod.patch_transport("package neta\n")
        with self.assertRaisesRegex(ValueError, "existing sampler marker"):
            mod.patch_benchmark("type qa005ChargeProbe struct {\n")

    def test_allocation_uses_capacity_and_release_is_once(self):
        internal = (
            "type outboundFrame struct {\n\tafterWrite         func()\n}\n"
            "type Session struct {\n\tactiveWrite               *outboundFrame\n}\n"
            "func (s *Session) enqueueControlLocked(item *outboundFrame) error {\n"
            "\ts.queuedControlBytes = next\n\tif item.barrier {\n"
            "}\nfunc (s *Session) enqueueApplicationLocked(item *outboundFrame) error {\n"
            "\ts.appQueue = append(s.appQueue, item)\n\ts.cond.Signal()\n}\n"
            "func (s *Session) releaseFrameChargeLocked(item *outboundFrame) {\n"
            "\tif item == nil {\n\t\treturn\n\t}\n\tif item.control {\n}\n"
        )
        patched = mod.patch_internal(internal)
        self.assertEqual(patched.count("item.qa005Observed = true"), 2)
        self.assertIn("uint64(cap(item.bytes))", patched)
        self.assertIn("qa005OwnedOutgoingApp", patched)
        self.assertIn("qa005OwnedPeak", patched)
        self.assertIn("qa005OwnedTotal", patched)
        self.assertIn("qa005OwnedIncomingReply", patched)
        self.assertIn("item.qa005Observed = false", patched)
        self.assertNotIn("retained-attributed-B", patched)

    def test_charge_release_after_callback_and_allocation_after_make(self):
        charges = (
            "type applicationReadCharge struct {\n\tbytes            uint64\n\tqueueReleased\n}\n"
            "type completionReadCharge struct {\n\tbytes    uint64\n\treleased bool\n}\n"
            "func app() {\n\tc.retainedReleased = true\n"
            "\tif next, ok := checkedSubUint64(\n}\n"
            "func completion() {\n\tc.released = true\n"
            "\tif next, ok := checkedSubUint64(\n}\n"
        )
        patched = mod.patch_charges(charges)
        self.assertIn("qa005SubtractOwnedLocked(s, &s.qa005OwnedIncomingApp", patched)
        self.assertIn("qa005SubtractOwnedLocked(s, &s.qa005OwnedIncomingReply", patched)
        transport = (
            "func read() {\n\t\tbody = make([]byte, bodyLength)\n"
            "\t\tif _, err := io.ReadFull(s.conn, body); err != nil {\n}\n"
        )
        patched = mod.patch_transport(transport)
        self.assertLess(patched.find("qa005AddOwnedLocked("), patched.find("io.ReadFull("))

    def test_refuses_output_inside_checkout_before_writing(self):
        with tempfile.TemporaryDirectory() as root:
            root = pathlib.Path(root)
            with self.assertRaisesRegex(ValueError, "outside source checkout"):
                mod.prepare(root, root / "inside")
            self.assertFalse((root / "inside").exists())


    def test_actual_source_outbound_build_to_queue_transfer(self):
        source = (SCRIPT.parent.parent / "session.go").read_text()
        patched = mod.patch_session(source)
        self.assertEqual(patched.count("qa005Transferred := false"), 3)
        self.assertEqual(patched.count("qa005Transferred = true"), 3)
        self.assertEqual(patched.count("qa005Observed: true"), 3)
        self.assertEqual(patched.count("qa005SubtractOwnedLocked(s, &s.qa005OwnedOutgoingApp"), 3)
        self.assertIn("frame = make([]byte, length)", patched)
        self.assertIn("qa005AddOwnedLocked(owner, &owner.qa005OwnedOutgoingApp", patched)
        self.assertIn("payload, s)", patched)
        self.assertEqual(source.count("qa005Transferred"), 0)
        with self.assertRaisesRegex(ValueError, "frame builder optional observer"):
            mod.patch_session(patched)


if __name__ == "__main__":
    unittest.main()
