# Benchmarks

This folder contains reproducible benchmark scripts used to compare the JPM native engine to Maven.

## What we measure

- **Native engine**: `jpm build project_test` (manifest has `engine: native`).
- **Maven**: Maven build (when `mvn` is installed) using a generated `pom.xml` from the same `jpm.yaml`.

The scripts write raw logs + a small JSON summary you can paste into the report.

## Quick run

From repo root:

```bash
./bench/bench.sh
```

If Maven is not installed, the script still benchmarks native and records Maven as `skipped`.

## Outputs

- `bench/results/latest/bench.json` – machine-readable summary
- `bench/results/latest/native.log` – raw native output
- `bench/results/latest/maven.log` – raw maven output (if available)

## Notes

- For fairness, each iteration deletes build outputs before timing.
- First iteration includes JVM warmup cost (javac startup); the script reports both **cold** and **warm** (excluding iteration 1) aggregates.
