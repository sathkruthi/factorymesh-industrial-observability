"""Run deliberately unsafe test-only policies, verify expected failure, then verify correction."""
import os
import pathlib
import subprocess
import sys

SCENARIOS = {
    "001": ("FM_UNSAFE_ORDERING", "./internal/store", "TestIncident001Ordering", "late telemetry corrupted current state"),
    "002": ("FM_UNSAFE_ACK", "./internal/pipeline", "TestAckOnlyAfterCommitAndRecovery", "acknowledged before durable commit"),
    "003": ("FM_UNSAFE_ALERT", "./internal/store", "TestIncident003AlertNoise", "noise handling"),
}


def main():
    if len(sys.argv) != 2 or sys.argv[1] not in SCENARIOS:
        raise SystemExit("Usage: python3 scripts/incident.py 001|002|003")
    incident = sys.argv[1]
    variable, package, test, expected = SCENARIOS[incident]
    command = ["go", "test", "-race", "-tags=integration", "-count=1", "-run", "^" + test + "$", "-v", package]
    out = pathlib.Path("evidence-local")
    out.mkdir(exist_ok=True)
    env = os.environ.copy()
    for key, *_ in SCENARIOS.values():
        env.pop(key, None)
    env[variable] = "1"
    broken = subprocess.run(command, env=env, text=True, capture_output=True, timeout=90)
    broken_text = broken.stdout + broken.stderr
    (out / (incident + "-broken.txt")).write_text(broken_text)
    assert broken.returncode != 0 and expected in broken_text, "Broken policy did not fail for the intended reason: " + broken_text
    env.pop(variable)
    fixed = subprocess.run(command, env=env, text=True, capture_output=True, timeout=90)
    fixed_text = fixed.stdout + fixed.stderr
    (out / (incident + "-fixed.txt")).write_text(fixed_text)
    assert fixed.returncode == 0, fixed_text
    print(f"INCIDENT-{incident}: unsafe policy failed for expected reason; corrected policy passed.")
    print(broken_text.strip())
    print(fixed_text.strip())


if __name__ == "__main__":
    main()
