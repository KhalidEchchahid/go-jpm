# JPM Master Sprint Plan - Phases 2-3 (Nov 2025)

**Date**: 2025-11-09T16:47:00Z  
**Status**: Ready for Execution  
**Audience**: Core Team, Contributors

---

## Executive Summary

This document is the single source of truth for all remaining work on JPM:
- **Phase 2**: Polish init UX, native engine foundation, and generators
- **Phase 3**: Full native engine implementation (resolver, fetcher, compiler, packager)
- **Total Sprints**: 9 (3 weeks per sprint avg)
- **Target**: MVP native build without Maven dependency by end of Phase 3

**Key Decision**: jpm.yaml is the single source of truth. pom.xml is generated (Phase 2), then phased out (Phase 3 with native engine).

---

## Part 1: Current Implementation Status

### ✅ Phase 1 (Delivered)
- [x] jpm.yaml manifest (YAML, not JSON)
- [x] Engine abstraction layer (pluggable: maven, native, gradle)
- [x] CLI commands: init, build, run, deps add/ls
- [x] Maven integration (pom.xml rendering, build delegation)
- [x] Prompt-first UX (no flags required)
- [x] Symlink scaffold (.jpm/maven/src/main/java → ./src)

### ✅ Sprint 1 / Phase 2 (Delivered)
- [x] .gitignore generator (idempotent, appends patterns)
- [x] README.md generator (template-based, placeholders)
- [x] 5 unit tests (100% passing, zero lint errors)
- [x] Code ready for integration

### ⏳ Remaining Work
- **Phase 2 Sprints 2-5**: Install assist, init polish, dependency fetching, build core
- **Phase 3 Sprints 6-9**: Lockfile, CLI enhancements, testing, hardening

---

## Part 2: Architecture Overview

### Current Tech Stack
```
User CLI (Cobra)
    ↓
Manifest Loader (YAML)
    ↓
Engine Router [maven | native]
    ├─ Maven Engine (Phase 1-2)
    │  └─ Delegates to: mvn compile package
    │
    └─ Native Engine (Phase 3)
       ├─ Resolver (graph building, conflict resolution)
       ├─ Fetcher (cache, parallel download)
       ├─ Compiler (javac invocation)
       ├─ Packager (JAR creation)
       └─ Classpath (scope filtering)
```

### Why No More pom.xml Post-Phase 2?
- **Current (Phase 2)**: pom.xml is generated from jpm.yaml as an intermediate for Maven
- **After Phase 2 (Phase 3)**: Native engine uses jpm.yaml directly; pom.xml deprecated
- **User sees**: Only jpm.yaml (plus jpm.lock.yaml for determinism)
- **Rationale**: Remove Maven dependency, speed up builds, simplify toolchain

---

## Part 3: Comprehensive Sprint Breakdown

### Sprint 1: Generators + Init Polish ✅ COMPLETE
**Duration**: 1 week (delivered)  
**Status**: Code + tests complete, ready for integration

#### Deliverables
1. **GitignoreGenerator** (internal/init/gitignore.go)
   - Creates .gitignore from scratch
   - Idempotently appends missing JPM patterns
   - Preserves user content
   
2. **ReadmeGenerator** (internal/init/readme.go)
   - Template-based with {{project_name}} placeholders
   - Sections: Quickstart, Structure, Dependencies, Troubleshooting
   - Prompts on existing file (skip default)

3. **Test Suite** (internal/init/gitignore_test.go)
   - 5 unit tests, 100% passing
   - Coverage: create new, idempotence, user content preservation

#### Integration Point
```go
// In cmd/jpm/init.go after scaffold
gitignoreGen := init.NewGitignoreGenerator(absDir)
gitignoreGen.Generate()

readmeGen := init.NewReadmeGenerator(absDir, projectName, javaVersion, "Main")
readmeGen.Generate(false) // false = skip if exists
```

---

### Sprint 2: Java Detection & Install Assist
**Duration**: 2-3 weeks  
**Depends On**: Sprint 1 complete  
**Owner**: CLI + Environment

#### Tasks

##### 1. Java Detection Core
**File**: `internal/java/detector.go`
- Detect JDK on PATH
- Check JAVA_HOME environment variable
- Parse `java -version` → extract major version {17, 21, 23}
- **Tests**: Mock output from Java 17/21/23 across platforms
- **Acceptance**: Detects all versions correctly; cross-platform parsing

##### 2. SDKMAN Integration (Linux/macOS)
**File**: `internal/java/sdkman.go`
- Detect if SDKMAN installed
- Prompt: "Use SDKMAN to install Java 21? [Y/n]"
- Execute: `curl https://get.sdkman.io | bash && sdk install java 21.0.1-tem`
- Verify: Run `java -version` post-install
- **Tests**: Mock shell execution; verify command construction

##### 3. Homebrew Hint (macOS)
**File**: `internal/java/homebrew.go`
- Detect macOS
- Offer: `brew install temurin@21` or `openjdk@21` (copy-paste command)
- **Tests**: Command generation

##### 4. Manual Path Entry
**File**: `internal/java/manual_path.go`
- Prompt: "Enter path to Java installation"
- Validate: `<path>/bin/java` exists + executable
- Run `java -version` to extract version
- Retry loop: max 3 attempts
- **Tests**: Path validation logic

##### 5. Init Flow Integration
**Update**: `cmd/jpm/init.go`
- Call detector early
- On fail: offer installer options (SDKMAN → Homebrew → Manual → Skip)
- Store outcome (Installed | Reused | Skipped)
- Print summary + next steps if Java installed

#### Acceptance Criteria
- [ ] `jpm init` detects Java on PATH/JAVA_HOME
- [ ] Offers SDKMAN install on Linux/macOS
- [ ] Falls back to manual entry
- [ ] Allows skip with clear next steps
- [ ] All platform variants tested

---

### Sprint 3: Resolver Core (Dependency Graph)
**Duration**: 3-4 weeks  
**Depends On**: Sprint 2 complete  
**Owner**: Engine + Core  
**Criticality**: CRITICAL PATH

#### Tasks

##### 1. Define Resolver Interfaces & Models
**File**: `internal/engine/resolver/model.go`
```go
type Dependency struct {
    Group, Artifact, Version string
    Scope string  // compile, test
    Optional bool
    Classifier string
    Exclusions []Exclusion
}

type Node struct {
    Coord Dependency
    Children []Node
    Metadata ResolutionMeta
}

type Graph struct {
    Root []*Node
    Nodes map[string]*Node
}

type ResolutionMeta struct {
    ResolvedVersion string
    Source string // "manifest" | "pom"
    Distance int
}
```

##### 2. POM Parser
**File**: `internal/engine/resolver/pom_parser.go`
- XML unmarshalling → POMModel
- Handle parent resolution (fetch parent POM recursively)
- Property substitution `${...}` expansion
- Extract: dependencies[], dependencyManagement[], exclusions[]
- **Tests**: Simple POM, with parent, with properties, missing parent

##### 3. Resolver Implementation (BFS + Conflict Resolution)
**File**: `internal/engine/resolver/resolver.go`
- **Algorithm**: Breadth-first traversal
- **Conflict**: Nearest-wins (direct > transitive)
- **Cycles**: DFS detection + error report
- **Memoization**: Cache parsed POMs + resolved nodes
- **Output**: ResolvedSet (GAV list with scopes)
- **Tests**: Simple graph, conflicts, cycles, exclusions

##### 4. Fetcher Interface Integration
**File**: `internal/engine/resolver/fetcher_interface.go`
- Resolver calls `Fetcher.FetchPOM(GAV)` and `Fetcher.FetchArtifact(GAV)`
- Handle network errors → retry via fetcher
- Handle 404 → mark unresolved; continue
- **Tests**: Mock fetcher

#### Acceptance Criteria
- [ ] Parse manifest dependencies into internal form
- [ ] Resolve transitive deps (no BOMs yet; gated to later)
- [ ] Detect and report cycles
- [ ] Produce deterministic ResolvedSet
- [ ] All unit tests pass

---

### Sprint 4: Fetcher + Cache
**Duration**: 3-4 weeks  
**Depends On**: Sprint 3 complete  
**Owner**: Engine + Network

#### Tasks

##### 1. Cache Layout & Index
**File**: `internal/engine/fetcher/model.go`
```
.jpm/cache/
├── artifacts/
│   ├── <sha256>/
│   │   ├── artifact.jar
│   │   └── artifact.pom
│   └── index.json  # GAV → sha256 mapping
└── metadata.json
```

**File**: `internal/engine/fetcher/cache.go`
- Load/save `.jpm/cache/index.json`
- Lookup by GAV → SHA256 → artifact path
- **Tests**: Index round-trip, lookup, concurrent access

##### 2. Artifact Fetcher
**File**: `internal/engine/fetcher/fetcher.go`
- HTTP GET (HTTPS only; reject http://)
- Timeout: 30s per request
- Retry backoff: 1s, 2s, 4s (max 3 attempts)
- Compute SHA-256 on-the-fly
- Store to `.jpm/cache/artifacts/<sha256>/`
- **Tests**: Cache hit/miss, retry, checksum verify

##### 3. Parallel Fetch Worker Pool
**File**: `internal/engine/fetcher/parallel.go`
- Concurrency: `min(8, CPUs*2)`
- Queue-based work distribution
- Thread-safe cache writes (mutex per artifact)
- **Tests**: Multiple workers, duplicate GAV handling

##### 4. Offline Mode
**File**: `internal/engine/fetcher/offline.go`
- Cache-only lookup; no network
- Collect missing artifacts
- Fail with actionable list (e.g., "Run `jpm build` online first")
- **Tests**: Offline success, offline failure

#### Acceptance Criteria
- [ ] Artifact fetch works with retry
- [ ] Cache layout implemented with SHA-256 keying
- [ ] Parallel fetch with worker pool
- [ ] Offline mode works when cache primed
- [ ] All checksums computed and verified

---

### Sprint 5: Compiler + Packager + Build Pipeline
**Duration**: 2-3 weeks  
**Depends On**: Sprint 4 complete  
**Owner**: Engine + Build

#### Tasks

##### 1. Compiler
**File**: `internal/engine/build/compiler.go`
- Discover `.java` files in `src/`
- Invoke `javac` with resolved classpath
- Output: `.jpm/work/classes/`
- Parse `javac` stderr for warnings/errors
- **Tests**: Simple compile, error capture

##### 2. Packager
**File**: `internal/engine/build/packager.go`
- Create deterministic JAR (sorted entries, zeroed timestamps)
- Embed manifest with `Main-Class` (if known)
- Output: `.jpm/out/<artifact>-<version>.jar`
- **Tests**: Byte-identical on repeat, manifest correctness

##### 3. Classpath Builder
**File**: `internal/engine/build/classpath.go`
- ForCompile: scopes {compile, provided}
- ForRuntime: {compile} - {provided}
- Map GAV → cache artifact path
- OS-specific separators (`:` on Unix, `;` on Windows)
- **Tests**: Scope filtering, path construction

##### 4. Main Class Detector
**File**: `internal/engine/build/main_class.go`
- Scan compiled `.class` files for `public static void main(String[])`
- Return class name (e.g., `app.Main`)
- Handle no match gracefully (require user prompt)
- **Tests**: Detection logic, ambiguous case

##### 5. Build Pipeline Orchestration
**File**: `internal/engine/build/builder.go`
- Coordinate: Resolve → Fetch → Compile → Package → Main-detect
- Log each step to `.jpm/logs/build-<timestamp>.log`
- **Tests**: End-to-end build with simple project

#### Acceptance Criteria
- [ ] javac invocation works with classpath
- [ ] JAR creation is deterministic
- [ ] Main class auto-detected when possible
- [ ] Logs written to .jpm/logs
- [ ] Native build complete for simple projects (0-N deps)

---

### Sprint 6: Lockfile & Reproducibility
**Duration**: 2-3 weeks  
**Depends On**: Sprints 3-5 complete  
**Owner**: Engine + Determinism

#### Tasks

##### 1. Lockfile Spec Implementation
**File**: `internal/engine/lock/lockfile.go`
- Load/save `jpm.lock.yaml`
- Format (see LOCKFILE_SPEC.md):
  ```yaml
  version: "1"
  timestamp: "2025-11-09T16:47:00Z"
  tool_version: "1.0.0"
  dependencies:
    - group: org.slf4j
      artifact: slf4j-api
      version: "2.0.16"
      sha256: "abc123..."
      source: "https://repo1.maven.org/maven2"
  ```
- Deterministic sorting (by groupId:artifactId)
- **Tests**: Round-trip, sorting consistency

##### 2. Integrate Resolver → Lockfile
- After resolution, generate lockfile
- Include checksums from fetched artifacts
- **Tests**: Lockfile generation from graph

##### 3. Integrate Fetcher → Lockfile Verification
- On fetch, verify checksum vs lockfile
- Re-fetch on mismatch + log warning
- **Tests**: Checksum match/mismatch

##### 4. End-to-End Integration Test
- Temp project with 1-2 real deps (e.g., slf4j + guava)
- Full pipeline: resolve → fetch → compile → package
- Verify output JAR exists + is valid
- **Tests**: Integration scenario with real deps

#### Acceptance Criteria
- [ ] Lockfile created on first build
- [ ] Lockfile reused on second build (no refetch)
- [ ] Deterministic (same manifest → same lockfile)
- [ ] Checksums verified
- [ ] End-to-end build works

---

### Sprint 7: CLI Enhancements (deps tree/sync, verbose, offline)
**Duration**: 2 weeks  
**Depends On**: Sprint 6 complete  
**Owner**: CLI

#### Tasks

##### 1. Implement `jpm deps tree`
**File**: `cmd/jpm/deps_tree.go`
- Render dependency graph with indentation
- Show scopes (compile, test, provided)
- Mark optional deps with `*`
- Example:
  ```
  org.slf4j:slf4j-api:2.0.16 [compile]
  ├─ ...transitive deps...
  └─ com.google.guava:guava:33.0.0-jre [compile]
     └─ ...
  ```
- **Tests**: Tree rendering

##### 2. Implement `jpm deps sync`
**File**: `cmd/jpm/deps_sync.go`
- Compare manifest vs lockfile
- Report diff: added, removed, version changed
- Preview before write
- **Tests**: Diff output

##### 3. Add Global Flags
- `--offline`: Use cache only
- `--verbose`: Show engine logs + executed commands
- Propagate through CLI to engine context
- **Tests**: Flag parsing + propagation

##### 4. Build with Native Engine (Optional)
**Update**: `cmd/jpm/build.go`
- Add `--engine native` flag (or future auto-detect from manifest)
- Default: maven (fallback)
- Future: auto-detect from `manifest.engine`
- **Tests**: Build with native engine

#### Acceptance Criteria
- [ ] `jpm deps tree` shows graph correctly
- [ ] `jpm deps sync` reports differences
- [ ] `--offline` flag prevents network access
- [ ] `--verbose` shows engine internals
- [ ] Native engine can be opted in

---

### Sprint 8: Testing & CI Matrix
**Duration**: 1-2 weeks  
**Depends On**: All components complete  
**Owner**: QA + CI

#### Tasks

##### 1. Unit Test Suite
- All components from Sprints 2-7
- Target ≥80% coverage for resolver/fetcher/build
- Local test: `go test ./...`
- **Tests**: Already specified per sprint

##### 2. Integration Tests
- Real deps from Maven Central (slf4j, guava, etc.)
- Offline build with primed cache
- Conflict resolution scenarios
- Lockfile reuse + determinism check
- **Tests**: Full build → run pipeline

##### 3. GitHub Actions CI Matrix
```yaml
strategy:
  matrix:
    os: [ubuntu-latest, macos-latest, windows-latest]
    java: ['17', '21']
jobs:
  - unit-tests
  - integration-tests
  - e2e-smoke
```

##### 4. E2E Smoke Tests
```bash
jpm init my-app
cd my-app
jpm build
jpm run
```
- Should complete without errors
- Output sensible and predictable

#### Acceptance Criteria
- [ ] Unit tests pass locally (race detector on)
- [ ] Integration tests on CI (3 OS × 2 Java versions)
- [ ] ≥80% coverage for critical paths
- [ ] No flaky tests
- [ ] E2E smoke test passes

---

### Sprint 9: Polish, Docs & Release
**Duration**: 1 week  
**Depends On**: Sprints 1-8 complete  
**Owner**: Docs + QA

#### Tasks

##### 1. Error Messages & Remediation
- Standardize error taxonomy:
  - Network errors (retry, offline fallback)
  - Resolution errors (user fix manifest)
  - Integrity errors (checksum mismatch, clear cache)
  - Compile errors (user fix code)
- Add actionable hints to all errors

##### 2. Performance Benchmarks
- Cold vs warm resolve timings
- Concurrency scalability (workers vs latency)
- Memory profiling for large graphs
- Document baseline expectations

##### 3. Documentation Updates
- Update README.md:
  - Native engine section
  - Offline builds + caching
  - Lockfile explanation
  - Troubleshooting
- Update INIT_PROTOTYPE.md with Phase 2-3 completion
- Add NATIVE_ENGINE.md (architecture overview)

##### 4. Migration Guide (Future)
- Document path from Maven pom.xml → native jpm
- For teams migrating existing projects

#### Acceptance Criteria
- [ ] All error messages clear + actionable
- [ ] Performance benchmarks documented
- [ ] User docs complete
- [ ] Ready for public release

---

## Part 4: Phase 2 vs Phase 3 Clarification

### Phase 2 (Sprints 1-5): Hybrid Approach
- **Manifest**: jpm.yaml (single source of truth)
- **Build Tool**: Maven (hidden in .jpm/)
- **Generators**: .gitignore, README
- **Install Assist**: JDK detection + helpers
- **pom.xml**: Generated from jpm.yaml (intermediate)
- **Status**: Stable, MVP ready

### Phase 3 (Sprints 6-9): Pure Native
- **Manifest**: jpm.yaml (unchanged)
- **Build Tool**: Native engine (javac + jar)
- **Lockfile**: jpm.lock.yaml (reproducibility)
- **pom.xml**: Deprecated (not used)
- **Status**: Full self-contained, no Maven dependency

---

## Part 5: Dependency Fetching Strategy (Optimized)

### Overview
The fetcher is the critical path for performance. Here's the optimal strategy:

### Step 1: Pre-Scan Root Dependencies
- Parse manifest → extract root dependencies
- Group by repository (default: Maven Central)
- Seed work queue

### Step 2: Iterative Graph Expansion (BFS)
- Process queue in breadth-first order
- Favors nearer dependencies (conflict resolution alignment)
- Each item: fetch POM → parse → enqueue children

### Step 3: Memoization
- Cache parsed POMs (avoid re-parsing)
- Cache resolved nodes (avoid re-resolution)
- Store parent + dependencyManagement info

### Step 4: Concurrency
- Worker pool: `min(8, CPUs * 2)` workers
- Each worker: fetch + parse independently
- Futures for dependent nodes (wait for parent before resolving children)

### Step 5: Conflict Resolution
- Nearest-wins: direct dep wins over transitive
- Document override rules (future: manifest config)

### Step 6: Lockfile Determinism
- Sort entries by `groupId:artifactId` before write
- UTC timestamps
- Stable SHA-256 keying

### Step 7: Checksums
- Compute SHA-256 on first fetch
- Store in lockfile
- Verify on reuse (cache hit + integrity)

### Step 8: Failure Handling
- Artifact fetch fails → mark node unresolved
- Continue processing (don't block)
- Surface aggregated warnings at end

### Step 9: Offline Mode
- Skip all network
- Mark missing nodes
- Fail fast if root dependency absent in cache

### Step 10: Cache Keying & Relocations
- Key by SHA-256
- Maintain GAV → SHA map in `.jpm/cache/index.json`
- Allows relocations / mirror strategies (future)

### Step 11: Retry Policy
- Network timeouts: exponential backoff (1s, 2s, 4s)
- Max 3 attempts per artifact
- Checksum mismatch: single redownload + invalidate cache

### Step 12: Metrics
- Capture durations per phase:
  - Resolve time
  - Fetch time (total bytes, cache hit rate)
  - Compile time
  - Package time
- Log to `.jpm/logs/build-<timestamp>.log`

---

## Part 6: .gitignore Generator Details

### Rules (Ordered)
```
# JPM build artifacts
.jpm/out/
.jpm/logs/
.jpm/cache/

# Maven/Gradle remnants
target/

# IDE/Editor metadata
.idea/
.classpath
.settings/
.project
*.iml
.vscode/
```

### Behavior
1. **If .gitignore absent**: Create with template
2. **If present**: Append missing lines idempotently
3. **Preserve user spacing**: Don't reformat existing content
4. **No duplicates**: Check before append

### Tests
- Run generator twice → lines appear once
- User content unchanged
- New file correct format

---

## Part 7: README Generator Template

### Sections
1. **Project Name & Description**
   - Placeholder: `{{project_name}}`
   - User-editable

2. **Quickstart**
   ```
   jpm build
   jpm run
   jpm deps add group:artifact@version
   ```

3. **Structure Overview**
   ```
   src/          # Your Java source
   .jpm/         # Build artifacts (don't commit)
   jpm.yaml      # Project manifest
   jpm.lock.yaml # Locked dependencies
   ```

4. **Running**
   - Setting main_class
   - Passing arguments
   - Troubleshooting

5. **Dependencies**
   - Adding deps: `jpm deps add`
   - Viewing tree: `jpm deps tree`
   - Offline builds

6. **Troubleshooting**
   - Java not found
   - Windows symlink fallback
   - Checksum mismatch

---

## Part 8: Risk Mitigation

### Risk: Resolver Complexity Creep
**Mitigation**: Gate BOM/import management to Sprint 3+. Start with simple nearest-wins.

### Risk: Network Flakiness
**Mitigation**: Retry with exponential backoff. Mock in unit tests. Allow retry in integration tests.

### Risk: Windows Path Issues
**Mitigation**: Fallback to junctions + directory copy. Test on Windows runner. Document implications.

### Risk: Large Dependency Graphs
**Mitigation**: Streaming graph build + memory profiling early. Bounded concurrency (no unbounded worker threads).

### Risk: Lockfile Non-Determinism
**Mitigation**: Sort entries by GAV. UTC timestamps. Stable hashing. Test determinism (same manifest → same lockfile).

---

## Part 9: Current Blockers & How to Unblock

### Question: Why Still Use pom.xml?

**Current State (Phase 2)**:
- jpm.yaml is source of truth
- pom.xml is **generated** from jpm.yaml
- Maven uses pom.xml to build
- Only intermediate; not user-edited

**Why (Phase 2)**:
- Maven is battle-tested (stable)
- Allows gradual transition to native engine
- Risk mitigation (fallback if native issues)

**When Removed (Phase 3)**:
- Native engine ready (sprints 3-6)
- Replaces Maven entirely
- jpm.yaml → Resolver → Fetcher → Compiler → Jar
- No pom.xml needed

**User Experience**:
- Phase 2: Users never see pom.xml (hidden in .jpm/)
- Phase 3: pom.xml gone entirely

### Action: Phase Out pom.xml References
**When**: After Sprint 5 (end of Phase 2)
- Remove pom.xml generation from build.go
- Switch to native engine by default
- Keep Maven engine as fallback (feature flag)
- Document migration

---

## Part 10: How to Execute This Plan

### Week 1 (Sprint 1 - Already Done)
- ✅ Generators implemented + tested
- ⏳ Integrate into init.go

### Week 2-3 (Sprint 2)
- Java detection
- SDKMAN/Homebrew/Manual installers
- Init integration

### Week 4-6 (Sprint 3)
- Resolver core + POM parser
- Graph building + conflict resolution
- Lockfile seeding

### Week 7-10 (Sprints 4-5)
- Fetcher + cache + parallel
- Compiler + packager + build pipeline

### Week 11-12 (Sprint 6)
- Lockfile reproducibility
- End-to-end integration

### Week 13-14 (Sprint 7)
- deps tree, deps sync, offline, verbose
- Native engine opt-in

### Week 15 (Sprint 8)
- CI matrix setup
- All tests passing

### Week 16 (Sprint 9)
- Polish, docs, release prep

---

## Part 11: Acceptance Criteria (Full Project)

### MVP Native Build (End of Phase 3)
- [ ] `jpm init` works with JDK assist + generators
- [ ] `jpm build` uses native engine (no Maven)
- [ ] `jpm run` launches compiled JAR
- [ ] `jpm deps add` updates jpm.yaml + lockfile
- [ ] `jpm deps tree` shows graph
- [ ] Lockfile deterministic (same manifest → same build)
- [ ] Offline build works (cache primed)
- [ ] All tests pass (≥80% coverage)
- [ ] Windows/macOS/Linux tested
- [ ] Zero tool internals exposed (no mvn/gradle shown)

---

## Part 12: Key Files to Create/Update

### New Files (Sprints 2-9)

**Sprint 2**: Java Detection
- `internal/java/detector.go`
- `internal/java/sdkman.go`
- `internal/java/homebrew.go`
- `internal/java/manual_path.go`
- Tests for above

**Sprint 3**: Resolver
- `internal/engine/resolver/model.go`
- `internal/engine/resolver/pom_parser.go`
- `internal/engine/resolver/resolver.go`
- Tests for above

**Sprint 4**: Fetcher
- `internal/engine/fetcher/model.go`
- `internal/engine/fetcher/cache.go`
- `internal/engine/fetcher/fetcher.go`
- `internal/engine/fetcher/parallel.go`
- `internal/engine/fetcher/offline.go`
- Tests for above

**Sprint 5**: Compiler/Packager
- `internal/engine/build/compiler.go`
- `internal/engine/build/packager.go`
- `internal/engine/build/classpath.go`
- `internal/engine/build/main_class.go`
- `internal/engine/build/builder.go`
- Tests for above

**Sprint 6**: Lockfile
- `internal/engine/lock/lockfile.go`
- Tests for above

**Sprint 7**: CLI
- Update `cmd/jpm/build.go` (native engine option)
- `cmd/jpm/deps_tree.go`
- `cmd/jpm/deps_sync.go`
- Tests for above

### Updated Files

**Sprint 1-2**: Generators + Init
- `cmd/jpm/init.go` (integrate generators + install assist)
- Tests

**Sprint 7**: CLI
- `cmd/jpm/root.go` (add global flags: --offline, --verbose)

---

## Part 13: Definition of Done (Per Sprint)

All sprints follow this checklist:
- [ ] Code complete (all tasks done)
- [ ] Code reviewed + approved
- [ ] Unit tests pass locally (race detector on)
- [ ] Integration tests pass (if applicable)
- [ ] All tests pass on CI (3 OS, 2 Java versions)
- [ ] Zero lint errors (golangci-lint ./...)
- [ ] No new compiler warnings
- [ ] Documentation updated (README, INIT_PROTOTYPE.md, architecture docs)
- [ ] Backward compatible or clearly feature-flagged
- [ ] Performance acceptable (no regressions)

---

## Part 14: Success Metrics

### By End of Phase 2
- Generators integrated
- JDK assist working
- Basic build pipeline (hidden Maven)
- User experience seamless

### By End of Phase 3
- Native engine fully operational
- No Maven dependency
- Lockfile reproducible
- All tests passing on 3 OS + 2 Java versions
- Ready for public preview/beta

---

## Part 15: Questions & Answers

**Q: When do we remove telemetry?**  
A: Already removed (see DELIVERY_SUMMARY_PHASE2.md, PHASE2_NATIVE_ENGINE_PLAN.md).

**Q: When do we stop using pom.xml?**  
A: Phase 3, Sprint 5 (end of native engine implementation). Phase 2 uses it as intermediate.

**Q: How do we ensure deterministic builds?**  
A: jpm.lock.yaml with SHA-256 checksums + sorted entries + UTC timestamps.

**Q: What if network fails?**  
A: Exponential backoff + 3 retries. With `--offline`, cache-only mode. Clear error messages.

**Q: Windows support?**  
A: Symlink fallback to junctions. Tested on CI. Path separators handled.

**Q: How long for MVP?**  
A: 9 sprints (~9-12 weeks). Can parallelize Sprints 2-3.

---

## Part 16: Next Actions

### Immediate (This Week)
1. Review & approve this plan
2. Integrate Sprint 1 (generators) into init.go
3. Test full init workflow
4. Commit to feature branch

### Next Week (Sprint 2 Start)
1. Create Java detection task tickets
2. Implement detector.go + sdkman.go
3. Integrate into init flow
4. Test on all 3 platforms

### Follow-Up (Sprint 3+)
Follow PHASE2_BACKLOG_PRIORITIZED.md; each sprint has detailed tasks.

---

## Appendix: Command Reference

### User-Facing Commands (Implemented/Planned)

**Init**
```bash
jpm init              # Interactive project setup
jpm init --dry-run    # Preview without writing (Phase 2)
```

**Build**
```bash
jpm build             # Build with current engine (maven/native)
jpm build --verbose   # Show all logs
jpm build --offline   # Cache-only build
```

**Run**
```bash
jpm run               # Run main class (interactive if not set)
jpm run -- arg1 arg2  # Pass arguments (Phase 3)
```

**Dependencies**
```bash
jpm deps add org.slf4j:slf4j-api@2.0.16   # Add dependency
jpm deps ls                                 # List dependencies
jpm deps tree                               # Show graph (Phase 3)
jpm deps sync                               # Reconcile manifest vs lock (Phase 3)
```

**Global Flags**
```bash
jpm build --verbose  # Show engine logs
jpm build --offline  # Use cache only
```

---

## Appendix: Document Index

**Planning**
- MASTER_SPRINT_PLAN.md (this file)
- PHASE2_BACKLOG_PRIORITIZED.md (detailed task breakdown)
- PHASE2_NATIVE_ENGINE_PLAN.md (original Phase 2 spec)

**Architecture**
- NATIVE_ENGINE_ARCHITECTURE.md (system overview)
- RESOLVER_IMPLEMENTATION.md (graph building)
- FETCHER_IMPLEMENTATION.md (cache + download)
- COMPILER_IMPLEMENTATION.md (javac)
- PACKAGER_IMPLEMENTATION.md (JAR creation)
- CLASSPATH_IMPLEMENTATION.md (scope filtering)
- GENERATORS_IMPLEMENTATION.md (.gitignore + README)

**Specifications**
- LOCKFILE_SPEC.md (jpm.lock.yaml format)
- TESTING_STRATEGY.md (test pyramid + CI)

**Progress**
- SPRINT_PROGRESS.md (early Phase 1)
- PHASE2_SPRINT1_COMPLETION.md (generators ✅)
- DELIVERY_SUMMARY_PHASE2.md (Phase 2 overview)

---

## Sign-Off

**Status**: 🟢 READY FOR EXECUTION

**Next**: Begin Sprint 2 (Java Detection) when this plan is approved.

---

*Last Updated: 2025-11-09T16:47:00Z*
