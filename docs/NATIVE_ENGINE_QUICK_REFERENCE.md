# Native Engine Quick Reference

## Quick Test Commands

### 1. **Basic Build**
```bash
./jpm build project_test
```
- Compiles all Java sources
- Creates JAR in `.jpm/out/`
- Generates lockfile `jpm.lock.yaml`
- Typical duration: 300-400ms

### 2. **Verbose Build** (See all build phases)
```bash
./jpm build -v project_test
```
Shows:
- Directory setup
- Manifest configuration
- Builder initialization
- Dependency resolution
- Source compilation command
- Lockfile generation
- Final timing

### 3. **Verify JAR Contents**
```bash
jar tf project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar
```
Expected:
- `Main.class`
- `META-INF/MANIFEST.MF`
- Any other compiled classes

### 4. **Check Manifest**
```bash
jar xf project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar META-INF/MANIFEST.MF
cat META-INF/MANIFEST.MF
rm -rf META-INF
```
Expected Main-Class: `Main`

### 5. **Inspect Lockfile**
```bash
cat project_test/jpm.lock.yaml
```
Should show:
- `lockfile_format: 1`
- `created_at` timestamp
- `resolver: native`
- Maven Central repository
- Dependencies (empty for now)

## Build Configuration (project_test/jpm.yaml)

```yaml
name: project_test
build_tool: maven
engine: native              # ← Set to "native" for native engine
java:
  version: "21"            # Source/target Java version
project:
  group_id: app            # Used as artifact prefix
  artifact_id: project-test
  version: 0.1.0-SNAPSHOT
app:
  main_class: Main         # ← Set in JAR manifest
dependencies: []           # (Not yet fetched - Sprint 7)
```

## Directory Structure After Build

```
project_test/
├── src/
│   └── *.java              # Source files
├── jpm.yaml                # Project manifest
├── jpm.lock.yaml           # Generated lockfile
└── .jpm/
    ├── out/
    │   └── app-0.1.0-SNAPSHOT.jar  # Compiled JAR
    └── work/
        └── classes/        # Compiled .class files
```

## Compiler Flags Used

The native engine compiles with these flags:
```
javac -encoding UTF-8 \
      -g \
      -Xlint:deprecation \
      -Xlint:unchecked \
      -source 21 \           # From jpm.yaml
      -target 21 \           # From jpm.yaml
      -d <work-dir>/classes \
      -cp <classpath> \
      *.java
```

## What Works ✅

- [x] Single & multiple Java file compilation
- [x] JAR creation with proper structure
- [x] Manifest generation with Main-Class
- [x] Lockfile generation (deterministic format)
- [x] Verbose build output with phase logging
- [x] Custom artifact names/versions from config
- [x] Custom Java source/target versions
- [x] No external dependencies (just Java)
- [x] Fast builds (~300-400ms)

## What Doesn't Work Yet ❌

- [ ] Dependency downloading (needs Fetcher integration)
- [ ] Transitive dependencies
- [ ] Version conflict resolution
- [ ] Maven properties in POMs
- [ ] Plugin configuration parsing
- [ ] Scope-based filtering (test/provided)

## Performance Baseline

| Scenario | Duration | Notes |
|----------|----------|-------|
| Simple project (1 source) | ~350ms | Initial JVM startup |
| Consecutive builds | ~300ms | Consistent after warmup |
| Multiple files (3 sources) | ~350ms | Minimal overhead |
| With lockfile write | ~20ms | Additional to build time |

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "no jpm.yaml found" | Use absolute path: `./jpm build /path/to/project_test` |
| Compilation errors | Check for syntax errors in Java files |
| JAR not created | Look for compilation errors in verbose output: `./jpm build -v` |
| Empty JAR | Check if .class files were compiled to work/classes/ |
| Lockfile missing | Check write permissions in project directory |

## Next Steps (Sprint 7+)

After verifying all tests pass:

1. **Enable fetcher** - Download dependencies from Maven Central
2. **Add transitive resolution** - Resolve dependency trees
3. **Test with real dependencies** - commons-lang3, slf4j, etc.
4. **Benchmark vs Maven** - Performance comparison
5. **Add `deps` commands** - Tree, audit, sync functionality
