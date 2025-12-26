# JPM Phase 2 + Native Engine Implementation Plan

Date: 2025-11-09T15:38:44.882Z
Owner: Core CLI / Engine
Status: Planning (Ready for backlog sizing)

---
## Scope Overview
Phase 2 focuses on: JDK install assist, improved init UX, telemetry opt-in, generators (.gitignore, README), and foundation for a future native build engine (Phase 2+ but planned now). Native Engine replaces Maven for build/deps with an internal resolver, fetcher, compiler, packager, and dependency graph services.

---
## Guiding Principles
- Prompt-first, reversible, minimal friction.
- Secure: HTTPS-only, checksum verification, deterministic lockfile.
- Performant: Parallel resolution/fetch with bounded concurrency.
- Offline-resilient: Graceful degradation with cached artifacts.
- Extensible: Clear interfaces (Resolver, Fetcher, Compiler, Packager).
- Observability: Structured logs + optional telemetry (privacy preserving).

---
## High-Level Epics
1. Install Assist & JDK Validation
2. Init UX Polish & Safety Enhancements
3. Generators (.gitignore, README.md)
4. Native Dependency Resolution & Fetching Core
5. Native Build Pipeline (Compile / Package / Run)
6. Lockfile & Dependency Graph Operations
7. CLI Enhancements (deps tree/audit, verbose, offline)
8. Testing & CI Matrix (Windows/macOS/Linux)
9. Performance & Caching Optimization

---
## Phase 2 Detailed Task Breakdown
### 1. Install Assist & JDK Validation
- Detect JDK: PATH + JAVA_HOME + `java -version` parser (already partial; extend for version extraction).
- Implement platform strategy matrix (macOS, Linux, Windows).
- SDKMAN integration (prompt consent, download script, verify success).
- Homebrew integration (macOS): brew install temurin / openjdk.
- Windows hints: winget + choco command suggestions.
- Manual path entry prompt: validate `<path>/bin/java` executable + version.
- Retry loop for failed detection (max N attempts, escape hatch Skip).
- Structured outcomes (Installed | Reused | Skipped) for telemetry.

### 2. Init UX Polish & Safety
- Enhanced non-empty dir preview: list first N entries + "Show more" paging.
- Dry-run mode: `jpm init --dry-run` prints planned actions without writing.
- Overwrite strategy refinement: offer per-file choices (Skip / Overwrite / Write .new).
- Rollback workflow: transactional creation ledger + rollback on failure.
- Timing output (already planned) with high-resolution clock.
- Input validation helpers: artifactId sanitizer (already) + groupId rules + Java version whitelist {17,21,23}.
- Template selection future-proofing (show disabled Gradle/Spring items with "coming soon").

### 3. Generators
- .gitignore generator:
  - Patterns: `.jpm/out/`, `.jpm/logs/`, `.jpm/cache/`, `target/`, `.idea/`, `.classpath`, `.settings/`, `.project`, `*.iml`, `/.vscode/`.
  - Idempotent append: preserve user modifications, avoid duplicates.
  - Unit test: ensure required lines present and no duplicates after multiple init runs.
- README.md generator (optional toggle):
  - Sections: Introduction, Quickstart, Commands (init/build/run/deps add/tree), Structure, Updating Dependencies, Troubleshooting.
  - Detect if README exists: prompt overwrite/skip.
  - Template injection: project name + main class + Java version.

### 4. Native Dependency Resolution Core
Interfaces:
- Resolver: builds full dependency graph from manifest entries.
- Model: Dependency(G,A,V, scope, optional, classifier, exclusions[]).
- Graph node: coordinates + metadata + children.
Tasks:
- Parse manifest dependencies into internal form.
- POM fetch + parse: parent, dependencyManagement import, exclusions, optional.
- Version resolution policy: direct version wins; BOM/import (phase-gated) future; nearest-wins fallback.
- Conflict resolution strategy documented & test matrix.
- Cycles detection (fail with explanation).
- Produce resolved set (GAV + scope + optional flag).
- Generate lockfile `jpm.lock.yaml` including:
  - root dependencies
  - resolved versions
  - SHA-256 checksums
  - source repository URL
  - timestamp & tool version.
- Idempotence: same manifest => same lockfile order & content.

### 5. Native Fetcher & Cache
- Artifact & POM retrieval (HTTPS only).
- Repository config: default Maven Central; allow additional repos in manifest later (phase-gate advanced features).
- Parallel fetch: worker pool (configurable concurrency, default = min(8, CPU*2)).
- Retry strategy: exponential backoff (max 3 attempts) for transient network errors.
- Integrity: compute SHA-256; compare with lockfile on reuse; embed if first time.
- Cache layout: `.jpm/cache/artifacts/<sha256>` & metadata index mapping GAV -> sha256.
- Partial download handling: temp files with `.part` suffix; resume or purge.
- Offline mode: if `--offline`, only use cache; error if missing with actionable message.
- Metrics reporting: counts (hits/misses), total bytes, durations.

### 6. Native Build (Compile / Package / Run)
- Classpath assembly from resolved artifacts.
- `javac` invocation: target classes to `.jpm/work/classes`.
- Source discovery: flat `src/` (no enforced package layout).
- Main class detection: scan for public class with `main(String[] args)` when missing.
- Jar builder: manifest with `Main-Class` when known; output to `.jpm/out/<artifact>-<version>.jar`.
- Incremental compile flag (future): detect changed source timestamps (phase-gate).
- Logging: build steps + timings to `.jpm/logs/build-<ts>.log`.
- Run pipeline: `java -cp <classpath> <mainClass>`.

### 7. Lockfile Operations & CLI Extensions
- `jpm deps tree` (native): render graph with scopes.
- `jpm deps audit` (placeholder): identify duplicates/conflicts.
- `jpm deps sync`: reconcile manifest vs lockfile; offer preview diff (added/removed/changed).
- Lockfile regeneration rule: only when manifest changed explicitly or `--update` used.
- Human-readable diff printing (old vs new lock entries).

### 8. CLI Enhancements
- Global `--offline` flag.
- Global `--verbose` flag: extra logs & executed commands.
- Error taxonomy: network (retry), resolution (user fix), parse (report bad POM), integrity (checksum mismatch).
- Styled warnings for optional or skipped steps.

### 9. Testing & CI
- Unit tests: version parser, artifactId sanitizer, POM parser (parent, deps, exclusions), lockfile deterministic ordering.
- Integration tests: temp project -> resolve -> build -> run hello world.
- Cache tests: repeated build uses cache (timing assertion).
- Offline test: prepopulate cache; run with --offline; success.
- Windows symlink fallback verification.
- GitHub Actions matrix: ubuntu-latest, macos-latest, windows-latest.

### 10. Performance & Optimization
- Benchmark: cold resolve vs warm resolve (cache hit ratio).
- Memory profiling for large graphs (simulate with synthetic dependency tree).
- Concurrency tuning: measure diminishing returns beyond N workers.
- Artifact size threshold: skip prefetch of javadoc/sources until requested.

### 11. Documentation Updates
- Update INIT_PROTOTYPE.md with Phase 2 status section.
- Add NATIVE_ENGINE.md (architecture overview, interfaces, flow diagrams) - future.
- README template improvements with native engine, lockfile explanation.

### 12. Security & Integrity
- Enforce HTTPS; reject plain HTTP repositories by default.
- Checksum verification mandatory once stored in lockfile.
- Graceful failure if checksum mismatch (instructions to clear cache or update).
- Basic supply chain note in README (integrity + reproducibility via lockfile).

---
## Native Engine Architectural Outline
Layers:
1. Manifest Loader
2. Resolver (graph building)
3. Lockfile Manager
4. Fetcher (network/cache)
5. Classpath Assembler
6. Compiler Driver
7. Packager (Jar)
8. Runtime Launcher
9. CLI Adapters (build/run/deps commands)

Data Flows:
Manifest -> Resolver -> Graph -> Lockfile -> Fetcher -> Cache -> Classpath -> Compile -> Jar -> Run.

Interfaces (Go):
- Resolver: Resolve(manifest) -> Graph, ResolvedSet
- Fetcher: Fetch(GAV, kind[POM|JAR]) -> Path, Metadata
- Cache: Get(GAV) -> Path? ; Put(GAV, bytes) -> Path
- Compiler: Compile(sources, classpath, outDir) -> Result
- Packager: Package(classesDir, manifestMeta) -> JarPath

---
## Dependency Fetching Strategy (Optimized)
1. Pre-Scan Root Dependencies: group by repository; seed queue.
2. Iterative Graph Expansion: breadth-first to favor nearer (conflict resolution alignment).
3. Memoization: do not re-parse POM twice; store parent/management info.
4. Concurrency: worker pool; each worker handles fetch+parse; futures for dependent nodes.
5. Conflict Resolution: nearest-wins (document); allow overrides in manifest (future: rules section).
6. Lockfile Determinism: sort entries by groupId:artifactId before write.
7. Checksums: compute on first fetch; store in lockfile; verify on reuse.
8. Failure Handling: if artifact fetch fails -> mark node unresolved; continue; surface aggregated warnings.
9. Offline Mode: skip network; mark missing nodes; fail fast if root dependency absent in cache.
10. Cache Keying: by SHA-256; map GAV -> sha in metadata index to allow relocations/mirrors.
11. Retry Policy: network timeouts/backoff; checksum mismatch triggers single redownload.
12. Metrics: capture durations per phase (resolve, fetch, compile) for future tuning.

---
## .gitignore Generation Details
Rules (ordered):
```
# JPM build artifacts
.jpm/out/
.jpm/logs/
.jpm/cache/
# Maven/Gradle remnants (if any)
target/
# IDE/Editor metadata
.idea/
.classpath
.settings/
.project
*.iml
.vscode/
```
Behavior:
- If .gitignore absent: create with template.
- If present: append missing lines; avoid duplicates; preserve user spacing.
- Tests: run generator twice -> lines appear once.

---
## README Generator Template (Draft)
Sections:
1. Project Name & Description (user-editable placeholder).
2. Quickstart:
   - `jpm init`
   - `jpm build`
   - `jpm run`
3. Structure Overview.
4. Dependencies Management: `jpm deps add group:artifact@version`, `jpm deps tree`.
5. Lockfile & Reproducibility.
6. Offline & Caching.
7. Updating Java version.
8. Troubleshooting (Java not found, symlink fallback, checksum mismatch).

---
## Backlog Prioritization (Suggested Order)
1. Install Assist (minimal) + README/.gitignore.
2. Resolver + POM parsing core.
3. Fetcher + Cache + Lockfile.
4. Native build (compile/package/run).
5. CLI extensions (deps tree/audit/sync, offline, verbose).
6. Performance & telemetry instrumentation.
7. Polish (dry-run, rollback, advanced overwrite strategies).

---
## Risks & Mitigations
- Complexity creep in resolver: keep BOM/import management out until stable base; feature-flag advanced cases.
- Network flakiness: robust retry + clear error taxonomy.
- Windows path/symlink issues: fallback already; ensure tests on Windows runner.
- Large dependency graphs: streaming build of graph + memory profiling early.

---
## Acceptance Criteria (Phase 2 + Native Seed)
- Init offers JDK assist with at least one automated path (SDKMAN/Linux, brew/macOS).
- .gitignore and README generated (idempotent).
- `jpm build` can operate with native engine (flag or auto when engine=native) for simple projects with 0–N dependencies.
- Lockfile created and reused on subsequent builds; deterministic.
- `jpm deps tree` shows resolved graph.
- Offline build works when cache primed.
- All network fetches checksum-verified and cached.

---
## Initial Story Seeds (Examples)
- STORY: Implement JDK detection & version parse (PATH + JAVA_HOME).
- STORY: Add SDKMAN installer w/ consent prompt.
- STORY: Generate .gitignore on init (idempotent).
- STORY: Generate README.md (prompt if exists).
- STORY: Define Manifest->Dependency translation.
- STORY: Implement POM fetch & parse (dependencies + exclusions + optional).
- STORY: Build resolver graph + conflict nearest-wins.
- STORY: Write lockfile with sorted entries + checksums.
- STORY: Implement artifact fetcher + cache layout.
- STORY: javac compile pipeline producing classes + jar.
- STORY: Main class auto-detect fallback.
- STORY: deps tree command using resolver output.
- STORY: Offline mode flag + cache-only behavior.
- STORY: README update docs for native engine preview.

---
## Out of Scope (Future Considerations)
- Gradle native parity.
- BOM/import dependencyManagement merge logic.
- Version ranges (+latest selection heuristics).
- Security advisories/Audit integration.
- Sources/javadoc artifact fetching.

---
## Final Notes
This plan seeds backlog items; each story should include success criteria + tests. Start with vertical slices (small project from init -> native build -> run) before expanding edge cases. Keep prototype flags to allow gradual rollout (engine=maven default until native stable).
