# Native Engine Testing Guide

This guide provides step-by-step instructions to test all current functionality of the JPM native engine.

## Prerequisites

- Java 21+ installed and in PATH
- Go 1.21+ (to build jpm)
- Git
- Access to the go-jpm repository

## Quick Start

### Build the jpm binary

```bash
cd /home/eamonamkassou/Work/go-jpm
go build ./cmd/jpm
```

Expected output: No errors, `jpm` binary created in project root.

## Test Scenarios

### 1. Basic Native Build (No Dependencies)

**Purpose**: Verify the core compile → package → lockfile pipeline works.

**Steps**:

```bash
# Clean previous build outputs
rm -rf project_test/.jpm/out project_test/jpm.lock.yaml

# Run native build with default (concise) output
./jpm build project_test
```

**Expected Output**:
```
→ building (native engine)
  compiled 1 source files
  packaged /path/to/project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar
  duration 300-400ms
```

**Verification**:
```bash
# Verify JAR was created
ls -lh project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar
# Expected: ~900 bytes

# Verify lockfile was created
cat project_test/jpm.lock.yaml
# Expected: YAML with empty dependencies array

# Verify JAR contents
jar tf project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar
# Expected: META-INF/MANIFEST.MF and Main.class

# Verify manifest has correct main class
jar xf project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar META-INF/MANIFEST.MF
cat META-INF/MANIFEST.MF
rm -rf META-INF
# Expected: Main-Class: Main
```

---

### 2. Verbose Build Output

**Purpose**: Verify detailed logging for each build phase.

**Steps**:

```bash
# Clean previous build
rm -rf project_test/.jpm/out project_test/jpm.lock.yaml

# Run with verbose flag
./jpm build -v project_test
```

**Expected Output** (includes all these phases):
```
  setup project directories
→ building (native engine)
  manifest artifact: app version: 0.1.0-SNAPSHOT java: 21 main: Main
  build builder initialized
  resolve creating dependency graph
  resolve no dependencies to resolve
  compile source from /path/to/project_test/src
Compiling with classpath: []
Running: javac -encoding UTF-8 -g ...
Compiled 1 sources to 1 classes
  package writing lockfile to /path/to/project_test/jpm.lock.yaml
  compiled 1 source files
  packaged /path/to/project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar
  duration XXXms
```

**Verification**:
- All build phases logged
- Source directory path shown
- Javac command visible with all flags
- Duration shown in milliseconds
- Lockfile write confirmed

---

### 3. Manifest Configuration

**Purpose**: Verify that project configuration is read from jpm.yaml.

**Steps**:

```bash
# Check current project_test configuration
cat project_test/jpm.yaml
```

**Expected Content**:
```yaml
name: project_test
build_tool: maven
engine: native
java:
  version: "21"
project:
  group_id: app
  artifact_id: project-test
  version: 0.1.0-SNAPSHOT
app:
  main_class: Main
dependencies: []
```

**Verification**:
```bash
# Verify artifact ID is used in JAR name
./jpm build project_test
ls -1 project_test/.jpm/out/
# Expected: app-0.1.0-SNAPSHOT.jar (artifact_id + version)

# Verify main class is set correctly
jar xf project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar META-INF/MANIFEST.MF
grep "Main-Class" META-INF/MANIFEST.MF
rm -rf META-INF
# Expected: Main-Class: Main
```

---

### 4. Multiple Source Files

**Purpose**: Verify the compiler handles multiple Java files.

**Steps**:

```bash
# Create additional Java files in project_test/src
cat > project_test/src/Helper.java << 'EOF'
public class Helper {
    public static String getHelp() {
        return "Help";
    }
}
EOF

cat > project_test/src/Utils.java << 'EOF'
public class Utils {
    public static void printUtils() {
        System.out.println("Utils");
    }
}
EOF

# Clean and rebuild
rm -rf project_test/.jpm/out project_test/jpm.lock.yaml

# Build with verbose output
./jpm build -v project_test
```

**Expected Output**:
```
  compiled 3 source files
  packaged /path/to/project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar
```

**Verification**:
```bash
# Verify all classes were compiled
jar tf project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar
# Expected: Main.class, Helper.class, Utils.class, META-INF/MANIFEST.MF

# Verify file count
jar tf project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar | wc -l
# Expected: 4 (3 classes + manifest)
```

---

### 5. Build Performance

**Purpose**: Measure build performance for simple projects.

**Steps**:

```bash
# Run 5 consecutive builds and track duration
echo "=== Build Performance Test ==="
for i in {1..5}; do
  rm -rf project_test/.jpm/out project_test/jpm.lock.yaml
  echo "Run $i:"
  ./jpm build project_test | grep duration
done
```

**Expected Output**:
```
Run 1:
  duration 350-400ms
Run 2:
  duration 300-350ms
Run 3:
  duration 300-350ms
Run 4:
  duration 300-350ms
Run 5:
  duration 300-350ms
```

**Analysis**:
- First run slightly slower (JVM warmup, initial compilation)
- Subsequent runs consistent (~300-350ms)
- No performance degradation over multiple runs

---

### 6. Lockfile Generation & Format

**Purpose**: Verify lockfile is created with correct format and structure.

**Steps**:

```bash
# Clean and rebuild
rm -rf project_test/.jpm/out project_test/jpm.lock.yaml
./jpm build project_test

# Examine lockfile
cat project_test/jpm.lock.yaml
```

**Expected Content**:
```yaml
lockfile_format: 1
created_at: "2025-11-12T..."
tool_version: 0.0.1
resolver: native
repositories:
    - id: maven-central
      url: https://repo.maven.apache.org/maven2
dependencies: []
```

**Verification**:
- Lockfile is valid YAML
- Contains proper metadata
- Timestamp in RFC3339 format
- Tool version matches
- Resolver is "native"

```bash
# Validate YAML syntax
./jpm build project_test > /dev/null
cat project_test/jpm.lock.yaml | head -5
# Should parse without errors

# Check timestamp format
grep "created_at" project_test/jpm.lock.yaml
# Expected: created_at: "YYYY-MM-DDTHH:MM:SSZ"
```

---

### 7. Build with Different Project Paths

**Purpose**: Verify jpm build works with different directory structures.

**Steps**:

```bash
# Build with explicit path
./jpm build ./project_test

# Build with relative path
cd /tmp && /home/eamonamkassou/Work/go-jpm/jpm build /home/eamonamkassou/Work/go-jpm/project_test

# Build current directory
cd project_test && /home/eamonamkassou/Work/go-jpm/jpm build .
```

**Expected Output**:
All three commands should produce identical results with:
- Same JAR file created
- Same lockfile generated
- Same compilation output

---

### 8. Build Failure Handling

**Purpose**: Verify proper error handling when build fails.

**Steps**:

```bash
# Create a syntax error in Java file
cat > project_test/src/Broken.java << 'EOF'
public class Broken {
    public static void main(String[] args {  // Missing closing paren
        System.out.println("Hello");
    }
}
EOF

# Attempt build
./jpm build project_test 2>&1
```

**Expected Output**:
- Error message about compilation failure
- Proper error code (non-zero exit)
- No JAR file created (or previous JAR preserved)

**Cleanup**:
```bash
# Remove the broken file
rm project_test/src/Broken.java
```

---

### 9. Verify Java Compiler Flags

**Purpose**: Verify javac is invoked with proper compiler flags.

**Steps**:

```bash
# Run build with verbose mode to see javac invocation
./jpm build -v project_test 2>&1 | grep "Running: javac"
```

**Expected Output**:
```
Running: javac -encoding UTF-8 -g -Xlint:deprecation -Xlint:unchecked -source 21 -target 21 -d <workdir>/classes -cp <classpath> <source-file>
```

**Verification** - Flags should include:
- `-encoding UTF-8` (consistent encoding)
- `-g` (debug info)
- `-Xlint:deprecation` (warn on deprecated usage)
- `-Xlint:unchecked` (warn on unchecked operations)
- `-source 21` (source version from jpm.yaml)
- `-target 21` (target version from jpm.yaml)
- Classpath specification
- Work directory for class output

---

### 10. Dependency Graph Creation (No Fetch)

**Purpose**: Verify dependency graph structure when dependencies are present (not yet fetched).

**Steps**:

```bash
# Add a dependency to jpm.yaml
cat > project_test/jpm.yaml << 'EOF'
name: project_test
build_tool: maven
engine: native
java:
  version: "21"
project:
  group_id: app
  artifact_id: project-test
  version: 0.1.0-SNAPSHOT
app:
  main_class: Main
dependencies:
  - group_id: org.slf4j
    artifact_id: slf4j-api
    version: 2.0.16
    scope: compile
EOF

# Build with verbose output
./jpm build -v project_test
```

**Expected Output**:
```
  resolve creating dependency graph
  resolve added 1 dependency(ies) to graph
```

**Current Limitation**:
- Dependency is added to graph but NOT downloaded
- Compilation proceeds with empty classpath (will fail if code uses the dependency)
- This is expected behavior - fetcher integration is Sprint 7

**Revert for next tests**:
```bash
cat > project_test/jpm.yaml << 'EOF'
name: project_test
build_tool: maven
engine: native
java:
  version: "21"
project:
  group_id: app
  artifact_id: project-test
  version: 0.1.0-SNAPSHOT
app:
  main_class: Main
dependencies: []
EOF
```

---

### 11. Output Directory Structure

**Purpose**: Verify the .jpm directory structure created during build.

**Steps**:

```bash
# Build the project
./jpm build project_test

# Examine directory structure
find project_test/.jpm -type f -o -type d | sort
```

**Expected Structure**:
```
project_test/.jpm/
├── out/
│   └── app-0.1.0-SNAPSHOT.jar
└── work/
    └── classes/
        ├── Main.class
        └── [other compiled classes]
```

**Verification**:
```bash
# Verify out directory
ls -lah project_test/.jpm/out/
# Expected: app-0.1.0-SNAPSHOT.jar (~900 bytes)

# Verify work directory
ls -lah project_test/.jpm/work/classes/
# Expected: *.class files for each Java file

# Verify no cached Maven directory
ls -la project_test/.jpm/ | grep maven
# Expected: No output (native engine doesn't use Maven bridge)
```

---

### 12. Verbose + Debug Mode

**Purpose**: Verify debug mode combines with verbose for enhanced output.

**Steps**:

```bash
# Build with both verbose and debug flags
./jpm build -v -d project_test 2>&1 | head -30
```

**Expected Output**:
- All verbose build phase information
- Additional debug details (if debug mode adds logging)
- No errors or warnings

---

## Comprehensive Test Script

Run all tests automatically:

```bash
#!/bin/bash
set -e

PROJECT_ROOT="/home/eamonamkassou/Work/go-jpm"
cd "$PROJECT_ROOT"

echo "=== JPM Native Engine Test Suite ==="
echo ""

# Test 1: Basic build
echo "Test 1: Basic native build"
rm -rf project_test/.jpm/out project_test/jpm.lock.yaml
./jpm build project_test > /dev/null
[ -f project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar ] && echo "✓ JAR created" || echo "✗ JAR missing"
[ -f project_test/jpm.lock.yaml ] && echo "✓ Lockfile created" || echo "✗ Lockfile missing"

# Test 2: Verbose output
echo ""
echo "Test 2: Verbose output"
rm -rf project_test/.jpm/out project_test/jpm.lock.yaml
OUTPUT=$(./jpm build -v project_test)
echo "$OUTPUT" | grep -q "setup" && echo "✓ Setup phase logged" || echo "✗ Setup phase missing"
echo "$OUTPUT" | grep -q "manifest" && echo "✓ Manifest phase logged" || echo "✗ Manifest phase missing"
echo "$OUTPUT" | grep -q "resolve" && echo "✓ Resolution phase logged" || echo "✗ Resolution phase missing"

# Test 3: JAR contents
echo ""
echo "Test 3: JAR contents verification"
JAR_PATH="project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar"
jar tf "$JAR_PATH" | grep -q "Main.class" && echo "✓ Main.class present" || echo "✗ Main.class missing"
jar tf "$JAR_PATH" | grep -q "META-INF/MANIFEST.MF" && echo "✓ Manifest present" || echo "✗ Manifest missing"

# Test 4: Manifest main class
echo ""
echo "Test 4: Manifest main class"
jar xf "$JAR_PATH" META-INF/MANIFEST.MF
grep -q "Main-Class: Main" META-INF/MANIFEST.MF && echo "✓ Main-Class correct" || echo "✗ Main-Class incorrect"
rm -rf META-INF

# Test 5: Performance (should be < 500ms)
echo ""
echo "Test 5: Build performance"
rm -rf project_test/.jpm/out project_test/jpm.lock.yaml
START=$(date +%s%N)
./jpm build project_test > /dev/null
END=$(date +%s%N)
DURATION=$((($END - $START) / 1000000))
[ $DURATION -lt 500 ] && echo "✓ Build completed in ${DURATION}ms" || echo "⚠ Build took ${DURATION}ms (longer than expected)"

echo ""
echo "=== All Tests Complete ==="
```

Save this as `test-native-engine.sh`, make it executable, and run:
```bash
chmod +x test-native-engine.sh
./test-native-engine.sh
```

---

## Troubleshooting

### Build fails with "no jpm.yaml found"
```bash
# Solution: Ensure you're in correct directory or use absolute path
./jpm build /path/to/project_test
```

### Java compilation errors
```bash
# Verify Java is installed and in PATH
java -version
javac -version

# Clean and rebuild
rm -rf project_test/.jpm
./jpm build project_test
```

### JAR file not created
```bash
# Check for compilation errors
./jpm build -v project_test 2>&1 | grep -i error

# Verify source files exist
ls -la project_test/src/
```

### Lockfile not created
```bash
# Check error messages
./jpm build -v project_test 2>&1 | grep -i "warning\|error"

# Verify .jpm directory exists
ls -la project_test/.jpm/
```

---

## Summary of Current Capabilities

✅ **Working Features**:
- Compile single and multiple Java source files
- Package compiled classes into JAR with proper manifest
- Generate deterministic lockfiles in YAML format
- Support for verbose logging and debugging
- Build without external dependencies (no Maven required)
- Configurable artifact name, version, main class
- Support for different source Java versions (21, 17, etc.)

❌ **Not Yet Implemented** (Sprint 7+):
- Dependency downloading from Maven Central
- Transitive dependency resolution
- Version conflict resolution
- Scope-based filtering (test/provided)
- Maven property substitution
- Plugin configuration parsing

---

## Next Steps

After verifying these features work correctly:

1. **Prepare for Sprint 7**: Dependency fetching
2. **Test with dependencies**: Once fetcher is integrated
3. **Performance profiling**: Identify bottlenecks
4. **CLI enhancements**: Add additional commands (deps tree, audit)
