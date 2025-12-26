# Classpath Assembly Guide

Date: 2025-11-09T15:52:08.717Z
Owner: Core Engine Team
Status: Implementation Reference

---
## Overview
Compute classpaths for compile, runtime, and test from the resolved dependency graph and cache.

---
## Interface
```go
type ClasspathBuilder interface {
  ForCompile(graph *resolver.Graph) ([]string, error)
  ForRuntime(graph *resolver.Graph, jarPath string) ([]string, error)
  ForTest(graph *resolver.Graph) ([]string, error)
}
```

---
## Rules
- Compile: include scopes {compile, provided}; exclude {test, optional unless explicitly requested}.
- Runtime: include {compile} minus {provided}; include optional only if root declared optional explicitly.
- Test: include {compile, test}; exclude provided from packaged run.

---
## Implementation
- For each node in graph (topologically sorted), filter by scope and exclusions already applied by resolver.
- Map each GAV to a cached path via fetcher/cache; error if missing and not offline-tolerant path.
- Build OS-specific classpath string with path.ListSeparator when invoking javac/java.
- Ensure project classes dir (.jpm/work/classes) or jarPath is first entry for precedence.

---
## Testing
- Graph with provided dep excluded at runtime; optional dep excluded unless explicitly turned on.
- Classpath order stable across runs (sorted by GAV then by path).
