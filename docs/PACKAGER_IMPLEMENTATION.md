# Packager (JAR) Implementation Guide

Date: 2025-11-09T15:52:08.717Z
Owner: Core Engine Team
Status: Implementation Reference

---
## Overview
Create runnable JARs from .jpm/work/classes to .jpm/out, embedding a deterministic manifest and optional Main-Class.

---
## Interface
```go
type Packager interface {
  Package(ctx context.Context, classesDir, outDir, artifact, version, mainClass string, includeMeta bool) (string, error)
}
```

---
## Behavior
- Input: compiled classes directory; optional mainClass; artifact/version from manifest.project.
- Output: jarPath = .jpm/out/<artifact>-<version>.jar; ensure outDir exists.
- Use archive/zip to add entries; store with normalized path separators ('/') regardless of OS.
- Deterministic ordering: collect files, sort by path; write with fixed permissions (0644) and zeroed modified time (Unix epoch) for reproducibility.
- Manifest (META-INF/MANIFEST.MF):
  - Manifest-Version: 1.0
  - Created-By: JPM <toolVersion>
  - Implementation-Version: <version>
  - Main-Class: <mainClass> (only if non-empty)
- Optionally include META-INF/INDEX.LIST later (defer for simplicity).

---
## Main-Class Detection
- If mainClass empty: optionally scan classes for `public static void main(String[] args)`; if 1 found, use it; if >1, prompt user in CLI layer.

---
## Error Handling
- If classesDir empty: return error "no classes to package".
- If write fails: include path in error; suggest checking permissions.

---
## Testing
- Unit: package simple class; verify manifest entries; verify deterministic JAR (byte-identical on repeat).
- Integration: compile + package + run via `java -cp`.
