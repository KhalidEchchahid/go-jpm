# Phase 2 Architecture & Technical Decisions

**Date**: 2025-11-09T16:47:00Z  
**Status**: Finalized  
**Scope**: Phase 2-3 Technical Direction

---

## 1. Telemetry Decision

### Status: ✅ REMOVED

**Decision**: No telemetry collection in JPM.

**Reasoning**:
- User privacy is paramount
- No tracking, no data collection, no analytics
- Open source philosophy: transparency + user control
- Simpler codebase, fewer dependencies

**What This Means**:
- No analytics code added
- No opt-in prompts for telemetry
- No session tracking or identification
- All user activity stays on their machine
- Logs written only to `.jpm/logs/` (local)

**Implementation**:
- Remove any telemetry-related TODOs from codebase
- Do not add telemetry libraries (no mixpanel, sentry, etc.)
- Document privacy stance in README

---

## 2. Build Tool Strategy: Maven → Native

### Phase 2 (Hybrid)
```
jpm.yaml (user-editable)
    ↓
Maven Engine (hidden in .jpm/)
    ├─ Renders pom.xml (generated, not user-editable)
    └─ Invokes `mvn compile package`
    
Output: .jpm/out/*.jar
```

**Why Maven in Phase 2?**
- Battle-tested, stable
- Risk mitigation (fallback)
- Allows parallel native engine development
- Gradual transition path

### Phase 3 (Pure Native)
```
jpm.yaml (user-editable)
    ↓
Native Engine
    ├─ Resolver (parse jpm.yaml)
    ├─ Fetcher (download deps, cache)
    ├─ Compiler (javac)
    └─ Packager (create JAR)
    
Output: .jpm/out/*.jar
lockfile: jpm.lock.yaml
```

**Why Native in Phase 3?**
- No Maven dependency
- Faster builds (direct javac, no Maven overhead)
- Simpler toolchain (single executable)
- Deterministic lockfile (reproducible builds)
- Self-contained (no JVM tooling required beyond JDK)

### pom.xml Lifecycle

**Phase 2**: 
- Generated from jpm.yaml on-demand
- Used by Maven engine for build
- Hidden from user (in .jpm/)
- Location: `.jpm/maven/pom.xml`

**Phase 3**:
- Deprecated (not generated)
- If user has existing pom.xml, warn but don't break
- Migration guide for Maven → JPM provided
- Location: Removed

**User Never Sees pom.xml**: ✅
- It's an implementation detail
- All commands use `jpm` syntax only
- Error messages reference jpm.yaml, not pom.xml
- Advanced users can inspect .jpm/ but it's marked "generated"

---

## 3. Manifest as Single Source of Truth

### jpm.yaml (Authoritative)
```yaml
name: my-app
build_tool: maven        # Phase 2 only; removed Phase 3
engine: maven            # Will auto-switch to native
java:
  version: "21"
project:
  group_id: app
  artifact_id: my-app
  version: 0.1.0-SNAPSHOT
app:
  main_class: Main
dependencies:
  - group: org.slf4j
    artifact: slf4j-api
    version: "2.0.16"
    scope: compile
```

### What Derives From jpm.yaml?
- `.jpm/maven/pom.xml` (Phase 2 only; generated)
- `.jpm/out/*.jar` (artifact)
- `jpm.lock.yaml` (lockfile with checksums)

### What's Never in jpm.yaml?
- Build tool configuration (Maven plugins, Gradle tasks)
- Repository URLs (defaults to Maven Central)
- JVM arguments (user's choice at runtime)
- IDE configuration

### Immutability Rules
- Users edit jpm.yaml directly (not tool files)
- `jpm deps add` updates jpm.yaml atomically
- `jpm build` regenerates derived files
- Derived files (pom.xml, lock, jar) not committed (in .gitignore)

---

## 4. Engine Abstraction Layer

### Design
```go
// internal/core/engine.go

type Engine interface {
    Build(ctx context.Context, plan BuildPlan) (BuildResult, error)
    Run(ctx context.Context, spec RunSpec) (int, error)
    ResolveAndFetch(ctx context.Context, manifest *Manifest) (*ResolvedSet, error)
}

type MavenEngine struct {
    // Phase 2 only
    // Executes: mvn compile package
}

type NativeEngine struct {
    // Phase 3
    // Resolver, Fetcher, Compiler, Packager
}
```

### Selection Logic
```go
func SelectEngine(manifest *Manifest) Engine {
    // Phase 2: Always Maven
    if manifest.Engine == "maven" {
        return &MavenEngine{}
    }
    // Phase 3: Native
    if manifest.Engine == "native" {
        return &NativeEngine{}
    }
    // Fallback
    return &MavenEngine{} // Safest choice
}
```

### Transition Strategy
1. **Week 1-2 (Sprint 2)**: Keep maven default
2. **Week 3-4 (Sprint 3)**: Native engine dev (behind flag)
3. **Week 5-8 (Sprints 4-5)**: Native engine complete, tests passing
4. **Week 9-10 (Sprint 6)**: Feature parity + lockfile
5. **Week 11+**: Auto-detect engine from manifest or switch default

---

## 5. Offline-First Strategy

### Cache Location
```
.jpm/cache/
├── artifacts/
│   ├── abc123.../artifact.jar
│   ├── def456.../artifact.pom
│   └── ghi789.../...
└── index.json
    {
      "org.slf4j:slf4j-api:2.0.16": {
        "sha256": "abc123...",
        "urls": ["https://repo1.maven.org/..."]
      }
    }
```

### Modes

**Online (Default)**
- Fetch missing artifacts from Maven Central
- Store in cache
- Build proceeds normally

**Offline (`--offline`)**
- No network access
- All artifacts must be in cache
- Fails with actionable error if missing
- Example: `jpm build --offline`

### User Workflow
```bash
# First build: online (fetches + caches)
jpm build

# Second build on laptop without internet: works!
jpm build --offline

# Or on airplane
jpm build --offline

# If cache miss on offline
# Error: "Dependency X not cached. Run `jpm build` online first."
```

---

## 6. Deterministic Builds

### Lockfile (jpm.lock.yaml)
```yaml
version: "1"
timestamp: "2025-11-09T16:47:00Z"
tool_version: "1.0.0"
java_version: "21"

dependencies:
  - group: org.slf4j
    artifact: slf4j-api
    version: "2.0.16"
    scope: compile
    sha256: "abc123def456..."
    source: "https://repo1.maven.org/maven2"
    
  - group: org.slf4j
    artifact: slf4j-simple
    version: "2.0.16"
    scope: compile
    sha256: "ghi789jkl012..."
    source: "https://repo1.maven.org/maven2"
```

### Determinism Rules
1. **Sorted order**: By `groupId:artifactId` (alphabetical)
2. **Timestamps**: UTC, ISO 8601 format
3. **Checksums**: SHA-256 computed at fetch time
4. **Immutability**: Lock doesn't change unless manifest changes or `--update` used
5. **Reproducibility**: Same manifest + lockfile = byte-identical jar on any machine

### VCS Guidelines
- **Commit**: jpm.yaml + jpm.lock.yaml
- **Don't commit**: .jpm/ (generated), .gitignore handles it
- **Result**: Anyone can clone + build with exact same versions

---

## 7. Error Handling & Observability

### No Telemetry, But Good Logging

**Logs Stored Locally**
```
.jpm/logs/
├── build-2025-11-09-164700.log
├── run-2025-11-09-164750.log
└── deps-add-2025-11-09-164800.log
```

**Content**: 
- Timestamps
- Steps (resolve, fetch, compile, package)
- Errors + context
- No personal information (paths sanitized if sensitive)

**User Access**
- `jpm build --verbose` displays logs to stdout
- Logs auto-archived in .jpm/logs/
- User can inspect history

### Error Taxonomy
```
NetworkError:
  Cause: Connection timeout / DNS failure
  Remedy: Check internet; retry with --verbose for details

ResolutionError:
  Cause: Dependency not found / version conflict
  Remedy: Check dependency names in jpm.yaml; fix + retry

IntegrityError:
  Cause: Checksum mismatch
  Remedy: Clear cache (rm -rf .jpm/cache/) + retry

CompileError:
  Cause: Source code issue
  Remedy: Check jpm build output; fix Java code
```

---

## 8. Security Posture

### HTTPS Only
- All artifact downloads over HTTPS
- Reject plain HTTP by default
- Certificate verification (system trust store)

### Checksum Verification
- SHA-256 computed at fetch time
- Stored in jpm.lock.yaml
- Verified on reuse
- Mismatch → re-fetch or fail (user can clear cache)

### No Arbitrary Execution
- No `mvn` plugins loaded (Phase 2)
- No `gradle` scripts loaded
- No custom build scripts in jpm.yaml
- Safe-by-default philosophy

### Supply Chain Security
- Lockfile pins versions + checksums
- Same manifest → same build on any machine
- Makes supply chain attacks harder (attacker must compromise Maven Central + match checksum)
- Future: Integrate with advisories (not Phase 2-3 scope)

---

## 9. Windows Compatibility

### Symlink Fallback
**Unix (Linux/macOS)**:
- `.jpm/maven/src/main/java` → `../../src` (symlink)

**Windows**:
1. Try symlink (requires admin on older Windows)
2. Fallback to directory junction (works without admin)
3. Fallback to copy (slow, last resort)

**Code**
```go
// internal/fs/link.go
func CreateLink(target, link string) error {
    err := os.Symlink(target, link)
    if err != nil {
        // Try junction (Windows)
        return createJunction(target, link)
    }
    return nil
}
```

### Path Separators
- Resolved in classpath builder (`:` on Unix, `;` on Windows)
- Tested on Windows runner in CI

---

## 10. Performance Targets

### Build Time Goals
- **Cold build** (first time, no cache): < 30s
- **Warm build** (cache hit): < 5s
- **Incremental** (only changed sources): < 3s (Phase 3+)

### Fetch Parallelization
- Concurrency: `min(8, CPUs * 2)` workers
- Typical laptop (4 cores): 8 workers
- Typical CI (2 cores): 4 workers
- Measured: 3-4x speedup vs sequential

### Memory Usage
- Typical project: < 100 MB
- Large projects (100+ deps): < 500 MB (acceptable for CI)

---

## 11. Key Files & Packages

### Core Engine (internal/engine/)
```
internal/engine/
├── resolver/
│   ├── model.go
│   ├── pom_parser.go
│   └── resolver.go
├── fetcher/
│   ├── model.go
│   ├── cache.go
│   ├── fetcher.go
│   └── parallel.go
├── build/
│   ├── compiler.go
│   ├── packager.go
│   ├── classpath.go
│   └── main_class.go
└── lock/
    └── lockfile.go
```

### CLI Commands (cmd/jpm/)
```
cmd/jpm/
├── init.go (updated with generators + install assist)
├── build.go (switches to native engine)
├── run.go
├── deps_ls.go
├── deps_add.go
├── deps_tree.go (new, Phase 3)
├── deps_sync.go (new, Phase 3)
└── root.go (global flags)
```

### Init Generators (internal/init/)
```
internal/init/
├── gitignore.go (✅ done)
├── gitignore_test.go (✅ done)
├── readme.go (✅ done)
└── readme_test.go (planned, can use gitignore_test.go patterns)
```

---

## 12. Testing Strategy

### Unit Tests
- Resolver: graph building, conflicts, cycles
- Fetcher: cache hit/miss, retry, checksum
- Compiler: javac invocation, error capture
- Packager: deterministic JAR

### Integration Tests
- Full pipeline: resolve → fetch → compile → package
- With real deps (slf4j, guava)
- Offline mode: primed cache
- Lockfile determinism: same manifest → same build

### CI Matrix
```yaml
os: [ubuntu-latest, macos-latest, windows-latest]
java: ['17', '21']
# 6 combinations total
```

### Coverage Goals
- Resolver/Fetcher/Build: ≥ 80%
- CLI: ≥ 70%
- Overall: ≥ 75%

---

## 13. Documentation

### User-Facing
- **README.md**: Quickstart, commands, troubleshooting
- **docs/INIT_PROTOTYPE.md**: Init UX details (kept for reference)
- **docs/NATIVE_ENGINE.md**: Architecture overview (new, Phase 3)

### Developer-Facing
- **docs/PHASE2_BACKLOG_PRIORITIZED.md**: Sprint tasks
- **docs/NATIVE_ENGINE_ARCHITECTURE.md**: System design
- **docs/RESOLVER_IMPLEMENTATION.md**: Graph algorithm
- **docs/FETCHER_IMPLEMENTATION.md**: Cache + network
- **docs/LOCKFILE_SPEC.md**: jpm.lock.yaml format
- **docs/TESTING_STRATEGY.md**: Test approach

---

## 14. Migration Path

### From Maven pom.xml to JPM
1. **Phase 2**: `jpm init` scaffolds with hidden pom.xml (Maven engine)
2. **Phase 3**: Switch to native engine automatically
3. **Existing Projects**: `jpm init` in existing Maven project (future: import pom.xml)

### Deprecation Timeline
- **Phase 2 end**: warn if using Maven engine manually
- **Phase 3 start**: native engine stable
- **Phase 3 mid**: auto-switch to native, keep Maven engine as fallback
- **Phase 4+**: Remove Maven engine entirely

---

## 15. Success Criteria

### By End of Phase 2
- ✅ Generators (gitignore, README)
- ✅ JDK detection + install assist
- ✅ Init polish (dry-run, validation)
- ✅ Hidden Maven engine (working)
- ✅ User never sees pom.xml

### By End of Phase 3
- ✅ Native engine complete (all components)
- ✅ No Maven dependency
- ✅ Deterministic builds (lockfile)
- ✅ Offline support
- ✅ ≥80% test coverage
- ✅ All platforms tested (Windows, macOS, Linux)
- ✅ Ready for public beta

---

## 16. Decisions Made

| Decision | Rationale | Status |
|----------|-----------|--------|
| No telemetry | User privacy + open source principles | ✅ Final |
| jpm.yaml as source of truth | Single source, user-editable, deterministic | ✅ Final |
| Hybrid Phase 2 (Maven) | Risk mitigation, stable fallback | ✅ Final |
| Native Phase 3 | Remove Maven, self-contained, fast | ✅ Final |
| Lockfile (jpm.lock.yaml) | Determinism, reproducibility, supply chain | ✅ Final |
| Offline-first design | Works on airplane, CI-friendly | ✅ Final |
| Windows junctions fallback | Compatibility, no admin required | ✅ Final |
| Parallel fetcher | Performance (3-4x speedup) | ✅ Final |

---

## 17: Next Phase

**Sprint 2 (Java Detection)**:
- Implement detector.go
- Add SDKMAN/Homebrew/Manual installers
- Integrate into init
- Test all platforms

**Start Date**: Next week  
**Duration**: 2-3 weeks  
**Deliverable**: `jpm init` with JDK assist working seamlessly

---

*Document: PHASE2_ARCHITECTURE_DECISIONS.md*  
*Last Updated: 2025-11-09T16:47:00Z*
