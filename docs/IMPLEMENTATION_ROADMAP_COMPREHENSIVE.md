# JPM Comprehensive Implementation Roadmap

**Date**: 2025-11-09  
**Status**: Ready for Sprint 2 Execution  
**Version**: 1.0 Final

---

## EXECUTIVE SUMMARY

### Current State (As of Nov 9, 2025)
- ✅ **Phase 1 Prototype Complete**: `jpm init`, `jpm build`, `jpm run`, `jpm deps add/ls`
- ✅ **Manifest System**: jpm.yaml as single source of truth
- ✅ **Maven Bridge**: Hidden Maven engine for builds (mvn in .jpm/)
- ✅ **Documentation**: 11 comprehensive docs + Sprint 1 generators
- ✅ **Sprint 1 Complete**: .gitignore & README generators (no telemetry)
- ⚠️ **Current Issue**: 
  - pom.xml still explicitly used in code (should be generated from jpm.yaml)
  - DependencyGraph interface incomplete in resolver
  - Tests failing (legacy java-legacy/ references)

### Questions Answered
- ❓ **Why still using pom.xml?** → Transitional until native engine replaces Maven
- ❓ **Aren't we using jpm.yaml?** → YES, jpm.yaml is source of truth; pom.xml is derived
- ❓ **Sprint 5 completed?** → NO; Current: Sprint 1 ✅, Need: Sprints 2-9

---

## IMPLEMENTATION PHASES

### PHASE 1: PROTOTYPE & FOUNDATION (✅ COMPLETED)
**Delivered**: jpm.yaml manifest, Maven bridge engine, init/build/run/deps commands

### PHASE 2: NATIVE ENGINE FOUNDATION (🟡 IN PROGRESS)
**Focus**: JDK assist, generators, native resolver/fetcher/compiler stack

### PHASE 3+: POLISH & OPTIMIZATION
**Focus**: Performance, Windows compat, advanced features

---

## PHASE 2 SPRINT BREAKDOWN (Sprints 2-9)

### Sprint 2: Java Detection & Installation Assist (1.5 weeks)
**Goal**: Auto-detect JDK; offer SDKMAN/Homebrew/manual installation

#### Tasks:
1. **Java Detection Service**
   - `internal/java/detector.go`: DetectJava() -> (path, version, err)
   - Check PATH, JAVA_HOME, common install locations
   - Parse `java -version` output (Java 17+)
   - Unit tests with mocked `exec.Command`
   - ✅ **Acceptance**: Detect Java 21 on test system

2. **SDKMAN Integration** (Linux/macOS)
   - `internal/java/sdkman.go`: InstallViaSDKMAN()
   - Download script, execute with bash, validate
   - Show progress to user
   - ✅ **Acceptance**: Install JDK 21 in temp sandbox

3. **Homebrew Integration** (macOS)
   - `internal/java/homebrew.go`: InstallViaHomebrew()
   - Detect brew; suggest `brew install temurin`
   - ✅ **Acceptance**: Suggest correct command

4. **Windows Hints**
   - `internal/java/windows.go`: SuggestWindowsInstall()
   - Offer winget/choco commands
   - ✅ **Acceptance**: Display commands to user

5. **Manual Path Entry**
   - Prompt user for JDK path
   - Validate `<path>/bin/java -version`
   - ✅ **Acceptance**: Accept valid path, reject invalid

6. **Init Flow Integration**
   - Update `cmd/jpm/init.go` to call detection
   - Prompt for install if missing
   - Show summary before confirming
   - ✅ **Acceptance**: Full interactive flow works

**Deliverables**: 5 new files (500 lines), updated init.go, 15+ unit tests  
**Risks**: Platform variation; test Windows locally or in CI  
**Dependencies**: None (Go stdlib only)

---

### Sprint 3: Native Resolver Foundation (2 weeks)
**Goal**: Build dependency graph from jpm.yaml; fetch POMs; resolve versions

#### Tasks:
1. **Manifest-to-Dependency Parser**
   - `internal/resolver/parser.go`: ParseManifestDependencies()
   - Convert manifest entries → internal Dependency model
   - Validate group, artifact, version
   - Unit tests with YAML fixtures
   - ✅ **Acceptance**: Parse sample jpm.yaml with 5 deps

2. **Dependency Graph Model**
   - `internal/resolver/graph.go`: Graph, Node, ResolutionMeta types
   - BFS traversal logic
   - Duplicate node detection (memoization)
   - Export to DOT format for visualization (nice-to-have)
   - Unit tests
   - ✅ **Acceptance**: Build graph with 10 nodes (simple deps)

3. **POM Fetcher & Parser**
   - `internal/resolver/pom_fetcher.go`: FetchPOM(group, artifact, version)
   - HTTPS-only to Maven Central
   - Parse dependencies, exclusions, optional flags, parent
   - Cache parsed POMs in memory during session
   - Unit tests with mock HTTP
   - ✅ **Acceptance**: Parse pom-with-parent + nested exclusions

4. **Version Resolution Strategy**
   - `internal/resolver/version_resolver.go`: ResolveVersion()
   - Direct version selection (manifest wins)
   - Nearest-wins for transitive (BFS order)
   - Document policy in code comments
   - Unit tests with conflict scenarios
   - ✅ **Acceptance**: Resolve known conflict (log4j/slf4j)

5. **Cycle Detection**
   - `internal/resolver/cycles.go`: DetectCycles()
   - DFS-based cycle finder
   - Return path to cycle for error reporting
   - Unit tests
   - ✅ **Acceptance**: Detect and report sample cycle

6. **Full Resolution Pipeline**
   - `internal/resolver/resolver.go`: Resolve(manifest) -> (graph, resolved[]Dependency, err)
   - Integrate all above components
   - Measure performance (target: <1s for 50-dep tree)
   - Integration tests
   - ✅ **Acceptance**: Resolve guava + transitive deps successfully

**Deliverables**: 6 files (1200 lines), 40+ tests, performance benchmarks  
**Risks**: POM parsing complexity (handle legacy XML quirks); network mocking  
**Dependencies**: Testing fixtures with real POMs (commit sample POMs)

---

### Sprint 4: Dependency Fetcher & Caching (1.5 weeks)
**Goal**: Download artifacts; manage cache; verify checksums

#### Tasks:
1. **Cache Layout & Indexing**
   - `internal/cache/layout.go`: CacheLayout type
   - `.jpm/cache/artifacts/<sha256>.<ext>` structure
   - Index file mapping GAV → sha256
   - Concurrent access safety (RWMutex)
   - Unit tests
   - ✅ **Acceptance**: Index 100 artifacts without conflicts

2. **Network Fetcher**
   - `internal/cache/fetcher.go`: Fetch(group, artifact, version, kind[JAR|POM])
   - HTTPS-only to Maven Central (configurable in future)
   - Retry logic: exponential backoff (max 3 attempts)
   - Partial download handling (.part files)
   - SHA-256 computation on download
   - Unit tests with mock HTTP
   - ✅ **Acceptance**: Fetch + verify checksum for guava JAR

3. **Offline Mode**
   - `internal/cache/offline.go`: OfflineMode flag + behavior
   - Skip network; error if cache miss
   - Actionable error messages
   - Unit tests
   - ✅ **Acceptance**: Offline build with primed cache succeeds

4. **Metrics & Reporting**
   - `internal/cache/metrics.go`: Track hits, misses, bytes, duration
   - Report to user in build summary
   - ✅ **Acceptance**: Print "Cache: 3 hits, 1 miss (1.2 MB) in 2.1s"

5. **Integration with Resolver**
   - Update resolver to use fetcher
   - Parallel fetch (worker pool, default: min(8, CPU*2))
   - ✅ **Acceptance**: Fetch 10 POMs concurrently

**Deliverables**: 4 files (800 lines), 30+ tests, performance benchmarks  
**Risks**: Concurrent cache access; network flakiness  
**Dependencies**: Sprint 3 resolver

---

### Sprint 5: Lockfile Generation & Management (1 week)
**Goal**: Create deterministic jpm.lock.yaml; implement `jpm deps sync`

#### Tasks:
1. **Lockfile Spec Implementation**
   - `internal/lockfile/writer.go`: WriteLockfile(resolved[], path)
   - jpm.lock.yaml format: sorted by GAV, includes SHA256, timestamp
   - Deterministic (same manifest → same lockfile)
   - Unit tests
   - ✅ **Acceptance**: Lockfile determinism: run 10x, MD5 same

2. **Lockfile Parser**
   - `internal/lockfile/reader.go`: ReadLockfile() -> []ResolvedDep
   - Parse YAML; validate format
   - Unit tests
   - ✅ **Acceptance**: Read generated lockfile back

3. **Diff & Sync**
   - `internal/lockfile/diff.go`: CompareLockfiles() -> diff with Added/Removed/Updated
   - Human-readable diff printing
   - ✅ **Acceptance**: Pretty-print "Added: guava 33.0.0"

4. **jpm deps sync Command**
   - `cmd/jpm/deps_sync.go`: Show lockfile changes; offer apply
   - Check manifest vs lockfile
   - Regenerate if manifest changed
   - ✅ **Acceptance**: Detect & apply dependency version bump

5. **Integration with Build**
   - Update `jpm build` to regenerate lockfile if manifest dirty
   - Read lockfile for classpath (cache-backed)
   - ✅ **Acceptance**: Build uses cached lockfile on second run

**Deliverables**: 5 files (600 lines), 20+ tests  
**Risks**: None  
**Dependencies**: Sprint 3-4

---

### Sprint 6: Native Compiler & Packager (1.5 weeks)
**Goal**: Replace Maven with native javac + jar

#### Tasks:
1. **Source Discovery**
   - `internal/compiler/discover.go`: DiscoverSources(srcDir) -> []JavaFile
   - Recursively find .java files in src/
   - Handle nested packages
   - Unit tests
   - ✅ **Acceptance**: Find 20 .java files in sample project

2. **Javac Compiler Driver**
   - `internal/compiler/compiler.go`: Compile(sources, classpath, outDir)
   - Invoke javac with proper flags (target Java version)
   - Capture stdout/stderr
   - Handle errors; map to user-friendly messages
   - Measure compile time
   - Unit tests with mock javac
   - ✅ **Acceptance**: Compile hello-world project

3. **Main Class Auto-Detection**
   - `internal/compiler/main_class.go`: DetectMainClass(classesDir) -> string
   - Scan .class files for `public static void main(String[])`
   - Use ASM library? (or simplistic regex parser)
   - Unit tests
   - ✅ **Acceptance**: Find main in sample project

4. **JAR Packager**
   - `internal/compiler/packager.go`: PackageJar(classesDir, deps, mainClass, outPath)
   - Create META-INF/MANIFEST.MF with Main-Class
   - Add all .class files
   - Add dependency JARs (if embedded) or just META-INF/
   - Deterministic: sorted order, reproducible timestamps
   - Unit tests
   - ✅ **Acceptance**: Create executable JAR

5. **Incremental Compilation (Optional)**
   - Detect changed source files since last compile
   - Recompile only changed + dependents
   - Skip if no changes
   - ✅ **Acceptance**: Second build faster if no changes

6. **Integration with Build Command**
   - Update `cmd/jpm/build.go` to use native compiler (flag: engine=native)
   - Keep Maven fallback available (engine=maven)
   - Print build summary with timing
   - ✅ **Acceptance**: `jpm build` uses native compiler

**Deliverables**: 5 files (900 lines), 25+ tests, 1 library integration  
**Risks**: ASM library dependency; Java version detection  
**Dependencies**: Sprint 3-5

---

### Sprint 7: CLI Extensions & Utilities (1 week)
**Goal**: `jpm deps tree`, `jpm deps audit`, offline mode, verbose logging

#### Tasks:
1. **Deps Tree Command**
   - `cmd/jpm/deps_tree.go`: Render dependency tree with scopes
   - Format: indented, colorized (optional)
   - Include transitive indicators
   - Unit tests
   - ✅ **Acceptance**: Pretty-print 10-node tree

2. **Deps Audit Command** (Placeholder)
   - `cmd/jpm/deps_audit.go`: Check for conflicts/duplicates
   - Report duplicate versions of same artifact
   - Identify "orphaned" deps (never used)
   - ✅ **Acceptance**: Identify conflicts in sample project

3. **Global --offline Flag**
   - Add to all commands that fetch
   - Documented error if cache miss
   - ✅ **Acceptance**: `jpm build --offline` works or errors appropriately

4. **Global --verbose Flag**
   - Show executed commands, timing, network details
   - Write to stdout when verbose
   - ✅ **Acceptance**: `jpm build --verbose` shows all steps

5. **Structured Logging**
   - `internal/log/logger.go`: Structured JSON logs to .jpm/logs/build-<ts>.log
   - Include timestamps, levels, contexts
   - ✅ **Acceptance**: Log file contains build events

**Deliverables**: 5 files (500 lines), 15+ tests  
**Risks**: None  
**Dependencies**: Sprint 1-6

---

### Sprint 8: Platform & Polish (1.5 weeks)
**Goal**: Windows support, error UX, dry-run mode, comprehensive testing

#### Tasks:
1. **Windows Path & Symlink Handling**
   - `internal/fs/links.go`: CreateSymlinkOrJunction()
   - Try symlink; fallback to junction on Windows
   - Fallback to copy if no admin rights
   - Test on Windows CI runner
   - ✅ **Acceptance**: Works on Windows without admin

2. **Dry-Run Mode**
   - `cmd/jpm/init.go`: --dry-run flag
   - Print planned actions without writing
   - ✅ **Acceptance**: `jpm init --dry-run` shows plan

3. **Error Taxonomy & Messages**
   - `internal/errors/errors.go`: StandardError types
   - NetworkError (retry hint), ResolutionError (user fix), ParseError (report), etc.
   - Map to user-friendly messages
   - ✅ **Acceptance**: Network error suggests offline flag

4. **Windows CI Setup**
   - `.github/workflows/test-windows.yml`: Run tests on windows-latest
   - Validate Java detection, symlink fallback
   - ✅ **Acceptance**: All tests green on Windows

5. **Integration Tests** (Full Flow)
   - `test/integration/init_to_run_test.go`: Full workflow
   - Init → add deps → build → run
   - Verify output, exit codes
   - Test on all 3 OS
   - ✅ **Acceptance**: E2E test passes on macOS/Linux/Windows

**Deliverables**: 4 files + tests (700 lines), CI config, 20+ tests  
**Risks**: Windows-specific; requires CI access  
**Dependencies**: Sprint 1-7

---

### Sprint 9: Performance & Final Polish (1 week)
**Goal**: Optimize, benchmark, documentation, release readiness

#### Tasks:
1. **Performance Benchmarks**
   - Measure: init, deps add, build, run cold & warm cache
   - Set targets: init <2s, build <5s (cold), <1s (warm)
   - Report in build summary
   - ✅ **Acceptance**: All operations under target

2. **Memory Profiling**
   - Profiling on large dependency graphs (100+ nodes)
   - Fix memory leaks if found
   - ✅ **Acceptance**: Memory stable with large graph

3. **Documentation Updates**
   - Update README with native engine info
   - Add troubleshooting section
   - Document offline mode
   - Add examples (add dep → build → run)
   - ✅ **Acceptance**: README comprehensive & clear

4. **Telemetry Removal Verification**
   - Audit codebase for any telemetry/analytics code
   - Remove if found
   - ✅ **Acceptance**: Zero telemetry in final code

5. **Release Checklist**
   - All tests passing (≥80% coverage)
   - No lint errors
   - Documentation complete
   - Changelog updated
   - Version bumped
   - ✅ **Acceptance**: Ready for v0.2.0 release

**Deliverables**: Benchmarks, optimizations (200 lines), updated docs  
**Risks**: None  
**Dependencies**: Sprint 1-8

---

## NATIVE ENGINE: DEPENDENCY FETCHING STRATEGY (OPTIMIZED)

### Fetching Flow (Diagram)
```
┌─ Manifest
│  (dependencies: [guava, log4j, ...])
│
├─ Parse Dependencies
│  │
│  └─ Create root nodes
│
├─ Iterative Expansion (BFS)
│  │
│  ├─ Round 1: Fetch POMs for [guava, log4j]
│  │   ├─ guava POM → dependencies [error_prone_annotations, ...]
│  │   └─ log4j POM → dependencies [junit, ...]
│  │
│  ├─ Round 2: Fetch POMs for new deps
│  │   └─ ... (transitive)
│  │
│  └─ Continue until no new deps
│
├─ Conflict Resolution (nearest-wins)
│  │
│  └─ If same artifact appears at different depths:
│     Choose closer version (BFS order)
│
├─ Lockfile Generation
│  │
│  ├─ Sort by GAV
│  ├─ Add SHA256 checksums
│  └─ Write jpm.lock.yaml
│
└─ Fetch Artifacts (Parallel)
   │
   ├─ Worker Pool (min(8, CPU*2))
   │
   ├─ Each worker:
   │   ├─ Check cache by SHA256
   │   ├─ If miss: Download + compute SHA
   │   ├─ Verify SHA matches lockfile
   │   └─ Store in .jpm/cache/artifacts/<sha256>.jar
   │
   └─ Report: N hits, M misses, X MB, T seconds
```

### Optimization Rules
1. **Parallel Resolution**: Fetch multiple POMs concurrently
2. **Memoization**: Parse each POM once; cache in memory
3. **Early Termination**: Stop if resolution fails (errors surface in build)
4. **Cache Optimization**: Content-addressed (SHA256); survives version changes
5. **Offline Support**: If --offline, use cache or error with actionable message
6. **Determinism**: Same manifest → same lockfile → reproducible builds

---

## .gitignore GENERATION STRATEGY

### Pattern Categories
```
# JPM build artifacts
.jpm/out/                  # Build output JARs
.jpm/logs/                 # Build logs
.jpm/cache/                # Dependency cache
.jpm/work/                 # Temporary compile dir

# Maven remnants (if switched from Maven)
target/
pom.xml.bak

# IDE/Editor metadata
.idea/
.classpath
.settings/
.project
*.iml
.vscode/
.DS_Store
*.swp
*.swo
*~

# OS files
Thumbs.db
.DS_Store
```

### Generation Behavior
- **Initial**: Create .gitignore if absent
- **Idempotent**: Running again doesn't duplicate lines
- **Preserves User Content**: Never remove or reorder existing lines
- **Appends Missing**: Add any missing patterns to end
- **Tests**: Verify determinism (run 10x, same result)

---

## README GENERATOR STRATEGY

### Template Sections
```markdown
# {{ PROJECT_NAME }}

A {{ JAVA_VERSION }}-based Java project using JPM.

## Quickstart

```bash
jpm build
jpm run
```

## Project Structure

```
src/              # Java source code
.jpm/             # JPM build artifacts (generated)
jpm.yaml          # Project manifest
jpm.lock.yaml     # Lock file (generated)
.gitignore        # Generated
```

## Dependencies

View dependencies:
```bash
jpm deps ls
jpm deps tree
```

Add a dependency:
```bash
jpm deps add org.junit.jupiter:junit-jupiter@5.9.2
```

## Building & Running

```bash
# Build the project
jpm build

# Run with default main class ({{ MAIN_CLASS }})
jpm run

# Run with arguments
jpm run -- --help
```

## Offline Mode

Build with cached dependencies:
```bash
jpm build --offline
```

## Troubleshooting

- **Java not found**: `jpm init` will guide installation
- **Dependency not found**: Check `jpm deps tree` for conflicts
- **Build fails**: Run with `--verbose` for details
```

### Placeholder Substitution
- `{{ PROJECT_NAME }}` → from manifest
- `{{ JAVA_VERSION }}` → from manifest
- `{{ MAIN_CLASS }}` → from manifest

### Generation Behavior
- **Conditional**: Only create if user agrees (prompt)
- **Skip if Exists**: Ask before overwriting
- **Template-Driven**: Easy to update globally

---

## CURRENT BLOCKER RESOLUTION

### Issue 1: pom.xml Still Used in Code
**Problem**: Code references `pom.xml` directly; should generate from jpm.yaml  
**Solution**: 
- Keep pom.xml generation for now (Maven bridge)
- But mark internally: "derived from jpm.yaml; do not edit"
- Sprint 6: Replace with native compiler (no pom.xml needed)

**Sprint 2 Action**:
```go
// In cmd/jpm/build.go
// Render pom.xml from manifest (temporary; will be replaced by native compiler)
pomContent := renderPomXML(manifest) // jpm.yaml → pom.xml
```

### Issue 2: DependencyGraph Undefined
**Problem**: `internal/engine/build/builder.go` references undefined `resolver.DependencyGraph`  
**Solution**: Sprint 3 will implement full resolver interface

**Immediate Action** (Sprint 2):
- Stub resolver interface in `internal/resolver/resolver.go`
- Update builder.go to use stub

### Issue 3: Tests Failing (java-legacy/ references)
**Problem**: Tests reference old java-legacy/ pom.xml files  
**Solution**: Update tests to use jpm.yaml-based projects
- Move test fixtures to new structure
- Or skip tests in java-legacy/

**Action**: Skip legacy tests for now; Sprint 8 will add proper integration tests

---

## SUCCESS METRICS

### Phase 1 ✅
- ✅ jpm init works end-to-end
- ✅ jpm build produces JAR
- ✅ jpm run executes main class
- ✅ jpm deps add/ls work
- ✅ jpm.yaml is source of truth

### Phase 2 (Target: End of Sprint 9)
- ✅ JDK auto-installation works (SDKMAN/Homebrew)
- ✅ .gitignore & README generated
- ✅ Native resolver builds full graph
- ✅ Lockfile deterministic
- ✅ Artifacts cached and verified
- ✅ Native compiler works
- ✅ `jpm deps tree` visualizes graph
- ✅ Offline mode works
- ✅ Windows compat verified
- ✅ Performance under targets
- ✅ Zero telemetry in code

---

## ROLLOUT PLAN

### Week 1-2 (Sprint 2)
- Deploy JDK detection + install assist
- Update init flow
- Land on feature/java-assist branch
- Internal testing

### Week 3-4 (Sprint 3)
- Deploy native resolver core
- Deps add/ls use native resolver
- Land on feature/native-resolver

### Week 5-6 (Sprints 4-5)
- Deploy fetcher + caching + lockfile
- `jpm build` generates jpm.lock.yaml
- Land on feature/native-fetcher

### Week 7-8 (Sprint 6)
- Deploy native compiler
- Retire Maven dependency (keep as fallback)
- Land on feature/native-compiler

### Week 9-10 (Sprints 7-8)
- Polish: deps tree, CLI flags, Windows, integration tests
- Land on feature/cli-polish

### Week 11 (Sprint 9)
- Performance, documentation, release prep
- Merge to main as v0.2.0

---

## FILE STRUCTURE (Final)

```
go-jpm/
├── cmd/jpm/
│   ├── init.go           (updated: use generators)
│   ├── build.go          (updated: use native compiler)
│   ├── run.go            (already done)
│   ├── deps_add.go       (already done)
│   ├── deps_ls.go        (already done)
│   ├── deps_tree.go      (Sprint 7)
│   ├── deps_audit.go     (Sprint 7)
│   └── deps_sync.go      (Sprint 5)
│
├── internal/
│   ├── java/
│   │   ├── detector.go   (Sprint 2)
│   │   ├── sdkman.go     (Sprint 2)
│   │   ├── homebrew.go   (Sprint 2)
│   │   └── windows.go    (Sprint 2)
│   │
│   ├── resolver/
│   │   ├── resolver.go           (Sprint 3)
│   │   ├── graph.go              (Sprint 3)
│   │   ├── pom_fetcher.go        (Sprint 3)
│   │   ├── version_resolver.go   (Sprint 3)
│   │   ├── cycles.go             (Sprint 3)
│   │   └── parser.go             (Sprint 3)
│   │
│   ├── cache/
│   │   ├── fetcher.go    (Sprint 4)
│   │   ├── layout.go     (Sprint 4)
│   │   ├── metrics.go    (Sprint 4)
│   │   └── offline.go    (Sprint 4)
│   │
│   ├── lockfile/
│   │   ├── writer.go     (Sprint 5)
│   │   ├── reader.go     (Sprint 5)
│   │   └── diff.go       (Sprint 5)
│   │
│   ├── compiler/
│   │   ├── compiler.go        (Sprint 6)
│   │   ├── discover.go        (Sprint 6)
│   │   ├── main_class.go      (Sprint 6)
│   │   └── packager.go        (Sprint 6)
│   │
│   ├── init/
│   │   ├── gitignore.go  (Sprint 1 ✅)
│   │   └── readme.go     (Sprint 1 ✅)
│   │
│   ├── errors/
│   │   └── errors.go     (Sprint 8)
│   │
│   ├── fs/
│   │   └── links.go      (Sprint 8)
│   │
│   ├── log/
│   │   └── logger.go     (Sprint 7)
│   │
│   └── core/
│       ├── manifest_yaml.go (existing)
│       └── engine.go        (existing)
│
├── docs/
│   ├── INIT_PROTOTYPE.md                (existing)
│   ├── NATIVE_ENGINE_ARCHITECTURE.md    (existing)
│   ├── PHASE2_NATIVE_ENGINE_PLAN.md     (existing)
│   ├── PHASE2_BACKLOG_PRIORITIZED.md    (existing)
│   ├── RESOLVER_IMPLEMENTATION.md       (existing)
│   ├── FETCHER_IMPLEMENTATION.md        (existing)
│   ├── COMPILER_IMPLEMENTATION.md       (existing)
│   ├── PACKAGER_IMPLEMENTATION.md       (existing)
│   ├── CLASSPATH_IMPLEMENTATION.md      (existing)
│   ├── GENERATORS_IMPLEMENTATION.md     (existing)
│   ├── LOCKFILE_SPEC.md                 (existing)
│   ├── TESTING_STRATEGY.md              (existing)
│   └── IMPLEMENTATION_ROADMAP_COMPREHENSIVE.md (THIS FILE)
│
├── .github/workflows/
│   ├── test.yml          (existing: macOS/Linux)
│   └── test-windows.yml  (Sprint 8)
│
└── go.mod, go.sum, README.md, LICENSE
```

---

## RISKS & MITIGATIONS

| Risk | Impact | Mitigation |
|------|--------|-----------|
| POM parsing complexity | Sprint 3 delay | Use `github.com/antchfx/xquery` or stdlib XML; tests with real POMs |
| Network flakiness | Sprint 4 unreliable | Robust retry logic; mock tests; CI with flaky network simulator |
| Windows symlink/junction issues | Sprint 8 blocker | Fallback to copy; test early on Windows CI |
| Large dependency graphs (100+ nodes) | Sprint 9 perf issue | Profiling; streaming graph construction; memoization |
| Java version extraction complexity | Sprint 2 delay | Use `java -version` parser; extensive unit tests |
| Concurrent cache access | Sprint 4 corruption | RWMutex; atomic writes; temp file → rename pattern |

---

## DECISION LOG

### Decision 1: Maven Bridge Strategy
**Context**: Need working builds now; native compiler not ready  
**Decision**: Keep hidden Maven engine as fallback; pom.xml generated from jpm.yaml  
**Rationale**: Users never see Maven; can switch to native later  
**Revisit**: Sprint 6 (retire Maven when native ready)

### Decision 2: No Telemetry
**Context**: Privacy concerns; user request  
**Decision**: Remove all telemetry code; no collection, no opt-in  
**Rationale**: Simpler UX; privacy-first  
**Revisit**: Never (unless requested by community)

### Decision 3: Lockfile Format
**Context**: Need reproducible builds  
**Decision**: jpm.lock.yaml (YAML, human-readable, sorted)  
**Rationale**: Readable for debugging; Git-friendly diffs  
**Revisit**: Never (locked in Phase 2)

### Decision 4: Nearest-Wins Conflict Resolution
**Context**: Dependency version conflicts  
**Decision**: Use BFS order (first appearance wins)  
**Rationale**: Intuitive; matches Maven behavior; documented  
**Revisit**: Sprint 7+ if needed (advanced override rules)

---

## QUESTION CLARIFICATIONS

### Q: "Why are we still using pom.xml? Aren't we using jpm.yaml?"
**A**: jpm.yaml IS the source of truth. pom.xml is a *derived* artifact generated from jpm.yaml during build for Maven bridge. Sprint 6 will replace Maven with native compiler, eliminating need for pom.xml.

### Q: "Have we completed Sprint 5?"
**A**: No. Current state: Sprint 1 ✅ (generators done). Sprints 2-9 are planned but not started. This roadmap defines what Sprints 2-9 will deliver.

### Q: "What about telemetry?"
**A**: Already removed. No telemetry in code. Phase 2 plan explicitly excludes it.

---

## EXECUTION CHECKLIST (Sprint 2 START)

- [ ] Review this roadmap with team
- [ ] Confirm Sprint 2 tasks with owners
- [ ] Set up feature/java-assist branch
- [ ] Create Sprint 2 subtasks in project tracker
- [ ] Assign Java detection work
- [ ] Begin Sprint 2 (Week 1: Nov 16-22)
- [ ] Daily standup on progress

---

## CONCLUSION

JPM is transitioning from prototype (Maven bridge) to production (native engine). Phase 2 will deliver a modern, fast, and dependency-neutral build system over the next 9 sprints. This roadmap provides task-level clarity, risk mitigation, and a clear path to v0.2.0 release.

**Status**: 🟢 Ready for Sprint 2 Execution  
**Target Release**: End of Sprint 9 (v0.2.0)  
**Owner**: Core CLI / Engine Team

---

**Document Version**: 1.0 Final  
**Last Updated**: 2025-11-09T17:02:51Z  
**Next Review**: Sprint 2 Kickoff (2025-11-16)
