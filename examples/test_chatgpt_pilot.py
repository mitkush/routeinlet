"""Offline lifecycle integration tests: real gateway, local fake connector."""
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
import unittest

ROOT = Path(__file__).resolve().parent.parent


class PilotLifecycleTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.build = tempfile.TemporaryDirectory(prefix="routeinlet-build-")
        cls.binary = Path(cls.build.name) / "routeinlet"
        subprocess.run(["go", "build", "-o", str(cls.binary), "./cmd/routeinlet"], cwd=ROOT, check=True)

    @classmethod
    def tearDownClass(cls):
        cls.build.cleanup()

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="ri-", dir="/tmp")
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.repo = self.root / "repo"
        (self.repo / "examples").mkdir(parents=True)
        for name in ("chatgpt_pilot.py", "chatgpt-connector.sh"):
            shutil.copy(ROOT / "examples" / name, self.repo / "examples" / name)
        shutil.copy(self.binary, self.repo / "routeinlet")
        self.profile = self.root / "profile"
        self.connector = self.root / "connector"
        self.connector.write_text("#!" + sys.executable + "\n" + '''
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
import sys
root = Path(__file__).parent
if (root / 'fail').exists():
    raise SystemExit(7)
class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.headers.get('Authorization') != 'Bearer connector-test':
            self.send_error(401)
            return
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'{"data": [{"id": "pilot"}]}')
    def do_POST(self):
        (root / 'inference-attempt').touch()
        self.send_error(500)
    def log_message(self, *args):
        pass
HTTPServer(('127.0.0.1', 8318), Handler).serve_forever()
''')
        self.connector.chmod(0o700)
        self.env = dict(os.environ, CONNECTOR_BIN=str(self.connector), PILOT_DIR=str(self.profile),
                        ROUTEINLET_CONNECTOR="http://127.0.0.1:8318", ROUTEINLET_MODEL="pilot",
                        ROUTEINLET_KEY="gateway-test", ROUTEINLET_CONNECTOR_KEY="connector-test")
        subprocess.run([str(self.repo / "routeinlet"), "init-chatgpt-pilot", str(self.profile)], env=self.env, check=True)
        self.addCleanup(lambda: self.command("stop", check=False))

    def command(self, action, env=None, check=True):
        result = subprocess.run([sys.executable, str(self.repo / "examples/chatgpt_pilot.py"), action,
                                 "--directory", str(self.profile)], env=env or self.env,
                                capture_output=True, text=True, timeout=55)
        if check:
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result

    def wait_stopped(self):
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline:
            result = self.command("status", check=False)
            if result.returncode == 0 and result.stdout.startswith("Stopped"):
                return
            time.sleep(0.1)
        self.fail("supervisor did not shut down")

    def assert_ports_free(self):
        for port in (8318, 8787):
            with socket.socket() as listener:
                listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
                listener.bind(("127.0.0.1", port))

    def test_start_status_duplicate_and_stop_without_keys(self):
        first = json.loads(self.command("start").stdout)
        again = self.command("start")
        self.assertIn("Already running", again.stdout)
        status = json.loads(self.command("status").stdout)
        self.assertEqual(status["gateway_pid"], first["gateway_pid"])
        self.assertEqual(status["connector_pid"], first["connector_pid"])
        self.assertEqual(status["model"], "pilot")
        self.assertFalse((self.root / "inference-attempt").exists())
        for path in (self.profile / "run").iterdir():
            self.assertEqual(path.stat().st_mode & 0o077, 0, path)
            if path.is_file():
                self.assertNotIn("gateway-test", path.read_text())
                self.assertNotIn("connector-test", path.read_text())
        clean = {k: v for k, v in self.env.items() if not k.startswith("ROUTEINLET_") and k != "CONNECTOR_BIN"}
        self.command("status", env=clean)
        self.command("stop", env=clean)
        self.command("stop", env=clean)
        self.assert_ports_free()

    def test_changed_model_or_key_requires_explicit_restart(self):
        self.command("start")
        for name in ("ROUTEINLET_MODEL", "ROUTEINLET_KEY", "ROUTEINLET_CONNECTOR_KEY"):
            result = self.command("start", env=dict(self.env, **{name: "different"}), check=False)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("different settings", result.stderr)
        self.assertEqual(json.loads(self.command("status").stdout)["model"], "pilot")
        self.command("stop")
        changed = dict(self.env, ROUTEINLET_MODEL="new-model")
        self.assertEqual(json.loads(self.command("start", env=changed).stdout)["model"], "new-model")

    def test_occupied_port_is_left_alone(self):
        for port in (8318, 8787):
            with self.subTest(port=port), socket.socket() as listener:
                listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
                listener.bind(("127.0.0.1", port))
                listener.listen()
                result = self.command("start", check=False)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(f"port {port} is occupied", result.stderr)
                self.command("stop")
                with socket.create_connection(("127.0.0.1", port), timeout=1):
                    pass
        self.assert_ports_free()

    def test_stale_control_state_is_not_contacted(self):
        run = self.profile / "run"
        run.mkdir(mode=0o700)
        with socket.socket() as unrelated:
            unrelated.bind(("127.0.0.1", 0))
            unrelated.listen()
            unrelated.settimeout(0.1)
            (run / "control.json").write_text(json.dumps({"port": unrelated.getsockname()[1], "token": "old"}))
            self.command("stop")
            with self.assertRaises(socket.timeout):
                unrelated.accept()
            self.command("start")
        self.command("stop")
        self.assertFalse((run / "control.json").exists())

    def test_wrong_control_token_cannot_stop_services(self):
        self.command("start")
        state = json.loads((self.profile / "run/control.json").read_text())
        with socket.create_connection(("127.0.0.1", state["port"]), timeout=1) as client:
            client.sendall(json.dumps({"command": "stop", "token": "incorrect ☃"}).encode() + b"\n")
            with client.makefile("rb") as stream:
                self.assertEqual(json.loads(stream.readline())["error"], "unauthorized")
        self.assertEqual(json.loads(self.command("status").stdout)["state"], "running")

    def test_invalid_profile_starts_no_services(self):
        (self.profile / ".env").touch()
        result = self.command("start", check=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("profile check failed", result.stderr)
        self.assert_ports_free()

    def test_connector_start_failure_cleans_up_gateway(self):
        (self.root / "fail").touch()
        result = self.command("start", check=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("process exited", result.stderr)
        self.assert_ports_free()

    def test_child_exit_stops_peer(self):
        status = json.loads(self.command("start").stdout)
        os.kill(status["connector_pid"], signal.SIGTERM)
        self.wait_stopped()
        self.assert_ports_free()

    def test_concurrent_starts_leave_one_pair(self):
        command = [sys.executable, str(self.repo / "examples/chatgpt_pilot.py"), "start", "--directory", str(self.profile)]
        processes = [subprocess.Popen(command, env=self.env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
                     for _ in range(2)]
        try:
            outputs = [p.communicate(timeout=55) for p in processes]
            self.assertTrue(any(p.returncode == 0 for p in processes), outputs)
            status = json.loads(self.command("status").stdout)
            self.assertEqual(status["state"], "running")
        finally:
            for p in processes:
                if p.poll() is None:
                    p.terminate()
                    p.wait(timeout=15)
        self.command("stop")
        self.assert_ports_free()


if __name__ == "__main__":
    unittest.main()
