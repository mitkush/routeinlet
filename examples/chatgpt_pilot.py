"""Manage the local ChatGPT pilot on macOS/Linux; never sends inference requests."""
import argparse
import fcntl
import hashlib
import hmac
import json
import os
from pathlib import Path
import select
import secrets
import signal
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
CONNECTOR = "http://127.0.0.1:8318"
GATEWAY = "http://127.0.0.1:8787"
START_TIMEOUT = 45


def private_file(path):
    return os.fdopen(os.open(path, os.O_CREAT | os.O_APPEND | os.O_WRONLY | os.O_NOFOLLOW, 0o600), "a")


def runtime_dir(profile):
    if not profile.is_dir():
        raise ValueError("pilot directory is missing; run init-chatgpt-pilot first")
    run = profile / "run"
    run.mkdir(mode=0o700, exist_ok=True)
    if run.is_symlink() or run.stat().st_uid != os.getuid() or run.stat().st_mode & 0o077:
        raise ValueError("pilot run directory must be owned by you with permissions 0700")
    return run


def configuration():
    required = ("CONNECTOR_BIN", "ROUTEINLET_MODEL", "ROUTEINLET_KEY", "ROUTEINLET_CONNECTOR_KEY")
    for name in required:
        if not os.environ.get(name):
            raise ValueError("set " + name + " in this terminal before starting")
    binary = Path(os.environ["CONNECTOR_BIN"])
    if not binary.is_absolute() or not binary.is_file() or not os.access(binary, os.X_OK):
        raise ValueError("CONNECTOR_BIN must name an existing absolute executable path")
    if not os.access(ROOT / "routeinlet", os.X_OK):
        raise ValueError("build RouteInlet first: go build -o routeinlet ./cmd/routeinlet")
    if os.environ.get("ROUTEINLET_CONNECTOR", CONNECTOR) != CONNECTOR:
        raise ValueError("this generated pilot requires ROUTEINLET_CONNECTOR=" + CONNECTOR)
    # Compare configurations across duplicate starts without writing keys to disk
    # or exposing them in status responses. The keys remain in process memory.
    settings = [str(ROOT)] + [os.environ[name] for name in required]
    return hashlib.sha256(json.dumps(settings).encode()).hexdigest()


def control(run, command, fingerprint=None):
    # A stale state file must never send commands to a reused port.
    if lock_available(run):
        return None
    try:
        try:
            with open(run / "control.json") as state_file:
                state = json.load(state_file)
        except ValueError:
            raise ValueError("invalid control state; restore a valid run/control.json before using status or stop") from None
        if (not isinstance(state, dict) or type(state.get("port")) is not int or
                not 1 <= state["port"] <= 65535 or not isinstance(state.get("token"), str) or
                len(state["token"]) != 64 or any(c not in "0123456789abcdef" for c in state["token"])):
            raise ValueError("invalid control state; restore a valid run/control.json before using status or stop")
        with socket.create_connection(("127.0.0.1", state["port"]), timeout=2) as client:
            client.sendall(json.dumps({"command": command, "fingerprint": fingerprint,
                                      "token": state["token"]}).encode() + b"\n")
            with client.makefile("rb") as stream:
                return json.loads(stream.readline(4096))
    except (FileNotFoundError, ConnectionRefusedError):
        return None


def lock_available(run):
    with private_file(run / "lock") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            return True
        except BlockingIOError:
            return False


def check_ports():
    for port in (8318, 8787):
        occupied = (f"port {port} is occupied; inspect with lsof -nP -iTCP:{port} -sTCP:LISTEN. "
                    "No existing process was stopped.")
        with socket.socket() as listener:
            # Retain reuse to avoid rejecting Linux TIME_WAIT sockets after stop.
            listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            try:
                listener.bind(("127.0.0.1", port))
            except OSError:
                raise ValueError(occupied) from None
        # On macOS the reusable bind can coexist with a wildcard listener.
        # A successful connection detects that listener before launching children.
        try:
            connection = socket.create_connection(("127.0.0.1", port), timeout=1)
        except ConnectionRefusedError:
            continue
        except OSError:
            raise ValueError(f"could not verify port {port} is free; inspect its listener before starting") from None
        else:
            connection.close()
            raise ValueError(occupied)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args):
        return None


def ready():
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    catalog = urllib.request.Request(CONNECTOR + "/v1/models", headers={
        "Authorization": "Bearer " + os.environ["ROUTEINLET_CONNECTOR_KEY"]})
    # stream=false is deliberately rejected by preflight before dispatch. This
    # also checks that the gateway's key and model match, without inference.
    probe = urllib.request.Request(GATEWAY + "/v1/chat/completions", data=json.dumps({
        "model": os.environ["ROUTEINLET_MODEL"], "stream": False,
        "messages": [{"role": "user", "content": "readiness"}]}).encode(), headers={
            "Content-Type": "application/json", "Authorization": "Bearer " + os.environ["ROUTEINLET_KEY"]})
    try:
        with opener.open(catalog, timeout=1) as response:
            if response.status != 200:
                return False
            listing = json.loads(response.read(1 << 20))
        entries = listing.get("data") if isinstance(listing, dict) else None
        if not isinstance(entries, list) or not entries:
            return False  # The connector may still be loading its credentials.
        models = {entry.get("id") for entry in entries if isinstance(entry, dict) and isinstance(entry.get("id"), str)}
        if not models:
            return False
        if os.environ["ROUTEINLET_MODEL"] not in models:
            raise RuntimeError("ROUTEINLET_MODEL is not listed by the connector; available IDs: " + ", ".join(sorted(models)[:20]))
        try:
            with opener.open(probe, timeout=1):
                return False
        except urllib.error.HTTPError as exc:
            with exc:
                return exc.code == 400 and json.loads(exc.read(4096)).get("error", {}).get("code") == "stream_required"
    except urllib.error.HTTPError as exc:
        exc.close()
        return False
    except (OSError, ValueError):
        return False


def stop_children(children):
    for child in reversed(children):
        if child.poll() is None:
            child.terminate()
    for child in reversed(children):
        try:
            child.wait(timeout=5)
        except subprocess.TimeoutExpired:
            child.kill()
            child.wait()


def supervise(profile, run):
    children = []
    logs = []
    stopping = False
    announced = False
    bound = False
    state_path = run / "control.json"
    token = secrets.token_hex(32)

    def request_stop(*_):
        nonlocal stopping
        stopping = True

    signal.signal(signal.SIGTERM, request_stop)
    signal.signal(signal.SIGINT, request_stop)
    with private_file(run / "lock") as lock, socket.socket() as server:
        try:
            try:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError:
                raise ValueError("pilot is already starting or running; use status") from None
            fingerprint = configuration()
            check_ports()
            env = dict(os.environ, PILOT_DIR=str(profile), ROUTEINLET_CONNECTOR=CONNECTOR)
            # Use the same profile check as the connector launcher, before any child.
            checked = subprocess.run([str(ROOT / "routeinlet"), "check-chatgpt-pilot"], cwd=profile,
                                     env=env, capture_output=True, timeout=10)
            if checked.returncode:
                raise ValueError("pilot profile check failed: " + checked.stderr.decode(errors="replace").strip())
            server.bind(("127.0.0.1", 0))
            server.listen(4)
            server.settimeout(0.2)
            # Only the lock holder replaces state. The random token authorizes
            # local control; it is unrelated to gateway or provider credentials.
            bound = True
            temporary_state = run / "control.tmp"
            temporary_state.unlink(missing_ok=True)
            with private_file(temporary_state) as state_file:
                json.dump({"port": server.getsockname()[1], "token": token}, state_file)
            os.replace(temporary_state, state_path)
            commands = [("connector", ["/bin/sh", str(ROOT / "examples/chatgpt-connector.sh"), "serve"]),
                        ("gateway", [str(ROOT / "routeinlet"), "start"])]
            for name, command in commands:
                log = private_file(run / (name + ".log"))
                logs.append(log)
                children.append(subprocess.Popen(command, cwd=ROOT, env=env, stdin=subprocess.DEVNULL,
                                                 stdout=log, stderr=subprocess.STDOUT))
            deadline = time.monotonic() + START_TIMEOUT - 5
            while not stopping:
                if any(child.poll() is not None for child in children):
                    raise ValueError("a pilot process exited; inspect the private logs in " + str(run))
                if ready():
                    break
                if time.monotonic() >= deadline:
                    raise ValueError("pilot readiness timed out; inspect the private logs in " + str(run))
                time.sleep(0.1)
            if stopping:
                raise ValueError("pilot startup interrupted")
            status = {"state": "running", "model": os.environ["ROUTEINLET_MODEL"],
                      "connector": CONNECTOR, "gateway": GATEWAY,
                      "connector_pid": children[0].pid, "gateway_pid": children[1].pid,
                      "logs": str(run)}
            print(json.dumps(status), flush=True)
            announced = True
            while not stopping:
                if any(child.poll() is not None for child in children):
                    raise ValueError("a pilot process exited; stopping its peer")
                try:
                    connection, _ = server.accept()
                except socket.timeout:
                    continue
                with connection:
                    connection.settimeout(1)
                    try:
                        with connection.makefile("rb") as stream:
                            message = json.loads(stream.readline(4096))
                        if not isinstance(message, dict) or not isinstance(message.get("token"), str) or not hmac.compare_digest(message["token"].encode(), token.encode()):
                            connection.sendall(b'{"error":"unauthorized"}\n')
                            continue
                        reply = dict(status)
                        if message.get("command") == "stop":
                            stopping = True
                            reply["state"] = "stopping"
                        elif message.get("command") != "status":
                            reply = {"error": "unknown command"}
                        elif message.get("fingerprint") is not None:
                            reply["matches"] = isinstance(message["fingerprint"], str) and hmac.compare_digest(message["fingerprint"].encode(), fingerprint.encode())
                        connection.sendall(json.dumps(reply).encode() + b"\n")
                    except (OSError, ValueError):
                        continue
        except Exception as exc:
            if not announced:
                print(json.dumps({"error": str(exc)}), flush=True)
            else:
                print(str(exc), file=sys.stderr, flush=True)
        finally:
            stop_children(children)
            for log in logs:
                log.close()
            if bound:
                state_path.unlink(missing_ok=True)


def start(profile, run):
    fingerprint = configuration()
    existing = control(run, "status", fingerprint)
    if existing:
        if not existing.get("matches"):
            raise ValueError("pilot is running with different settings; use status, then stop before starting again")
        print("Already running: " + json.dumps(existing))
        return
    with private_file(run / "supervisor.log") as log:
        proc = subprocess.Popen([sys.executable, str(Path(__file__).resolve()), "_supervise", "--directory", str(profile)],
                                stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=log, start_new_session=True)
        try:
            readable, _, _ = select.select([proc.stdout], [], [], START_TIMEOUT)
            if not readable:
                raise ValueError("pilot startup timed out; inspect " + str(run / "supervisor.log"))
            result = json.loads(proc.stdout.readline(4096))
            if "error" in result:
                raise ValueError(result["error"])
            print(json.dumps(result))
        except BaseException:
            proc.terminate()
            proc.wait(timeout=15)
            raise
        finally:
            proc.stdout.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("start", "stop", "status", "_supervise"))
    parser.add_argument("--directory", default=os.environ.get("PILOT_DIR", str(Path.home() / "routeinlet-chatgpt-pilot")))
    args = parser.parse_args()
    profile = Path(args.directory).expanduser().resolve()
    run = runtime_dir(profile)
    if args.command == "_supervise":
        supervise(profile, run)
    elif args.command == "start":
        start(profile, run)
    else:
        result = control(run, args.command)
        if result is None:
            if not lock_available(run):
                raise ValueError("pilot is starting or stopping; retry shortly")
            print("Stopped (no managed pilot). Existing unmanaged processes are not controlled.")
            return
        if args.command == "stop":
            deadline = time.monotonic() + 15
            while not lock_available(run):
                if time.monotonic() >= deadline:
                    raise ValueError("shutdown is still in progress; inspect the private logs")
                time.sleep(0.1)
            print("Stopped connector and gateway.")
        else:
            print(json.dumps(result))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, subprocess.SubprocessError) as exc:
        raise SystemExit("Pilot: " + str(exc)) from None
