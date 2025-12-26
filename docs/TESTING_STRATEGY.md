# JPM Testing Strategy (Phase 2 + Native Engine)

Date: 2025-11-09
Owner: Core Team
Status: Stable Plan

---
## Goals
- Ensure functional correctness, determinism, security, and performance of the native engine and Phase 2 features.
- Provide a repeatable CI matrix across Linux/macOS/Windows and Java 17/21/23.

---
## Test Pyramid
- Unit tests: fast, isolated, high coverage for resolver/fetcher/compiler/packager/classpath/manifest parsing.
- Integration tests: validate cross-module behavior (resolve→fetch→compile→package→run).
- End-to-end (E2E) smoke: CLI scenarios mirroring user workflows.

---
## Unit Tests
- Manifest: YAML parse/serialize round-trip; defaults; validation (java versions, ids).
- POM Parser: parent inheritance, properties, exclusions, optional, scopes.
- Resolver: nearest-wins conflicts, cycle detection, memoization, exclusions applied.
- Fetcher: cache hit/miss, checksum verify, retry backoff, offline failure listing, concurrent fetch lock.
- Classpath: scope filtering, ordering stability, OS separators.
- Compiler: no-sources success, error parsing, argfile chunking.
- Packager: deterministic JAR (byte-identical); manifest fields; error on empty classes.
- Generators: .gitignore idempotence; README placeholder substitution.

---
## Integration Tests
- Resolve+Fetch real deps (e.g., slf4j-api, guava) against Maven Central (networked).
- Offline build with primed cache; verify zero network calls.
- Conflict graph resolution with synthetic POM fixtures (local httptest server).
- End-to-end: `jpm init` → `jpm deps add` → `jpm build` → `jpm run` (hello world).
- Lockfile reuse: second build uses cache and produces identical lockfile.
- Windows path/junction fallback with symlink-restricted environment.

---
## Performance Tests
- Cold vs warm builds: measure resolve/fetch/compile/package timings; set thresholds.
- Concurrency scaling: N workers vs throughput for fetch; diminishing returns curve.
- Large graph synthetic: 500+ artifacts; memory usage bounds; CPU time.

---
## Security & Integrity Tests
- HTTPS enforcement: reject http URLs.
- Checksum mismatch: detect and fail after single retry; remediation message.
- Cache corruption: invalid file contents detected by SHA-256.

---
## CI Matrix
- OS: ubuntu-latest, macos-latest, windows-latest
- Java: 17, 21 (primary), 23 (preview)
- Jobs: unit → integration → e2e (smoke); artifact caching between jobs optional

---
## Tooling & Conventions
- Go test with race detector for unit/integration where feasible.
- Use local httptest server for deterministic POM/artifact fixtures.
- Golden files for lockfile outputs (sorted, deterministic).

---
## Acceptance & Regression
- Acceptance: criteria from PHASE2_NATIVE_ENGINE_PLAN.md must pass on CI.
- Regression suite: pin known-bug scenarios (conflict resolution, exclusions, offline miss list).

---
## Coverage Targets
- Unit: ≥80% across resolver/fetcher/build pipeline.
- Integration/E2E: scenario coverage documented; not measured by %.

---
## Flake Management
- Mark networked tests as "slow"; allow retries with capped count.
- Prefer fixture-based tests to minimize external dependencies.
