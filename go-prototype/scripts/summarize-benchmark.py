#!/usr/bin/env python3
"""Summarize retained startup-inclusive samples; never time or run calculators."""
import argparse
import hashlib
import json
import math
from pathlib import Path
import statistics


def summarize(samples):
    if len(samples) != 20:
        raise ValueError("require exactly 20 samples per calculator/case")
    for sample in samples:
        if sample.get("error") or sample["exit_code"] != 0 or sample["before"] != sample["after"]:
            raise ValueError("failed or mutating sample cannot be benchmarked")
        if sample["duration_ns"] <= 0 or sample.get("peak_rss_bytes", 0) <= 0:
            raise ValueError("missing timing or peak-memory measurement")
    times = sorted(s["duration_ns"] / 1_000_000 for s in samples)
    memory = [s["peak_rss_bytes"] for s in samples]
    return {"samples": len(samples), "median_ms": statistics.median(times),
            "p95_ms": times[math.ceil(0.95 * len(times)) - 1],
            "min_ms": times[0], "max_ms": times[-1],
            "median_peak_rss_bytes": statistics.median(memory), "max_peak_rss_bytes": max(memory)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("evidence", type=Path)
    parser.add_argument("--go-executable", type=Path, required=True)
    parser.add_argument("--baseline-dir", type=Path, required=True)
    parser.add_argument("--runtime-dir", type=Path, required=True)
    args = parser.parse_args()
    provenance = json.loads((args.evidence / "provenance.json").read_text())
    if not provenance["complete"]:
        raise ValueError("incomplete parity/repeatability run")
    if hashlib.sha256(args.go_executable.read_bytes()).hexdigest() != provenance["candidate_sha256"]:
        raise ValueError("Go executable differs from measured candidate")
    raw = json.loads((args.evidence / "benchmark-samples.json").read_text())
    if set(raw) != {"actions-root", "actions-intermediate"}:
        raise ValueError("benchmark case inventory changed")
    cases = {}
    for name, calculators in raw.items():
        if len(calculators) != 2:
            raise ValueError("missing calculator")
        cases[name] = {label: summarize(calculators[i]) for i, label in enumerate(["csharp", "go"])}
    def size(path):
        files = [p for p in path.rglob("*") if p.is_file()]
        if not files:
            raise ValueError(f"empty distribution directory: {path}")
        return sum(p.stat().st_size for p in files)
    report = {"platform": provenance["platform"], "condition": provenance["benchmark_condition"],
              "percentile": "nearest rank", "cases": cases,
              "distribution_bytes": {"go_single_executable": args.go_executable.stat().st_size,
                                     "csharp_published_directory": size(args.baseline_dir),
                                     "dotnet_runtime_directory": size(args.runtime_dir)},
              "limits": "Warm cache, tiny historical repositories, published framework-dependent C# versus unstripped Go; no NativeAOT comparison. Child Git process count not instrumented."}
    path = args.evidence / "benchmark-summary.json"
    path.write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()
