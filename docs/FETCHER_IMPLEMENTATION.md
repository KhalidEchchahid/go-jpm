# Fetcher & Caching Implementation Guide

Date: 2025-11-09
Owner: Core Engine Team

---
## Overview
The Fetcher manages network retrieval of artifacts and POMs, implements intelligent caching, performs integrity checks, and provides graceful fallback for offline scenarios.

---
## Data Model

```go
package fetcher

// Fetcher is the interface for artifact retrieval & caching.
type Fetcher interface {
    // FetchArtifact retrieves a JAR; returns cache path + metadata.
    FetchArtifact(ctx context.Context, gav string) (*CachedArtifact, error)
    
    // FetchPOM retrieves a POM; returns content + metadata.
    FetchPOM(ctx context.Context, gav string) ([]byte, *ArtifactMeta, error)
}

// CachedArtifact represents a successfully cached artifact.
type CachedArtifact struct {
    Path          string             // Absolute path in .jpm/cache
    SHA256        string             // Computed checksum
    Size          int64              // Bytes
    LastFetched   time.Time
    Repository    string             // Which repo served it (for future mirror support)
    FromCache     bool               // Whether served from cache
}

// ArtifactMeta is metadata from repository or cache.
type ArtifactMeta struct {
    SHA256        string
    Size          int64
    Timestamp     time.Time
    SourceURL     string
    ContentType   string             // e.g., "application/java-archive"
}

// CacheIndex maps GAV -> cached artifact for quick lookup.
type CacheIndex struct {
    Entries       map[string]*CacheEntry
    Version       string             // Schema version
    LastUpdated   time.Time
}

type CacheEntry struct {
    GAV           string
    SHA256        string
    Path          string
    Size          int64
    Fetched       time.Time
    Repository    string
}

// Fetcher implementation config.
type NativeFetcher struct {
    CacheDir      string             // .jpm/cache
    RepositoryURL string             // Default: Maven Central
    Concurrency   int                // Worker pool size
    Timeout       time.Duration      // Per-artifact timeout
    RetryPolicy   *RetryPolicy
    OfflineMode   bool
    Logger        Logger
    Lock          sync.RWMutex       // Protect concurrent cache writes
}

type RetryPolicy struct {
    MaxAttempts   int           // Default: 3
    Backoff       []time.Duration // [1s, 2s, 4s]
}
```

---
## Cache Layout

```
.jpm/
  cache/
    index.json                                # Cache index (mapping GAV -> sha256)
    artifacts/
      abc123def456.../
        artifact.jar                          # Binary content
        metadata.json                         # {"size": 12345, "fetched": "..."}
      def456abc789.../
        artifact.jar
        pom.xml                              # For POMs
        metadata.json
```

---
## Algorithm

### 1. Fetch Flow
```
FetchArtifact(gav):
    1. Check lockfile: does it have SHA256 for this GAV?
       if yes:
           requestedSHA := lockfile[gav].sha256
       else:
           requestedSHA := "" (will compute after fetch)
    
    2. Check index: does cache have entry for (GAV, requestedSHA)?
       if yes && file exists:
           return CachedArtifact (FromCache=true)
    
    3. Download from repository:
       - Make HTTP GET request
       - Stream to temp file (.part)
       - Compute SHA256 on-the-fly
       - Compare vs requestedSHA if known
       
    4. Store in cache:
       - Move .part to artifacts/<sha256>/artifact.jar
       - Write metadata.json
       - Update index.json
    
    5. Return CachedArtifact (FromCache=false)
```

### 2. Cache Hit
```
1. Load index from .jpm/cache/index.json
2. Lookup (GAV, SHA256) -> CacheEntry
3. Check if file exists at entry.Path
4. If yes && matches SHA256: return path
5. Else: proceed to download
```

### 3. Cache Miss or Invalid
```
1. If --offline flag:
    - Log "Artifact <GAV> not in cache"
    - Add to missing list
    - Continue (collect all missing before failing)

2. Else (online):
    - Download with retries
    - Compute SHA256
    - Verify if lockfile expected SHA
    - If mismatch: warn, redownload once
    - Store in cache
```

### 4. Parallel Fetch
```
1. Collect all artifacts to fetch
2. Create worker pool (size = min(Concurrency, len(artifacts)))
3. Distribute work across workers
4. Each worker:
    - Pick artifact from queue
    - Call Fetch()
    - Append result (success/error) to results
5. Wait for all workers
6. Aggregate results + errors
```

---
## Network Implementation

### Repository Configuration
```go
type Repository struct {
    ID            string             // "maven-central"
    URL           string             // "https://repo.maven.apache.org/maven2"
    Authentication *Auth             // Optional
    Mirror        string             // Override URL if set
}

// Default repositories
DefaultRepositories = []Repository{
    {
        ID: "maven-central",
        URL: "https://repo.maven.apache.org/maven2",
    },
}
```

### URL Construction
```
GAV: org.slf4j:slf4j-api:2.0.16
Path: org/slf4j/slf4j-api/2.0.16/

Artifact URL:
  https://repo.maven.apache.org/maven2/org/slf4j/slf4j-api/2.0.16/slf4j-api-2.0.16.jar

POM URL:
  https://repo.maven.apache.org/maven2/org/slf4j/slf4j-api/2.0.16/slf4j-api-2.0.16.pom
```

### HTTP Client Setup
```go
client := &http.Client{
    Timeout: fetcher.Timeout,  // 30s per artifact
    Transport: &http.Transport{
        MaxIdleConns:       10,
        MaxIdleConnsPerHost: 5,
        DisableKeepAlives:  false,
    },
}

// Enforce HTTPS
if !strings.HasPrefix(repoURL, "https://"):
    return error("HTTP not allowed; use HTTPS")
```

### Retry Strategy
```
For attempt = 1 to MaxAttempts:
    try:
        response := http.Get(url, timeout)
        if response.StatusCode == 200:
            return response.Body
        else if response.StatusCode in [429, 500, 502, 503]:
            // Transient error; retry
            wait(backoff[attempt-1])
            continue
        else if response.StatusCode == 404:
            return NotFound (no retry)
        else if response.StatusCode in [401, 403]:
            return AccessDenied (no retry)
    catch timeout:
        wait(backoff[attempt-1])
        continue
    catch connection error:
        wait(backoff[attempt-1])
        continue

return error(all retries exhausted)
```

---
## Integrity Verification

### Checksum Computation
```go
func computeSHA256(reader io.Reader) (string, error) {
    hash := sha256.New()
    if _, err := io.Copy(hash, reader); err != nil {
        return "", err
    }
    return hex.EncodeToString(hash.Sum(nil)), nil
}
```

### Lockfile Verification
```
If lockfile has entry for GAV:
    expectedSHA := lockfile[GAV].sha256
    if downloadedSHA != expectedSHA:
        log Warning: "Checksum mismatch for <GAV>"
        Try download once more
        if still mismatches:
            fail with "Integrity check failed"
Else:
    Store downloaded SHA in lockfile
```

---
## Offline Mode

### Activation
```
jpm build --offline
```

### Behavior
```
1. Set fetcher.OfflineMode = true
2. For each artifact to fetch:
    - Check cache
    - If present: use cached version
    - If missing:
        - Add to "missing" list
        - Continue (don't fail immediately)
3. After collecting all artifacts:
    - If missing list non-empty:
        Report missing GAVs
        Suggest: jpm build (online) or pre-populate cache
        Exit with error
    - Else:
        Proceed to compile (all artifacts available)
```

---
## Concurrent Access & Locking

### Race Condition: Concurrent Writes
```
Thread A                          Thread B
Check cache: miss                 Check cache: miss
Download...                       Download...
Write cache/abc123/               Write cache/abc123/
  ← Two threads write same file!
```

### Solution: File-Level Lock
```go
// Acquire lock before writing
fetcher.Lock.Lock()
defer fetcher.Lock.Unlock()

// Double-check after lock acquired
if cacheEntry.Exists() && isValid():
    return cacheEntry  // Another thread beat us

// Proceed with write
writeCache(path, data)
updateIndex()
```

### Alternative: Atomic Rename
```go
// Write to temp file in same filesystem
tempFile := tempDir + "artifact.jar.tmp"
writeFile(tempFile, data)

// Atomic rename
os.Rename(tempFile, cacheFile)  // POSIX atomic
```

---
## Metrics & Monitoring

### Capture
```go
type FetchMetrics struct {
    TotalArtifacts  int
    CacheHits       int
    CacheMisses     int
    TotalBytes      int64
    Duration        time.Duration
    FailedFetches   int
    RetriesNeeded   int
}
```

### Output
```
Fetcher metrics:
  Total: 12 artifacts
  Cache hits: 8 (67%)
  Cache misses: 4 (33%)
  Total bytes: 45.2 MB
  Duration: 3.45s
  Retries: 2 (network transients)
```

---
## Error Handling

| Scenario | Code | Action |
|----------|------|--------|
| Artifact found | 200 | Store in cache, return |
| Artifact not found | 404 | Fail (suggest version typo) |
| Access denied | 403 | Fail (suggest credentials) |
| Server error | 500+ | Retry with backoff |
| Timeout | - | Retry with backoff |
| Checksum mismatch | - | Redownload once; if still mismatch, fail |
| Network error | - | Retry with backoff |
| Offline + cache miss | - | Fail with missing list |

---
## Caching Best Practices

### Cache Invalidation
1. **Manual**: `jpm cache clear` (not yet implemented; future).
2. **Automatic**: Lockfile mismatch triggers redownload.

### Duplicate Detection
```
If two processes try to fetch same GAV concurrently:
    - First acquires lock, downloads, stores
    - Second sees cache entry, returns it
    - No duplicate downloads
```

### Cross-Project Deduplication
```
Projects A and B both depend on org.slf4j:slf4j-api:2.0.16
    - First project: downloads & caches with sha256=abc123
    - Second project: finds abc123 in cache, reuses
    - Saves bandwidth & disk space
```

---
## Example Flow

```
=== Fetch org.slf4j:slf4j-api:2.0.16 ===

Step 1: Load index
  - Read .jpm/cache/index.json
  - Look for entry: org.slf4j:slf4j-api:2.0.16
  - Not found in index

Step 2: Check lockfile
  - lockfile[org.slf4j:slf4j-api:2.0.16].sha256 = "e3b0c44..."
  - Set expectedSHA = "e3b0c44..."

Step 3: Download
  - URL: https://repo.maven.apache.org/maven2/org/slf4j/slf4j-api/2.0.16/slf4j-api-2.0.16.jar
  - GET request
  - Stream to .jpm/cache/artifacts/e3b0c44.../artifact.jar.tmp
  - Compute SHA256: "e3b0c44..."
  - Match! expectedSHA == computed

Step 4: Store
  - Move .jar.tmp -> .jar
  - Write metadata.json
  - Add index entry: {"org.slf4j:slf4j-api:2.0.16": {...}}
  - Write index.json

Step 5: Return
  - CachedArtifact{
      Path: ".jpm/cache/artifacts/e3b0c44.../artifact.jar",
      SHA256: "e3b0c44...",
      Size: 12345,
      FromCache: false,
    }

=== Fetch again (cache hit) ===

Step 1: Load index
  - Read .jpm/cache/index.json
  - Find entry: org.slf4j:slf4j-api:2.0.16 -> sha256=e3b0c44...

Step 2: Verify file
  - Check .jpm/cache/artifacts/e3b0c44.../artifact.jar exists
  - Return CachedArtifact (FromCache=true)
  - Time: 10ms (cache local read)
```

---
## Testing

### Unit Tests
1. **Cache hit**: File exists, valid metadata.
2. **Cache miss**: File absent, fetch required.
3. **Checksum mismatch**: Redownload triggered.
4. **Offline mode**: Cache only, no network.
5. **Concurrent fetches**: No duplicate downloads.
6. **Retry backoff**: Exponential backoff on transient errors.

### Integration Tests
1. **Real fetch**: Download 5 artifacts; verify all cached.
2. **Reuse**: Build twice; second build faster.
3. **Offline**: Prime cache; build --offline; success.
4. **Network timeout**: Simulate timeout; verify retry + backoff.
5. **Checksum mismatch**: Corrupt cache; verify redownload.

---
## Implementation Checklist

- [ ] Define cache layout & index structure.
- [ ] Implement cache index load/save.
- [ ] Implement download with SHA256 computation.
- [ ] Implement cache storage (atomic rename).
- [ ] Implement cache hit detection.
- [ ] Implement retry logic with exponential backoff.
- [ ] Implement parallel fetch with worker pool.
- [ ] Implement offline mode.
- [ ] Implement concurrent access protection (locking).
- [ ] Implement HTTPS enforcement.
- [ ] Unit tests (all scenarios).
- [ ] Integration tests (real network).
- [ ] Benchmark (download speed, concurrency scalability).
