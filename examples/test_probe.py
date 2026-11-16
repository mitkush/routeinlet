import io
import json
import os
import unittest
from unittest.mock import patch

import probe


def event(content=None, reason=None):
    choice = {"delta": {"content": content}, "finish_reason": reason}
    return "data: " + json.dumps({"choices": [choice]}) + "\n\n"


class ProbeTest(unittest.TestCase):
    def run_turn(self, wire, arrivals=None):
        response = io.BytesIO(wire.encode())
        response.headers = {"Gateway-Request-Id": "fixture", "Gateway-Billing-Source": "unknown"}
        with patch.dict(os.environ, ROUTEINLET_KEY="test", ROUTEINLET_MODEL="test"), \
             patch("urllib.request.urlopen", return_value=response), \
             patch("time.monotonic", side_effect=arrivals or []):
            return probe.turn([], measure_stream=arrivals is not None)

    def test_finish_reasons(self):
        for reason, message in [("length", "truncated"), ("content_filter", "filtered"),
                                ("tool_calls", "unexpected finish_reason"), (None, "normal stop")]:
            with self.subTest(reason=reason), self.assertRaisesRegex(ValueError, message):
                self.run_turn(event("OK", reason) + "data: [DONE]\n\n")
        self.assertEqual(self.run_turn(event("OK", "stop") + "data: [DONE]\r\n\r\n"), "OK")

    def test_missing_terminal_boundary(self):
        with self.assertRaisesRegex(ValueError, "before"):
            self.run_turn(event("OK", "stop") + "data: [DONE]\n")

    def test_arrival_timing(self):
        fragment = "a" * 400
        wire = event(fragment) * 3 + event(reason="stop") + "data: [DONE]\n\n"
        with self.assertRaisesRegex(ValueError, "inconclusive"):
            self.run_turn(wire, [1, 1.001, 1.002])
        self.assertEqual(self.run_turn(wire, [1, 1.2, 1.4]), fragment * 3)
