"""Local fault injection. Requires the factorymesh Compose stack; never targets a remote daemon."""
import json
import re
import subprocess
import time
import urllib.error
import urllib.request

BASE = "http://127.0.0.1:8080"


def compose(*args):
    result = subprocess.run(["docker", "compose", *args], check=True, capture_output=True, text=True, timeout=90)
    return result.stdout.strip()


def get(path):
    try:
        with urllib.request.urlopen(BASE + path, timeout=6) as response:
            return response.status, response.read().decode()
    except urllib.error.HTTPError as error:
        return error.code, error.read().decode()


def until(predicate, message):
    deadline = time.monotonic() + 60
    while time.monotonic() < deadline:
        try:
            if predicate():
                return
        except (OSError, urllib.error.URLError):
            pass
        time.sleep(0.25)
    raise AssertionError(message)


def state():
    code, body = get("/devices/press-003")
    assert code == 200, (code, body)
    return json.loads(body)


def metrics():
    code, body = get("/metrics")
    assert code == 200
    return {m[0]: float(m[1]) for m in re.findall(r"^(factorymesh_\w+) ([0-9.e+\-]+)$", body, re.M)}


def main():
    context = subprocess.run(["docker", "context", "inspect"], check=True, capture_output=True, text=True)
    endpoint = json.loads(context.stdout)[0]["Endpoints"]["docker"]["Host"]
    assert endpoint.startswith("unix://"), "Fault injection requires a local Unix-socket Docker context"
    assert get("/health/ready")[0] == 200
    start = state()["sequence"] + 1
    before = metrics().get("factorymesh_retries_total", 0)
    try:
        compose("stop", "database")
        assert get("/health/live")[0] == 200
        assert get("/health/ready")[0] == 503
        output = compose("run", "--rm", "--no-deps", "simulator", "--device", "press-003", "--start", str(start), "--count", "120", "--interval", "0s", "--duplicate")
        fixtures = [json.loads(line) for line in output.splitlines() if line.startswith("{")]
        assert len(fixtures) == 120
        until(lambda: metrics().get("factorymesh_retries_total", 0) > before, "database outage did not exercise retry")
        observed = []
        for _ in range(12):
            m = metrics()
            observed.append(m["factorymesh_queue_depth"])
            assert 0 <= m["factorymesh_queue_depth"] <= 64, m
            time.sleep(0.25)
        print(json.dumps({"phase": "outage", "published": 240, "queue_max_observed": max(observed), "retry_count": metrics()["factorymesh_retries_total"], "liveness": 200, "readiness": 503}))
        compose("start", "database")
        until(lambda: get("/health/ready")[0] == 200, "database recovery readiness failed")
        until(lambda: state()["sequence"] == start + 119, "pending events did not recover")
        query = "SELECT count(*) FROM telemetry WHERE device='press-003' AND generation=1 AND sequence BETWEEN %d AND %d" % (start, start + 119)
        n = compose("exec", "-T", "database", "psql", "-U", "factorymesh", "-d", "factorymesh", "-Atc", query)
        assert n == "120", ("duplicate business effects", n)
        compose("stop", "broker")
        until(lambda: get("/health/ready")[0] == 503, "broker failure not detected")
        assert get("/health/live")[0] == 200
        compose("start", "broker")
        until(lambda: get("/health/ready")[0] == 200, "broker reconnect failed")
        # Replay identical evidence after a reconnect. Database effects must remain unchanged.
        fixture = json.dumps(fixtures[-1], separators=(",", ":"))
        compose("exec", "-T", "broker", "mosquitto_pub", "-h", "localhost", "-q", "1", "-t", "factorymesh/telemetry", "-m", fixture)
        until(lambda: state()["sequence"] == start + 119, "post-reconnect state regression")
        time.sleep(0.5)
        assert compose("exec", "-T", "database", "psql", "-U", "factorymesh", "-d", "factorymesh", "-Atc", query) == "120"
        print(json.dumps({"phase": "recovered", "stored_unique": 120, "last_sequence": state()["sequence"], "broker_reconnected": True}))
    finally:
        # Restore only this named local project's dependencies, even when assertions fail.
        compose("start", "database", "broker")


if __name__ == "__main__":
    main()
