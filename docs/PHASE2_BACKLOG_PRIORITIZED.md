# Phase 2 Implementation Backlog (Prioritized)

Date: 2025-11-09T15:54:45Z
Status: Ready for Sprint Planning

---
## Sprint 1: Generators + Init Polish (1-2 weeks)
Quick wins; unblocks user-facing polish; low risk.

### Tasks
1. **Implement .gitignore Generator** 
   - File: `internal/init/gitignore.go`
   - Read existing .gitignore (if exists); append JPM patterns idempotently
   - Unit test: idempotence, line preservation
   - Integrate into `jpm init` after scaffold

2. **Implement README.md Generator**
   - File: `internal/init/readme.go`
   - Template with placeholders: {{project_name}}, {{java_version}}, {{main_class}}
   - Prompt on existing README (skip by default)
   - Unit test: placeholder substitution, safe overwrite
   - Integrate into `jpm init`

3. **Improve Init UX (Dry-run + Validation)**
   - Add `--dry-run` flag to `jpm init`
   - Add groupId + Java version whitelist validation
   - List first 10 entries on non-empty dir; add "show more" option
   - Unit test: validation logic
   - Integrate into existing `cmd/jpm/init.go`

---
## Sprint 2: Install Assist (2-3 weeks)
JDK detection + minimal installer; Foundation for Phase 2 completion.

### Tasks
1. **Extend Java Detection**
   - File: `internal/java/detector.go`
   - Parse `java -version` output → extract major version {17,21,23}
   - Check JAVA_HOME env var
   - Unit test: version parsing across platforms (Java 17/21/23 output formats)

2. **Implement SDKMAN Installer (Linux/macOS)**
   - File: `internal/java/sdkman.go`
   - Detect platform; check if sdkman installed
   - Prompt: "Use SDKMAN to install Java 21? [Y/n]"
   - Execute: `curl https://get.sdkman.io | bash && sdk install java 21.0.1-tem`
   - Verify: run `java -version` post-install
   - Unit test: mock shell execution

3. **Implement Homebrew Hint (macOS)**
   - File: `internal/java/homebrew.go`
   - Detect macOS; offer: `brew install temurin@21` or `openjdk@21`
   - Copy-paste command; no automation (user copy-pastes into terminal)
   - Unit test: command generation

4. **Implement Manual Path Entry**
   - File: `internal/java/manual_path.go`
   - Prompt: "Enter path to Java installation (e.g., /usr/libexec/java_home)"
   - Validate: check `<path>/bin/java` executable; run `java -version`
   - Retry loop: max 3 attempts
   - Unit test: path validation

5. **Integrate into Init Flow**
   - Update `cmd/jpm/init.go`: call detector early; on fail, offer installer options
   - Store outcome (Installed | Reused | Skipped) for audit
   - Print summary + next steps if Java installed

---
## Sprint 3: Resolver Core (3-4 weeks)
Foundation for native engine; critical path.

### Tasks
1. **Define Resolver Interfaces & Models**
   - File: `internal/engine/resolver/model.go`
   - Graph, Node, ExclusionRule, ResolutionMeta (per RESOLVER_IMPLEMENTATION.md)
   - Unit test: struct initialization

2. **Implement POM Parser**
   - File: `internal/engine/resolver/pom_parser.go`
   - XML unmarshalling into POMModel
   - Handle parent resolution (fetch parent POM recursively)
   - Property substitution (${...})
   - Unit test: fixture POMs (simple, with parent, with properties)

3. **Implement Resolver (BFS + Conflict Resolution)**
   - File: `internal/engine/resolver/resolver.go`
   - Queue-based BFS traversal
   - Nearest-wins conflict logic
   - Cycle detection (DFS)
   - Memoization (cache parsed POMs + resolved nodes)
   - Unit test: simple graph, conflicts, cycles, exclusions

4. **Integrate with Fetcher Interface**
   - Resolver calls fetcher.FetchPOM()
   - Handle 404 (unresolved), network errors (retry via fetcher)
   - Unit test: mock fetcher

---
## Sprint 4: Fetcher + Cache (3-4 weeks)
Data pipeline; parallel critical.

### Tasks
1. **Define Fetcher Interface & Cache Layout**
   - File: `internal/engine/fetcher/model.go`
   - Cache directory structure; CacheEntry, CacheIndex
   - Unit test: struct serialization

2. **Implement Cache Index Management**
   - File: `internal/engine/fetcher/cache.go`
   - Load/save .jpm/cache/index.json
   - Lookup by GAV + SHA256
   - Unit test: index load/save round-trip

3. **Implement Artifact Fetcher**
   - File: `internal/engine/fetcher/fetcher.go`
   - HTTP GET with timeout, retry backoff (1s, 2s, 4s)
   - HTTPS enforcement
   - Compute SHA256 on-the-fly
   - Store to .jpm/cache/artifacts/<sha256>/
   - Unit test: cache hit/miss, retry, checksum verify

4. **Implement Offline Mode**
   - Cache-only lookup; collect missing; fail with list
   - Unit test: offline success + offline failure scenarios

5. **Implement Parallel Fetch**
   - Worker pool (concurrency = min(8, CPUs*2))
   - Queue-based distribution
   - Thread-safe locking for concurrent cache writes
   - Unit test: multiple workers, duplicate GAV handling

---
## Sprint 5: Compiler + Packager (2-3 weeks)
Build pipeline; determinism critical.

### Tasks
1. **Implement Compiler**
   - File: `internal/engine/build/compiler.go`
   - Discover .java files in src/
   - Invoke javac with classpath
   - Parse warnings/errors from javac output
   - Unit test: simple compile, error capture, argument chunking

2. **Implement Packager**
   - File: `internal/engine/build/packager.go`
   - Create deterministic JAR (sorted entries, zeroed timestamps)
   - Embed manifest with Main-Class (if known)
   - Unit test: byte-identical on repeat, manifest correctness

3. **Implement Classpath Builder**
   - File: `internal/engine/build/classpath.go`
   - ForCompile: {compile, provided}
   - ForRuntime: {compile} - {provided}
   - Map GAV -> cache path
   - Unit test: scope filtering, OS path separator

4. **Implement Main Class Detector**
   - File: `internal/engine/build/main_class.go`
   - Scan compiled classes for `public static void main`
   - Unit test: detection logic, ambiguous case

---
## Sprint 6: Lockfile + Integration (2-3 weeks)
Determinism & reproducibility.

### Tasks
1. **Implement Lockfile Manager**
   - File: `internal/engine/lock/lockfile.go`
   - Load/save jpm.lock.yaml
   - Deterministic sorting (by GAV)
   - Unit test: round-trip, sorting

2. **Integrate Resolver → Lockfile**
   - After resolution, generate lockfile with checksums
   - Unit test: lockfile generation from graph

3. **Integrate Fetcher → Lockfile Verification**
   - On fetch, verify checksum vs lockfile; re-fetch on mismatch
   - Unit test: checksum match/mismatch scenarios

4. **End-to-End Test: Resolve + Fetch + Compile + Package**
   - Create temp project with 1-2 real deps
   - Run full pipeline; verify output JAR
   - Integration test: CLI `jpm build` using native engine (flag or auto)

---
## Sprint 7: CLI Enhancements (2 weeks)
User-facing commands.

### Tasks
1. **Implement `jpm deps tree`**
   - File: `cmd/jpm/deps_tree.go` (or extend existing)
   - Render graph with indentation, scopes, optional markers
   - Unit test: tree rendering

2. **Implement `jpm deps sync`**
   - File: `cmd/jpm/deps_sync.go`
   - Compare manifest vs lockfile; report diff (added/removed/changed)
   - Preview before write
   - Unit test: diff output

3. **Add Global Flags: --offline, --verbose**
   - Propagate through CLI to engine
   - Unit test: flag parsing

4. **Integration: Build with Native Engine**
   - Update `cmd/jpm/build.go` to optionally use native engine
   - Flag: `--engine native` or auto-detect from manifest (future)
   - Fallback to Maven if native not ready
   - Integration test: build with native

---
## Sprint 8: Testing & CI (1-2 weeks)
Validation across platforms.

### Tasks
1. **Unit Test Suite**
   - All components from Sprints 1-7
   - Target ≥80% coverage for resolver/fetcher/build
   - Run locally: `go test ./...`

2. **Integration Tests**
   - Real deps (slf4j, guava) from Maven Central
   - Offline build with primed cache
   - Conflict resolution scenarios
   - Lockfile reuse + determinism

3. **GitHub Actions CI**
   - Matrix: ubuntu-latest, macos-latest, windows-latest
   - Java: 17, 21
   - Jobs: unit → integration → e2e smoke
   - Artifact: cache .jpm/ between runs (optional)

4. **E2E Smoke Tests**
   - `jpm init` (with installers if applicable)
   - `jpm deps add org.slf4j:slf4j-api@2.0.16`
   - `jpm build`
   - `jpm run`
   - All commands succeed; output sensible

---
## Sprint 9: Polish & Documentation (1 week)
Hardening & user guides.

### Tasks
1. **Error Messages**
   - Standardize error taxonomy (network, resolution, integrity, compile)
   - Clear remediation suggestions
   - Verbose mode output

2. **Performance Benchmarks**
   - Cold vs warm resolve timings
   - Concurrency scalability curve
   - Memory profiling for large graphs

3. **Update User Docs**
   - README.md: add native engine, offline, lockfile sections
   - INIT_PROTOTYPE.md: Phase 2 completion notes
   - Migration guide: Maven → native (future)

---
## Risk & Mitigation
- **Resolver complexity**: Keep BOM/import out of Sprint 3; feature-flag for Sprint 3+
- **Network flakiness**: Mock in unit tests; allow retries in integration tests
- **Windows path issues**: Symlink fallback already tested; focus on compiler/packager

---
## Definition of Done (Per Sprint)
- ✓ Code complete + reviewed
- ✓ Unit tests pass locally (race detector on)
- ✓ Integration tests on CI (all 3 platforms, Java 21)
- ✓ No new linting errors
- ✓ Docs updated if user-facing
- ✓ Backward compatible or feature-flagged

---
## Suggested Start
1. **Immediate (this week)**: Sprint 1 (generators + init polish) — fastest path to user value
2. **Next (weeks 2-3)**: Sprint 2 (install assist) + Sprint 3 (resolver) in parallel
3. **Follow-up (weeks 4-6)**: Sprints 4, 5, 6 (critical path for MVP)
4. **Final (weeks 7-9)**: Sprints 7, 8, 9 (hardening + release)

Total: ~9 weeks for full Phase 2 + Native Engine MVP. Can parallelize Sprints 2 & 3.
