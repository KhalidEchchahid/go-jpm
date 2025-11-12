# Complete Testing Steps for JPM Native Engine

## Prerequisites

Ensure you're in the go-jpm directory and have built the binary:

```bash
cd /home/eamonamkassou/Work/go-jpm
go build ./cmd/jpm
```

## Test Sequence

### TEST 1: Basic Compilation (No Dependencies)

**Clear previous builds:**
```bash
rm -rf project_test/.jpm/out project_test/jpm.lock.yaml
```

**Run build:**
```bash
./jpm build project_test
```

**Expected Output:**
```
→ building (native engine)
  compiled 1 source files
  packaged /home/eamonamkassou/Work/go-jpm/project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar
  duration 350ms
```

**Verify:**
```bash
# Check JAR exists
[ -f project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar ] && echo "✓ JAR created"

# Check lockfile exists
[ -f project_test/jpm.lock.yaml ] && echo "✓ Lockfile created"

# Check size
ls -lh project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar
```

---

### TEST 2: Verbose Build Output

**Run with verbose flag:**
```bash
rm -rf project_test/.jpm/out project_test/jpm.lock.yaml
./jpm build -v project_test
```

**Verify you see these phases:**
- `setup project directories`
- `manifest artifact: app version: 0.1.0-SNAPSHOT java: 21 main: Main`
- `build builder initialized`
- `resolve creating dependency graph`
- `resolve no dependencies to resolve`
- `compile source from ...`
- `Running: javac -encoding UTF-8 ...`
- `Compiled ... sources to ... classes`
- `package writing lockfile`
- `compiled 1 source files`
- `packaged .../app-0.1.0-SNAPSHOT.jar`
- `duration XXXms`

---

### TEST 3: JAR Contents Verification

**List JAR contents:**
```bash
jar tf project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar
```

**Verify contains:**
- `Main.class` ✓
- `META-INF/MANIFEST.MF` ✓

**Extract and verify manifest:**
```bash
jar xf project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar META-INF/MANIFEST.MF
cat META-INF/MANIFEST.MF
rm -rf META-INF
```

**Expected manifest:**
```
Manifest-Version: 1.0
Created-By: JPM 0.0.1
Implementation-Version: 0.1.0-SNAPSHOT
Main-Class: Main
```

---

### TEST 4: Multiple Source Files

**Create additional source files:**
```bash
cat > project_test/src/Helper.java << 'EOF'
public class Helper {
    public static String help() {
        return "Helper";
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
```

**Run build:**
```bash
rm -rf project_test/.jpm/out project_test/jpm.lock.yaml
./jpm build project_test
```

**Verify:**
```bash
# Should show "compiled 3 source files"
./jpm build project_test | grep compiled

# Check JAR contains all classes
jar tf project_test/.jpm/out/app-0.1.0-SNAPSHOT.jar | grep -E "\.(class|MF)"
```

**Cleanup:**
```bash
rm project_test/src/Helper.java project_test/src/Utils.java
```

---

### TEST 5: Build Performance

**Run 5 consecutive builds:**
```bash
for i in {1..5}; do
  echo "Run $i:"
  rm -rf project_test/.jpm/out project_test/jpm.lock.yaml
  ./jpm build project_test 2>&1 | grep duration
done
```

**Expected:** Each run 300-400ms, consistent timing

---

### TEST 6: Lockfile Format

**Examine lockfile:**
```bash
cat project_test/jpm.lock.yaml
```

**Verify it contains:**
- `lockfile_format: 1` ✓
- `created_at: "2025-11-12T..."` (RFC3339 format) ✓
- `tool_version: 0.0.1` ✓
- `resolver: native` ✓
- `repositories:` section with maven-central ✓
- `dependencies: []` (empty array) ✓

---

### TEST 7: Project Configuration

**Check manifest configuration:**
```bash
cat project_test/jpm.yaml
```

**Verify:**
- `engine: native` (triggers native pipeline) ✓
- `java.version: "21"` (used for javac -source/-target) ✓
- `project.artifact_id: project-test` (used in JAR name) ✓
- `project.version: 0.1.0-SNAPSHOT` (used in JAR name) ✓
- `app.main_class: Main` (set in manifest) ✓

**Verify JAR name matches config:**
```bash
ls project_test/.jpm/out/
# Should show: app-0.1.0-SNAPSHOT.jar
# Format: {artifact_id}-{version}.jar
```

---

### TEST 8: Compiler Flags

**Run verbose build and check javac command:**
```bash
./jpm build -v project_test 2>&1 | grep "Running: javac"
```

**Verify it contains:**
- `-encoding UTF-8` (consistent character encoding) ✓
- `-g` (include debug symbols) ✓
- `-Xlint:deprecation` (warn on deprecated API usage) ✓
- `-Xlint:unchecked` (warn on unchecked operations) ✓
- `-source 21` (source version from jpm.yaml) ✓
- `-target 21` (target version from jpm.yaml) ✓
- `-d .../classes` (output directory) ✓
- `-cp ...` (classpath specification) ✓

---

### TEST 9: Directory Structure

**Examine build directory structure:**
```bash
find project_test/.jpm -type f -o -type d | sort
```

**Verify structure:**
```
project_test/.jpm/
├── out/
│   └── app-0.1.0-SNAPSHOT.jar
└── work/
    └── classes/
        └── Main.class
```

**Verify no Maven directories** (native engine doesn't use Maven):
```bash
ls project_test/.jpm/ | grep -i maven
# Should have NO output
```

---

### TEST 10: Build Path Flexibility

**Test building with different paths:**
```bash
# Explicit relative path
./jpm build ./project_test

# Absolute path
./jpm build /home/eamonamkassou/Work/go-jpm/project_test

# From project directory
cd project_test && /home/eamonamkassou/Work/go-jpm/jpm build . && cd ..
```

**Verify:** All three produce identical results ✓

---

### TEST 11: Error Handling

**Create a Java file with syntax error:**
```bash
cat > project_test/src/Broken.java << 'EOF'
public class Broken {
    public static void main(String[] args {  // Missing closing paren
        System.out.println("Hello");
    }
}
EOF
```

**Attempt to build:**
```bash
./jpm build project_test 2>&1
```

**Verify:**
- Compilation error reported ✓
- Non-zero exit code ✓
- No JAR created ✓

**Cleanup:**
```bash
rm project_test/src/Broken.java
```

---

### TEST 12: All Unit Tests Pass

**Run complete test suite:**
```bash
go test ./...
```

**Verify:**
- All 76 tests pass ✓
- 0 failures ✓
- 0 skips ✓

---

## Summary Checklist

After running all tests, you should have verified:

- [ ] Basic build creates JAR and lockfile
- [ ] Verbose output shows all build phases
- [ ] JAR contains expected classes and manifest
- [ ] Manifest has correct Main-Class value
- [ ] Multiple source files compile correctly
- [ ] Build time is consistent and fast (~300-400ms)
- [ ] Lockfile is valid YAML with correct format
- [ ] Project paths work with different formats
- [ ] Project configuration read correctly
- [ ] Compiler flags all present
- [ ] Directory structure created properly
- [ ] Build errors handled gracefully
- [ ] All 76 unit tests still pass

## Documentation Files

Refer to these for more details:

1. **NATIVE_ENGINE_TESTING_GUIDE.md** - 12 detailed test scenarios with explanations
2. **NATIVE_ENGINE_QUICK_REFERENCE.md** - Quick commands and troubleshooting
3. **PHASE2_SPRINT6_COMPLETION.md** - Full Sprint 6 achievements and documentation
