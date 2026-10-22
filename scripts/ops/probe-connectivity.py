#!/usr/bin/env python3
"""Compare normal DNS with each advertised address. Output contains no client IP or credentials."""
import argparse
import concurrent.futures
import datetime
import json
import math
import socket
import subprocess


def probe(host, address=None):
    url = "https://" + host + ("/health" if host == "api.solvr.dev" else "/")
    command = ["curl", "--noproxy", "*", "--connect-timeout", "5", "--max-time", "10",
               "-A", "Mozilla/5.0 (Solvr connectivity probe)", "-sS", "-o", "/dev/null", "-w",
               "%{remote_ip}|%{http_code}|%{time_namelookup}|%{time_connect}|%{time_appconnect}|%{time_starttransfer}|%{time_total}"]
    if address:
        resolved = "[" + address + "]" if ":" in address else address
        command += ["--resolve", host + ":443:" + resolved]
    result = subprocess.run(command + [url], capture_output=True, text=True, timeout=12)
    fields = result.stdout.split("|")
    if len(fields) != 7:
        return {"host": host, "target": address, "exit": result.returncode, "ok": False}
    return {"host": host, "target": address, "exit": result.returncode, "remote": fields[0],
            "status": fields[1], "ok": result.returncode == 0 and fields[1] == "200",
            **{name: float(value) for name, value in zip(
                ["dns_s", "connect_s", "tls_s", "first_byte_s", "total_s"], fields[2:])}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--attempts", type=int, default=3, help="fresh normal-DNS requests per host")
    parser.add_argument("--normal-only", action="store_true", help="omit individual-address probes")
    args = parser.parse_args()
    if args.attempts < 1:
        parser.error("--attempts must be positive")
    report = {"at_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(), "hosts": []}
    healthy = True
    for host in ["solvr.dev", "api.solvr.dev"]:
        addresses = sorted({row[4][0] for row in socket.getaddrinfo(host, 443, type=socket.SOCK_STREAM)})
        per_address = []
        if not args.normal_only:
            with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
                per_address = list(pool.map(lambda address: probe(host, address), addresses))
        normal = [probe(host) for _ in range(args.attempts)]
        times = sorted(row["total_s"] for row in normal if row["ok"])
        p95 = times[math.ceil(len(times) * .95) - 1] if times else None
        ok = all(row["ok"] for row in normal + per_address)
        healthy = healthy and ok
        report["hosts"].append({"host": host, "addresses": addresses, "per_address": per_address,
                                "normal": normal, "normal_p95_s": p95, "all_requests_ok": ok})
    print(json.dumps(report, indent=2))
    return 0 if healthy else 1


if __name__ == "__main__":
    raise SystemExit(main())
