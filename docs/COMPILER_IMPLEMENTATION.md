# Compiler Implementation Guide

Date: 2025-11-09T15:52:08.717Z
Owner: Core Engine Team
Status: Implementation Reference

---
## Overview
Compile Java sources from a flat src/ into .jpm/work/classes using javac, with classpath built from the native resolver. Focus on determinism, clear errors, and speed.

---
## Interface
```go
type Compiler interface {
  Compile(ctx context.Context, srcDir, outDir string, classpath []string, javaVersion string) (*CompileResult, error)
}

type CompileResult struct {
  Sources     int
  Classes     int
  Duration    time.Duration
  Warnings    []string
}
```

---
## Behavior
- Source discovery: recursively find *.java under src/; skip non-UTF8 files with warning.
- Ensure .jpm/work/classes exists; clean on full build; incremental is future work.
- Build classpath string using OS-specific separator; include .jpm/work/classes first when recompile.
- Invoke: `javac -encoding UTF-8 -g -Xlint:deprecation -Xlint:unchecked -source <ver> -target <ver> -d <outDir> -cp <classpath> <files...>`.
- Capture stdout/stderr; parse lines into warnings and errors; surface first N errors with file:line.
- Return error code if javac exits non-zero.

---
## Options
- javaVersion: default from manifest.java.version; validate among {17,21,23}.
- classpath: include compile + provided scopes for compilation; exclude test.
- Limits: when >10k files, chunk arguments to avoid OS argv limits; use @argfile for javac if needed.

---
## Determinism
- Normalize file ordering: sort source list lexicographically before compile (affects reproducible diagnostics).
- Do not embed timestamps in classes (javac behavior acceptable; reproducible JAR handled by packager).

---
## Errors & Messages
- If no sources: succeed with 0 classes; print hint to add src/Main.java.
- If java not found: suggest installing JDK (Phase 2 installer), or set JAVA_HOME.
- If classpath artifact missing in offline mode: fail with list of missing GAVs.

---
## Testing
- Unit: builds with 1 file; build with syntax error; deprecation warning captured; invalid java version.
- Integration: resolve deps, compile referencing external classes; large file count chunking; offline compile using cache.
