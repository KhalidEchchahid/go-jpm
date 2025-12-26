# Native Engine Architecture & Implementation Guide

Date: 2025-11-09
Owner: Core Engine Team
Status: Implementation Reference

---
## Overview
The Native Engine is JPM's internal build, dependency resolution, and package management system. It replaces Maven/Gradle with a purpose-built resolver, fetcher, compiler, and packager written in Go. Users interact only via `jpm` commands; internal machinery is never exposed.

---
## Design Goals
1. **Simplicity**: No XML, no verbose configs; single source of truth is jpm.yaml.
2. **Performance**: Parallel resolution, intelligent caching, incremental builds (future).
3. **Reproducibility**: Deterministic lockfile (jpm.lock.yaml) ensures exact versions everywhere.
4. **Offline-first**: Cache-conscious design with graceful degradation.
5. **Security**: HTTPS-only, checksum verification, transparent supply chain.
6. **Extensibility**: Clear interfaces for future engines (Gradle, native compilation).

---
## System Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        CLI Layer                             │
│  jpm init | jpm build | jpm run | jpm deps add/tree/audit   │
└──────────────────────────────────────────────────────────────┘
                            ↓
┌──────────────────────────────────────────────────────────────┐
│                    Command Handlers                          │
│  LoadManifest → ValidateConfig → SelectEngine               │
└──────────────────────────────────────────────────────────────┘
                            ↓
┌──────────────────────────────────────────────────────────────┐
│                  Native Engine Interface                     │
│                 (Build | Run | Test | Deps)                 │
└──────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────┬──────────────────┬────────────────────────┐
│    Resolver     │    Fetcher       │    Build Pipeline      │
│  (Graph Build)  │ (Cache + Network)│  (Compile → Package)   │
└─────────────────┴──────────────────┴────────────────────────┘
                            ↓
┌──────────────────────────────────────────────────────────────┐
│                Storage & Cache Layer                         │
│  .jpm/cache/artifacts/<sha256> | .jpm/lock.yaml             │
└──────────────────────────────────────────────────────────────┘
```

---
## Core Components

### 1. Resolver (Dependency Graph Builder)
**File**: `internal/engine/resolver/resolver.go`

**Purpose**: Transform manifest dependencies into a complete, conflict-resolved graph.

**Interface**:
```go
type Resolver interface {
    // Resolve builds graph from manifest, returning root node + all unique artifacts.
    Resolve(ctx context.Context, manifest *core.Manifest) (*Graph, error)
}

type Graph struct {
    Root     *Node              // Entry point (synthetic or first dependency)
    Nodes    map[string]*Node   // All unique GAV -> resolved node
    Metadata *ResolutionMeta    // Timestamp, version, strategy used
}

type Node struct {
    GAV         string             // "group:artifact:version"
    Scope       core.DependencyScope
    Optional    bool
    Parents     []*Node            // Path to root (for cycle detection)
    Children    []*Node            // Direct dependencies
    Exclusions  []string           // Excluded GAV prefixes
    Meta        *NodeMetadata      // Checksum, repo, fetch status
}

type ResolutionMeta struct {
    Timestamp  time.Time
    Strategy   string  // "nearest-wins"
    ToolVer    string
}
```

**Algorithm**:
1. **Initialize**: Queue root dependencies (from manifest).
2. **BFS Expansion**: 
   - Pop dependency from queue.
   - Fetch its POM + parse (groupId, artifactId, version, parent, dependencyManagement, dependencies).
   - Apply parent inheritance if needed.
   - For each child dependency:
     - Check if GAV already resolved (memoization).
     - If conflict: apply nearest-wins (user closer to root wins).
     - Enqueue if new or closer.
3. **Validate**: Detect cycles; fail with explanation.
4. **Finalize**: Topological sort for deterministic ordering.

**Key Methods**:
- `resolveGAV(group, artifact, versionRange) -> resolvedVersion` (Phase 2: simple; Phase 3+: ranges).
- `parsePOM(pomContent) -> POMModel` (parent, deps, dependencyManagement).
- `applyExclusions(node, exclusionPatterns) -> filteredChildren`.
- `detectConflict(existing, candidate) -> winner` (nearest distance to root).
- `buildLockfile() -> Lock`.

**Memoization**:
- Cache parsed POMs by GAV (avoid re-fetch/parse).
- Cache resolved conflicts by (A, B) pair.

---
### 2. Fetcher (Network & Cache)
**File**: `internal/engine/fetcher/fetcher.go`

**Purpose**: Retrieve artifacts and POMs with caching, retries, integrity checks.

**Interface**:
```go
type Fetcher interface {
    // FetchArtifact downloads JAR; returns path in cache + metadata.
    FetchArtifact(ctx context.Context, gav string) (*CachedArtifact, error)
    
    // FetchPOM downloads POM; returns content + metadata.
    FetchPOM(ctx context.Context, gav string) ([]byte, *ArtifactMeta, error)
}

type CachedArtifact struct {
    Path          string             // Absolute path to cached file
    SHA256        string             // Computed checksum
    Size          int64
    LastFetched   time.Time
    Repository    string             // Which repo served it
}

type ArtifactMeta struct {
    SHA256        string
    Size          int64
    Timestamp     time.Time
    SourceURL     string
}
```

**Cache Layout**:
```
.jpm/
  cache/
    artifacts/
      <sha256>                     # Content-addressed storage
        pom.xml (optional)
        index.json                 # {"gav": "...", "size": ..., "fetched": ...}
```

**Algorithm**:
1. **Check Cache**: Lookup by SHA256 (from lockfile if available).
2. **If Hit**: Verify file exists; return path.
3. **If Miss**:
   - Fetch from repository (default: Maven Central).
   - Compute SHA256 on-the-fly.
   - Store with content-addressing.
   - Record in index.
   - Update lockfile entry.
4. **Integrity**: Compare computed SHA256 vs lockfile; on mismatch, redownload once.
5. **Offline Mode**: If `--offline`, skip network; fail if not in cache.

**Network**:
- Worker pool (default concurrency: min(8, CPUs * 2)).
- Per-artifact timeout: 30s.
- Per-repository timeout: 60s (for metadata queries).
- Retries: exponential backoff (1s, 2s, 4s) for transient errors (5xx, timeout).
- HTTPS only; reject HTTP by default.

**Error Handling**:
- Network error → retry with backoff.
- 404 → artifact not found (suggest version typo).
- 403 → access denied (suggest credentials/mirror).
- Checksum mismatch → warn, redownload once.
- Offline + cache miss → fail with "run without --offline" suggestion.

---
### 3. Dependency Graph Processor
**File**: `internal/engine/deps/processor.go`

**Purpose**: Apply scope rules, exclusions, optional flags to produce compile/runtime/test classpaths.

**Interface**:
```go
type DepsProcessor interface {
    // ClasspathFor returns artifacts needed for given scope.
    ClasspathFor(graph *resolver.Graph, scope core.DependencyScope) []string
    
    // Tree renders graph for human consumption.
    Tree(graph *resolver.Graph, format string) string
}
```

**Scope Rules** (Maven semantics):
- `compile`: included in all classpaths (default).
- `test`: included only in test classpath; excluded from runtime.
- `provided`: excluded from packaged JAR (assume available at runtime).
- `optional`: included only if explicitly requested.

**Exclusion Processing**:
- Node lists excluded GAV patterns (e.g., "log4j:*").
- Processor filters out matching transitive deps.

---
### 4. Lockfile Manager
**File**: `internal/engine/lock/lockfile.go`

**Purpose**: Persist resolved state; ensure reproducibility.

**Format** (`jpm.lock.yaml`):
```yaml
# Auto-generated; do not edit manually
version: "1"
timestamp: "2025-11-09T15:47:47Z"
tool_version: "0.0.354"
resolver: "native"

dependencies:
  - gav: "org.slf4j:slf4j-api:2.0.16"
    scope: compile
    optional: false
    sha256: "abc123..."
    source_url: "https://repo.maven.apache.org/maven2/..."
    size: 12345
    
  - gav: "com.google.guava:guava:33.0.0-jre"
    scope: compile
    optional: false
    sha256: "def456..."
    source_url: "..."
    size: 54321
```

**Operations**:
- `Load(path)`: Parse YAML.
- `Save(path, graph)`: Generate from graph; sort by GAV.
- `Sync(manifest, existing)`: Compare; report added/removed/changed.
- `GetArtifactPath(gav)`: Lookup in cache.

**Determinism**:
- Sort entries by `group:artifact:version` lexicographically.
- Pin timestamps to UTC only (no timezone offsets).
- Exclude non-deterministic fields (like fetch duration).

---
### 5. Compiler & Packager
**File**: `internal/engine/build/compiler.go`, `internal/engine/build/packager.go`

**Purpose**: Compile sources and create JAR.

**Compiler Interface**:
```go
type Compiler interface {
    // Compile processes all .java files in srcDir; outputs .class files to outDir.
    Compile(ctx context.Context, srcDir, outDir, classpath string) (*CompileResult, error)
}

type CompileResult struct {
    ClassCount  int
    Warnings    []string
    Duration    time.Duration
}
```

**Algorithm**:
1. **Discover**: Walk `src/` for `.java` files.
2. **Prepare**: Ensure `.jpm/work/classes` exists.
3. **Invoke**: `javac -d <outDir> -cp <classpath> <files...>`.
4. **Capture**: stdout/stderr for warnings/errors.
5. **Return**: Success or detailed error (parse line numbers).

**Main Class Detection**:
If not set in manifest, scan compiled classes:
- Look for `public static void main(String[] args)` method.
- Accept first match or prompt user if ambiguous.
- Update manifest.

**Packager Interface**:
```go
type Packager interface {
    // Package creates JAR from classes dir; returns path.
    Package(ctx context.Context, classesDir, mainClass, outDir string) (string, error)
}
```

**Algorithm**:
1. **Create JAR**: Using Go's `archive/zip` package.
2. **Add Manifest**:
   - `Main-Class: <mainClass>`
   - `Implementation-Version: <version>`
   - `Created-By: JPM <version>`
3. **Add Classes**: All `.class` files recursively from `classesDir`.
4. **Output**: `.jpm/out/<artifact>-<version>.jar`.

---
### 6. Classpath Assembler
**File**: `internal/engine/build/classpath.go`

**Purpose**: Combine resolved artifacts into classpath string.

**Algorithm**:
1. For each artifact in graph with scope ∈ {compile, runtime}:
   - Get path from Fetcher/Cache.
   - Exclude `optional` artifacts unless explicitly included.
2. Join with OS path separator (`:` on Unix, `;` on Windows).
3. Prepend `.jpm/work/classes` if compiling.

---
### 7. Runtime Launcher
**File**: `internal/engine/runtime/launcher.go`

**Purpose**: Execute compiled JAR.

**Algorithm**:
1. Verify JAR exists.
2. Extract `Main-Class` from manifest or use manifest.app.main_class.
3. Build classpath (JAR + transitive optional deps if requested).
4. Invoke `java -cp <classpath> <mainClass> <userArgs...>`.
5. Stream stdout/stderr.
6. Propagate exit code.

---
## Dependency Resolution Strategy

### Conflict Resolution
**Policy**: Nearest-wins (Maven transitive).

**Example**:
```
Root
  ├─ A v1.0
  │   └─ C v2.0
  └─ B v1.0
      └─ C v1.5

Result: C v2.0 (A is direct dependency of root, closer than B→C)
```

**Implementation**:
- Track distance from root for each node.
- On conflict, select node with minimum distance.
- On tie (same distance), prefer first encountered (stable order).

### Version Resolution
**Phase 1 (Current)**:
- Direct version only (no ranges).
- Manifest declares exact versions.

**Phase 2+ (Future)**:
- Range support (e.g., `[1.0, 2.0)`, `1.+`).
- Selection: prefer highest release, avoid snapshots by default.

### Cycle Detection
If path from root to node includes node itself:
- Log cycle path: `A -> B -> A`.
- Fail fast with explanation.
- Suggest breaking cycle (remove optional dep).

---
## Lockfile Format & Determinism

**Example**:
```yaml
version: "1"
timestamp: "2025-11-09T15:47:47Z"
tool_version: "0.0.354"
resolver: "native"

dependencies:
  - gav: "com.example:app:1.0"
    scope: compile
    optional: false
    sha256: "e3b0c44..."
    source_url: "https://repo.maven.apache.org/maven2/com/example/app/1.0/app-1.0.jar"
    size: 0
```

**Determinism Guarantees**:
- Sorted by GAV alphabetically.
- Timestamps in UTC (Z suffix).
- No random ordering.
- Same manifest + repos => byte-identical lockfile (bit-reproducible).

---
## Error Taxonomy

| Error | Example | Recovery |
|-------|---------|----------|
| **Resolution** | Unsatisfiable version conflict | User edits manifest; retry. |
| **Fetch** | Artifact not found (404) | Suggest version typo; check manifest. |
| **Network** | Timeout connecting to repository | Retry with backoff; suggest --offline. |
| **Integrity** | Checksum mismatch | Redownload once; if fails, suggest cache clear. |
| **Parse** | Malformed POM XML | Report POM GAV + line number; suggest reporting upstream. |
| **Compile** | Syntax error in source | javac output with file + line. |

---
## Caching Strategy

**Three-Tier Cache**:
1. **In-Memory**: Parsed POMs + resolved nodes during current build.
2. **Disk (Content-Addressed)**: `.jpm/cache/artifacts/<sha256>`.
3. **Lockfile**: Persisted checksums for next build verification.

**Invalidation**:
- In-memory: cleared per build.
- Disk: checked against lockfile; redownload if mismatch.
- Lockfile: regenerated if manifest changes.

**Optimization**:
- Parallel fetch for independent artifacts.
- Checkpoint after each phase (resolve, fetch, compile).

---
## Offline Mode

**Activation**: `jpm build --offline`

**Behavior**:
1. Skip all network requests.
2. Use only `.jpm/cache/artifacts/`.
3. If artifact missing from cache:
   - Log GAV + missing artifact.
   - Fail with summary of missing artifacts.
   - Suggest: `jpm build` (online) or populate cache manually.

---
## Verbose Logging

**Activation**: `jpm build --verbose`

**Output**:
- Resolver: "Resolving graph for X dependencies..."
  - "  - org.slf4j:slf4j-api:2.0.16 (nearest-wins vs 2.0.10)"
  - "  - Cycle detected: A -> B -> A (skipping)"
- Fetcher: "Fetching 5 artifacts (2 cached, 3 network)..."
  - "  - org.slf4j:slf4j-api (12 KB from cache)"
  - "  - com.google.guava:guava (2.1 MB, 2.34s)"
- Compiler: "Compiling 3 sources to .jpm/work/classes"
  - "  - Main.java"
  - "  - Util.java"
  - "  - Warnings: 1"
- Packager: "Packaging to .jpm/out/app-1.0.jar"

---
## Testing Strategy

### Unit Tests
- **POM Parser**: parent resolution, exclusions, optional flags.
- **Resolver**: conflict resolution, cycle detection, memoization.
- **Fetcher**: cache hits/misses, retries, checksum verification.
- **Compiler**: classpath assembly, main class detection.
- **Packager**: manifest creation, JAR structure.

### Integration Tests
- **End-to-End**: Init → add deps → build → run (simple project).
- **Cache Reuse**: Build twice; verify second is faster + uses cache.
- **Offline**: Prime cache; build --offline; success.
- **Conflict Resolution**: Multi-level deps with conflicts; verify nearest-wins.
- **Windows Symlink**: Verify fallback to directory creation.

### CI Matrix
- **Ubuntu**: Java 17, 21, 23
- **macOS**: Java 21 (latest)
- **Windows**: Java 21 (latest)

---
## Performance Tuning

### Baseline Metrics
- Cold resolve (no cache): < 5s for 10 deps.
- Warm resolve (cache): < 1s.
- Fetch 10 artifacts in parallel: 2-3s (network dependent).
- Compile 100 sources: < 5s.
- Package JAR: < 500ms.

### Optimization Opportunities
- **Memoization**: Cache parsed POMs (done).
- **Parallelization**: Worker pool for fetch (done).
- **Incremental Compile**: Detect changed sources (future).
- **Streaming**: Large graphs should not load entirely in memory (future).

---
## Configuration (jpm.yaml)

**Native Engine Section** (future):
```yaml
engine: native
repositories:
  - id: maven-central
    url: https://repo.maven.apache.org/maven2
  - id: custom
    url: https://my-repo.com/maven

resolver:
  strategy: nearest-wins
  skip_snapshots: true  # Prefer releases
  
fetcher:
  concurrency: 8
  timeout_sec: 30
  retry_backoff: [1, 2, 4]
```

---
## Security & Supply Chain

1. **HTTPS Only**: Enforce TLS; reject plain HTTP.
2. **Checksum Verification**: SHA-256 computed + verified.
3. **Lockfile Integrity**: Pin exact versions + checksums.
4. **Gradual Rollout**: Keep `engine: maven` default until native proven stable.

---
## Future Enhancements

- **BOM Support**: Import managed dependencies from parent/BOM.
- **Version Ranges**: `[1.0, 2.0)` with heuristics.
- **Gradle Parity**: Native Gradle plugin support.
- **Supply Chain Auditing**: Detect known vulnerabilities in lockfile.
- **Incremental Builds**: Cache compiled classes; recompile only changed sources.

---
## Glossary

- **GAV**: Group:Artifact:Version (Maven coordinate).
- **Scope**: Dependency lifetime (compile, test, runtime, provided, optional).
- **Transitive**: Dependency of a dependency.
- **Lockfile**: Deterministic record of resolved + fetched artifacts.
- **Nearest-Wins**: Conflict resolution favoring dependencies closest to root.
- **Content-Addressed**: Storage keyed by hash of content (SHA-256).
