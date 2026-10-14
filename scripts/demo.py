import json
import subprocess
import urllib.request

with urllib.request.urlopen("http://127.0.0.1:8080/devices/press-001", timeout=5) as response:
    sequence = json.load(response)["sequence"] + 1
subprocess.run(["docker", "compose", "run", "--rm", "simulator", "--start", str(sequence)], check=True)
