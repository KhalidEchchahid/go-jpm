# Resolver Implementation Guide

Date: 2025-11-09
Owner: Core Engine Team

---
## Overview
The Resolver transforms manifest dependencies into a complete, conflict-resolved dependency graph. It fetches POMs, parses metadata, applies version resolution, and detects conflicts using a nearest-wins policy.

---
## Data Model

```go
package resolver

// Graph represents the complete resolved dependency tree.
type Graph struct {
    Root          *Node              // Synthetic root or first dependency
    Nodes         map[string]*Node   // All unique GAVs
    Metadata      *ResolutionMeta
    UnresolvedIDs []string           // Missing/failed artifacts (partial resolution)
}

// Node represents a single artifact in the graph.
type Node struct {
    GAV              string            // "groupId:artifactId:version"
    GroupID          string
    ArtifactID       string
    Version          string
    Scope            core.DependencyScope
    Optional         bool
    Classifier       string
    
    // Graph structure
    Parents          []*Node           // Nodes that depend on this
    Children         []*Node           // Direct dependencies
    Distance         int               // Hops from root (0 = root)
    
    // Metadata
    Exclusions       []ExclusionRule   // e.g., "log4j:*"
    Properties       map[string]string // ${property} from POM
    Resolved         bool              // Successfully fetched & parsed
    CheckSum         string            // SHA-256 from lockfile (if available)
    
    // Conflict resolution
    Conflict         *ConflictInfo     // Other versions competed
}

// ExclusionRule represents a pattern to exclude transitive deps.
type ExclusionRule struct {
    GroupID    string // e.g., "org.slf4j"
    ArtifactID string // e.g., "*" (wildcard) or "slf4j-core"
}

// ConflictInfo tracks the loser in a version conflict.
type ConflictInfo struct {
    CandidateVersion string
    CandidateDistance int
    Reason           string // "nearest-wins"
}

// ResolutionMeta stores metadata about the resolution process.
type ResolutionMeta struct {
    Timestamp      time.Time
    Duration       time.Duration
    Strategy       string // "nearest-wins"
    ToolVersion    string
    RepoCount      int
    ArtifactsFetched int
}
```

---
## Algorithm

### 1. Initialization
```
Input: manifest (root dependencies), fetcher
Output: Graph

1. Create root node (synthetic or first dependency)
2. Initialize queue with root dependencies
3. Initialize visited set (avoid infinite loops)
4. Initialize conflict tracker
```

### 2. Breadth-First Traversal
```
While queue not empty:
    node := pop from queue
    
    if node in visited:
        continue  // Already processed
    
    mark node as visited
    
    // Fetch and parse POM
    pom := fetchPOM(node.GAV)
    
    // Apply parent inheritance if present
    if pom.parent:
        parentPOM := fetchPOM(pom.parent)
        mergeParent(pom, parentPOM)
    
    // Process dependencies
    for each dep in pom.dependencies:
        if dep.optional && not in manifest:
            skip  // Only include optional if root explicitly requested
        
        // Resolve version (simple for now; future: ranges)
        resolvedVersion := dep.version
        
        candidate := Node(
            GAV: dep.groupId : dep.artifactId : resolvedVersion,
            Scope: dep.scope,
            Optional: dep.optional,
            Exclusions: dep.exclusions,
            Distance: node.Distance + 1
        )
        
        // Check for conflict
        if existing := nodes[candidate.GAV.GroupID:ArtifactID]:
            winner := resolveConflict(existing, candidate)
            if winner == candidate:
                // Update existing or skip (already resolved closer)
                continue
        else:
            // New GAV; add to graph
            nodes[candidate.GAV] = candidate
            node.Children.append(candidate)
            candidate.Parents.append(node)
            queue.append(candidate)
```

### 3. Conflict Resolution (Nearest-Wins)
```
resolveConflict(existing, candidate):
    if existing.Distance < candidate.Distance:
        return existing  // Keep closer node
    
    if existing.Distance > candidate.Distance:
        // Update existing to use candidate version
        existing.Version = candidate.Version
        existing.Distance = candidate.Distance
        // Propagate distance to transitive children (adjust as needed)
        return existing (updated)
    
    // Tie: same distance
    // Prefer first encountered (existing) by default
    // Can add tie-breaking rule: prefer higher version
    if existing.Version >= candidate.Version:
        return existing
    else:
        return existing (updated to candidate)
```

### 4. Cycle Detection
```
detectCycles(node, visited, recursionStack):
    mark node as visited
    mark node as in recursionStack
    
    for each child in node.Children:
        if child not in visited:
            if detectCycles(child, visited, recursionStack):
                return true  // Cycle found
        else if child in recursionStack:
            // Cycle: reconstruct path
            path := reconstructPath(node, child)
            throw CycleError(path)
    
    mark node as not in recursionStack
    return false
```

### 5. Validation & Finalization
```
1. Run cycle detection from root
2. Collect all unresolved nodes (fetch failed, parse failed)
3. Topologically sort for deterministic ordering
4. Compute closure (ensure all transitive deps included)
5. Return Graph with metadata
```

---
## Integration with Fetcher

The Resolver calls Fetcher for:
1. **POM Fetch**: `fetcher.FetchPOM(gav) -> (xmlContent, metadata)`
2. **Artifact Fetch** (optional, for validation): `fetcher.FetchArtifact(gav) -> (cachedPath, metadata)`

### Error Handling
- **POM not found (404)**: Mark node as unresolved; continue.
- **POM parse error**: Log error; mark unresolved; continue.
- **Network error**: Retry via Fetcher; if all retries fail, mark unresolved.
- **Cycle**: Throw CycleError immediately; halt resolution.

---
## POM Model & Parsing

```go
type POMModel struct {
    GroupID          string
    ArtifactID       string
    Version          string
    Packaging        string // e.g., "jar"
    
    Parent           *GAV
    DependencyMgmt   []Dependency  // BOM entries (phase-gate)
    Dependencies     []Dependency
    Properties       map[string]string
}

type Dependency struct {
    GroupID      string
    ArtifactID   string
    Version      string
    Scope        string              // compile, test, provided, optional, runtime
    Optional     bool
    Classifier   string
    Type         string              // e.g., "jar"
    Exclusions   []ExclusionRule
}
```

### POM Parsing Logic
```
1. Parse XML
2. Extract groupId, artifactId, version
3. Handle parent resolution (inherit groupId/version if missing)
4. Extract properties (for ${property} substitution)
5. Parse dependencies:
   - Apply scope defaults
   - Mark optional=true if <optional>true</optional>
   - Parse exclusions
6. Substitute properties in versions (e.g., ${project.version} -> actual)
```

---
## Exclusion Matching

```go
matches(exclusionRule, dep) bool:
    if exclusionRule.GroupID != "*" && exclusionRule.GroupID != dep.GroupID:
        return false
    if exclusionRule.ArtifactID != "*" && exclusionRule.ArtifactID != dep.ArtifactID:
        return false
    return true
```

---
## Memoization

**Cache Structure**:
```go
type ResolverCache struct {
    POMs map[string]*POMModel      // GAV -> parsed POM
    Nodes map[string]*Node          // GAV -> resolved node
    Visited map[string]bool          // GAV -> already processed
}
```

**Benefits**:
- Avoid re-parsing same POM multiple times.
- Avoid re-resolving same GAV across branches.

---
## Example Flow

```
Manifest:
  dependencies:
    - org.slf4j:slf4j-api:2.0.16
    - com.google.guava:guava:33.0.0-jre

=== Resolve ===

Queue: [slf4j-api:2.0.16, guava:33.0.0-jre]
Nodes: {}

Step 1: Process slf4j-api:2.0.16
  - Fetch POM
  - Parse: no parent, no deps
  - Mark Resolved
  - Queue: [guava:33.0.0-jre]

Step 2: Process guava:33.0.0-jre
  - Fetch POM
  - Parse: has 3 dependencies:
    - com.google.failureaccess:failure-access:1.0.1 (compile)
    - com.google.guava:listenablefuture:9999.0-empty-to-avoid-conflict-with-guava (compile)
    - org.checkerframework:checker-qual:4.1.0 (compile)
    - org.checkerframework:checker-incompatibility-qual:2.40.0 (compile)
  - Enqueue all 4

Steps 3-6: Process failure-access, listenablefuture, etc.
  - Each has no further deps

Result Graph:
  Root
    ├─ org.slf4j:slf4j-api:2.0.16 (distance=1, scope=compile)
    └─ com.google.guava:guava:33.0.0-jre (distance=1, scope=compile)
        ├─ com.google.failureaccess:failure-access:1.0.1 (distance=2, scope=compile)
        ├─ com.google.guava:listenablefuture:9999.0-empty-to-avoid-conflict-with-guava (distance=2, scope=compile)
        ├─ org.checkerframework:checker-qual:4.1.0 (distance=2, scope=compile)
        └─ org.checkerframework:checker-incompatibility-qual:2.40.0 (distance=2, scope=compile)

Lockfile generated with all 7 artifacts, sorted, with checksums.
```

---
## Testing

### Unit Tests
1. **Simple graph**: 1 dep, no transitive.
2. **Linear chain**: A -> B -> C (3 levels).
3. **Conflict resolution**: A → C v1, B → C v2 (verify v1 wins if A closer).
4. **Exclusions**: Dep with exclusion rule; verify excluded child not added.
5. **Cycle detection**: A → B → A (verify error).
6. **Optional handling**: Manifest vs optional deps.

### Integration Tests
1. **Real project**: Init, add 5 real deps, resolve, verify lockfile structure.
2. **Cache reuse**: Resolve twice; verify second uses cached POMs.
3. **Offline**: Prepopulate cache; resolve --offline; verify no network calls.

---
## Implementation Checklist

- [ ] Define Graph/Node/ResolutionMeta structs.
- [ ] Implement BFS traversal with queue.
- [ ] Implement conflict resolution (nearest-wins).
- [ ] Implement cycle detection (DFS).
- [ ] Implement POM parsing (XML unmarshalling).
- [ ] Implement parent inheritance.
- [ ] Implement property substitution.
- [ ] Implement exclusion matching.
- [ ] Implement memoization (cache POMs + nodes).
- [ ] Integrate Fetcher (handle network errors gracefully).
- [ ] Unit tests (all scenarios).
- [ ] Integration tests (real graph).
- [ ] Benchmarks (time, memory for large graphs).
