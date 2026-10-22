#!/usr/bin/env python3
"""Regression tests for the installed skill's HTTP and SSE transport (loopback only)."""
import contextlib
import http.server
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import threading
import time
import unittest

SCRIPT_DIR = Path(__file__).resolve().parent
EVENT = 'id: 1\nevent: message\ndata: {"id":1,"body":"test"}\n\n'


class TransportTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="solvr-transport-")
        self.addCleanup(self.tmp.cleanup)
        self.work = Path(self.tmp.name)
        self.env = dict(os.environ, SOLVR_API_KEY="test-only", SOLVR_CONFIG_DIR=str(self.work / "config"),
                        SOLVR_CONNECT_TIMEOUT="1", SOLVR_REQUEST_TIMEOUT="1", SOLVR_STREAM_IDLE_TIMEOUT="1")
        self.env.pop("SOLVR_ROOM_TOKEN", None)

    def run_cli(self, *args, timeout=6):
        p = subprocess.Popen(["bash", str(SCRIPT_DIR / "solvr.sh"), *args], env=self.env,
                             stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, start_new_session=True)
        try:
            out, err = p.communicate(timeout=timeout)
        except subprocess.TimeoutExpired:
            os.killpg(p.pid, signal.SIGKILL)
            p.communicate()
            self.fail("CLI exceeded its transport deadline")
        return p.returncode, out, err

    def stub(self, content):
        binary = self.work / "bin"
        binary.mkdir()
        curl = binary / "curl"
        curl.write_text("#!/usr/bin/env python3\nimport os, sys, time\n" + content)
        curl.chmod(0o755)
        self.env["PATH"] = str(binary) + os.pathsep + self.env["PATH"]
        self.env["SOLVR_TEST_CURL_PID"] = str(self.work / "curl.pid")

    def assert_network_error(self, result):
        code, out, err = result
        self.assertNotEqual(code, 0, (out, err))
        self.assertEqual(json.loads(err)["error"]["code"], "NETWORK_ERROR")

    def test_01_connection_failure_is_not_success(self):
        self.stub("sys.exit(28)\n")
        result = self.run_cli("room", "watch", "example", "--ticket", "test-only", "--json")
        self.assert_network_error(result)
        self.assertEqual(result[1], "")

    def test_02_incomplete_response_with_http_200_fails(self):
        self.stub("print('{\"data\":[]}\\n200'); sys.exit(18)\n")
        result = self.run_cli("room", "read", "example", "--token", "test-only", "--json")
        self.assert_network_error(result)
        self.assertEqual(result[1], "")

    def test_03_failure_after_an_event_is_reported(self):
        self.stub("sys.stdout.write(" + repr(EVENT) + "); sys.stdout.flush(); sys.exit(18)\n")
        result = self.run_cli("room", "watch", "example", "--ticket", "test-only", "--json")
        self.assert_network_error(result)
        self.assertEqual(json.loads(result[1])["id"], "1")

    def test_04_max_is_success_and_reaps_curl(self):
        self.stub("open(os.environ['SOLVR_TEST_CURL_PID'],'w').write(str(os.getpid()))\n"
                  "sys.stdout.write(" + repr(EVENT) + "); sys.stdout.flush(); time.sleep(30)\n")
        code, out, err = self.run_cli("room", "watch", "example", "--ticket", "test-only", "--max", "1", "--json")
        self.assertEqual(code, 0, err)
        self.assertEqual(json.loads(out)["id"], "1")
        pid = int((self.work / "curl.pid").read_text())
        with self.assertRaises(ProcessLookupError):
            os.kill(pid, 0)

    @contextlib.contextmanager
    def server(self, behavior):
        class Handler(http.server.BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def log_message(self, *_):
                pass

            def do_GET(self):
                try:
                    if behavior == "slow":
                        time.sleep(2)
                        return
                    self.send_response(200)
                    self.send_header("Content-Type", "text/event-stream")
                    self.end_headers()
                    self.wfile.flush()
                    if behavior == "idle":
                        self.wfile.write(b': heartbeat\n\n')
                        self.wfile.flush()
                        time.sleep(3)
                    elif behavior == "heartbeats":
                        for _ in range(7):
                            self.wfile.write(b': heartbeat\n\n')
                            self.wfile.flush()
                            time.sleep(.3)
                        self.wfile.write(EVENT.encode())
                        self.wfile.flush()
                        time.sleep(2)
                except (BrokenPipeError, ConnectionResetError):
                    pass
                self.close_connection = True

        srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        srv.daemon_threads = True
        thread = threading.Thread(target=srv.serve_forever, daemon=True)
        thread.start()
        self.env["SOLVR_API_URL"] = f"http://127.0.0.1:{srv.server_port}/v1"
        try:
            yield
        finally:
            srv.shutdown()
            srv.server_close()
            thread.join()

    def test_05_json_response_wait_is_bounded(self):
        with self.server("slow"):
            start = time.monotonic()
            self.assert_network_error(self.run_cli("room", "read", "example", "--token", "test-only", "--json"))
            self.assertLess(time.monotonic() - start, 2)

    def test_06_connected_stream_silence_fails(self):
        with self.server("idle"):
            start = time.monotonic()
            self.assert_network_error(self.run_cli("room", "watch", "example", "--ticket", "test-only", "--json"))
            self.assertLess(time.monotonic() - start, 2.5)

    def test_07_heartbeats_outlive_request_budget(self):
        with self.server("heartbeats"):
            start = time.monotonic()
            code, out, err = self.run_cli("room", "watch", "example", "--ticket", "test-only", "--max", "1", "--json")
            self.assertEqual(code, 0, err)
            self.assertEqual(json.loads(out)["id"], "1")
            self.assertGreater(time.monotonic() - start, 2)

    def test_08_legacy_stream_has_silence_deadline(self):
        with self.server("idle"):
            code, _, err = self.run_cli("room-stream", "example", "--token", "test-only")
            self.assertNotEqual(code, 0)
            self.assertIn("NETWORK_ERROR", err)

    def test_09_invalid_timeout_never_calls_curl(self):
        self.stub("open(os.environ['SOLVR_TEST_CURL_PID'],'w').write('called'); sys.exit(0)\n")
        self.env["SOLVR_CONNECT_TIMEOUT"] = "0"
        self.assert_network_error(self.run_cli("room", "read", "example", "--token", "test-only", "--json"))
        self.assertFalse((self.work / "curl.pid").exists())

    def test_10_clean_eof_remains_success(self):
        self.stub("sys.stdout.write(" + repr(EVENT) + "); sys.stdout.flush(); sys.exit(0)\n")
        code, out, err = self.run_cli("room", "watch", "example", "--ticket", "test-only", "--json")
        self.assertEqual(code, 0, err)
        self.assertEqual(json.loads(out)["id"], "1")

    def test_11_termination_reaps_transport(self):
        self.stub("open(os.environ['SOLVR_TEST_CURL_PID'],'w').write(str(os.getpid())); time.sleep(30)\n")
        self.env["SOLVR_STREAM_IDLE_TIMEOUT"] = "30"
        p = subprocess.Popen(["bash", str(SCRIPT_DIR / "solvr.sh"), "room", "watch", "example", "--ticket", "test-only"],
                             env=self.env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
                             start_new_session=True)
        try:
            deadline = time.monotonic() + 3
            while not (self.work / "curl.pid").exists() and time.monotonic() < deadline:
                time.sleep(.02)
            self.assertTrue((self.work / "curl.pid").exists())
            pid = int((self.work / "curl.pid").read_text())
            p.terminate()
            p.communicate(timeout=3)
            self.assertEqual(p.returncode, 143)
            with self.assertRaises(ProcessLookupError):
                os.kill(pid, 0)
        finally:
            with contextlib.suppress(ProcessLookupError):
                os.killpg(p.pid, signal.SIGKILL)
            p.communicate()


if __name__ == "__main__":
    unittest.main()
