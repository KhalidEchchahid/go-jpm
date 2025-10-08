# Technical Implementation Deep Dive

**Detailed explanation of JPM's architecture and implementation**

---

## 📐 Architecture Overview

JPM follows a clean, layered architecture inspired by Hexagonal/Ports & Adapters pattern:

```
┌─────────────────────────────────────────────────┐
│              CLI Layer (cmd/jpm)                │
│  • Cobra commands                               │
│  • Flag parsing                                 │
│  • User interaction                             │
└────────────┬────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────────┐
│           Core Domain (internal/core)           │
│  • BuildTool type                               │
│  • ProjectInspector interface                   │
│  • InspectorFactory                             │
└────────────┬────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────────┐
│        Adapters (internal/adapters)             │
│  • maven.MavenProjectInspector                  │
│  • gradle.GradleProjectInspector                │
│  • (future: sbt, bazel, etc.)                   │
└─────────────────────────────────────────────────┘
```

### Key Design Principles

1. **Dependency Inversion**: Core domain doesn't depend on adapters
2. **Interface Segregation**: Small, focused interfaces (`ProjectInspector`)
3. **Open/Closed**: Easy to extend (new adapters) without modifying core
4. **Single Responsibility**: Each package has one clear purpose

---

## 🧩 Component Breakdown

### 1. Core Domain Layer (`internal/core/`)

#### BuildTool Type (`buildtool.go`)

**Purpose:** Type-safe representation of supported build tools.

**Implementation:**
```go
type BuildTool int

const (
    Maven BuildTool = iota  // 0
    Gradle                   // 1
)
```

**Why `iota` enumeration?**
- Memory efficient (just an integer)
- Type-safe (can't pass arbitrary strings)
- Exhaustive switch checks (compiler warns on missing cases)

**String Conversion:**
```go
func (bt BuildTool) String() string {
    switch bt {
    case Maven:  return "maven"
    case Gradle: return "gradle"
    default:     return "unknown"
    }
}
```

**Parsing from User Input:**
```go
func ParseBuildTool(s string) (BuildTool, error) {
    switch s {
    case "maven":  return Maven, nil
    case "gradle": return Gradle, nil
    default:       return Maven, fmt.Errorf("unsupported build tool: %s", s)
    }
}
```

**Design Decision:** Return error instead of panic allows graceful handling of invalid user input.

---

#### ProjectInspector Interface (`inspector.go`)

**Purpose:** Contract for inspecting project structure (modules, dependencies, etc.)

**Current Definition:**
```go
type ProjectInspector interface {
    ListModules(projectRoot string) ([]string, error)
}
```

**Design Rationale:**

1. **String path instead of `fs.FS`:**
   - Simpler CLI integration (flags give us strings)
   - Easy to construct absolute paths
   - Future: Can still layer on `fs.FS` for testing

2. **Return `[]string` not `[]Module`:**
   - Current requirement is just module names
   - YAGNI (You Aren't Gonna Need It) - avoid premature abstraction
   - Easy to expand later:
     ```go
     type Module struct {
         Name string
         Path string
         Type ModuleType
     }
     ```

3. **Error return:**
   - Go idiomatic error handling
   - Allows detailed error messages
   - Non-panicking (resilient to bad input)

**Future Extensions:**
```go
type ProjectInspector interface {
    ListModules(projectRoot string) ([]string, error)
    
    // Future methods (will implement in Sprint 2-3):
    ListDependencies(projectRoot string) ([]Dependency, error)
    GetDependencyTree(projectRoot string) (*DependencyTree, error)
    FindUnusedDependencies(projectRoot string) ([]Dependency, error)
}
```

---

#### InspectorFactory (`factory.go`)

**Purpose:** Create appropriate inspector based on build tool type.

**Implementation:**
```go
type InspectorFactory struct{}

func NewInspectorFactory() *InspectorFactory {
    return &InspectorFactory{}
}

func (f *InspectorFactory) ForTool(tool BuildTool) (ProjectInspector, error) {
    switch tool {
    case Maven:
        return maven.NewMavenProjectInspector(), nil
    case Gradle:
        return nil, fmt.Errorf("gradle support not yet implemented")
    default:
        return nil, fmt.Errorf("unsupported build tool: %s", tool.String())
    }
}
```

**Design Patterns:**

1. **Factory Method Pattern:**
   - Centralizes object creation
   - Easy to add new build tools
   - Returns interface, not concrete type (dependency inversion)

2. **Explicit Error Handling:**
   - Unsupported tools return errors, not panics
   - Allows graceful degradation

**Why not use `init()` or global variables?**
```go
// ❌ Bad: Global state
var MavenInspector = maven.NewMavenProjectInspector()

// ✅ Good: Factory method
inspector, err := factory.ForTool(Maven)
```

Benefits:
- Testable (can inject mock factories)
- No hidden initialization order bugs
- Explicit dependencies

---

### 2. Adapter Layer (`internal/adapters/`)

#### Maven Adapter (`maven/inspector.go`)

**Purpose:** Parse Maven POM files to extract project information.

**Full Implementation:**
```go
package maven

import (
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strings"
)

type MavenProjectInspector struct{}

func NewMavenProjectInspector() *MavenProjectInspector {
    return &MavenProjectInspector{}
}

func (i *MavenProjectInspector) ListModules(projectRoot string) ([]string, error) {
    // Step 1: Locate pom.xml
    pomPath := filepath.Join(projectRoot, "pom.xml")
    
    // Step 2: Validate file exists
    if _, err := os.Stat(pomPath); os.IsNotExist(err) {
        return nil, fmt.Errorf("pom.xml not found at: %s", pomPath)
    }
    
    // Step 3: Read file contents
    content, err := os.ReadFile(pomPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read pom.xml: %w", err)
    }
    
    // Step 4: Strip XML comments
    xmlWithoutComments := regexp.MustCompile(`(?s)<!--.*?-->`).ReplaceAllString(string(content), "")
    
    // Step 5: Find <modules> block
    modulesBlockRegex := regexp.MustCompile(`(?is)<modules>(.*?)</modules>`)
    block := modulesBlockRegex.FindStringSubmatch(xmlWithoutComments)
    if len(block) < 2 {
        return []string{}, nil // No modules = single module project
    }
    
    // Step 6: Extract individual <module> tags
    moduleRegex := regexp.MustCompile(`(?is)<module>(.*?)</module>`)
    matches := moduleRegex.FindAllStringSubmatch(block[1], -1)
    
    // Step 7: Clean and collect module names
    var modules []string
    for _, match := range matches {
        if len(match) >= 2 {
            module := strings.TrimSpace(match[1])
            if module != "" {
                modules = append(modules, module)
            }
        }
    }
    
    return modules, nil
}
```

**Deep Dive: Regex Patterns**

##### 1. Comment Stripping
```go
regexp.MustCompile(`(?s)<!--.*?-->`)
```

**Breakdown:**
- `(?s)` - DOTALL mode (`.` matches newlines)
- `<!--` - Literal XML comment start
- `.*?` - Non-greedy match (stops at first `-->`)
- `-->` - Literal XML comment end

**Why remove comments first?**
```xml
<!-- Disabled module
<modules>
  <module>old-module</module>
</modules>
-->
<modules>
  <module>current-module</module>
</modules>
```

Without comment removal, we'd find both blocks!

---

##### 2. Modules Block Extraction
```go
regexp.MustCompile(`(?is)<modules>(.*?)</modules>`)
```

**Breakdown:**
- `(?is)` - Flags: case-insensitive + DOTALL
- `<modules>` - Opening tag (case-insensitive)
- `(.*?)` - **Capture group 1**: Module block content
- `</modules>` - Closing tag

**Why `(?is)` flags?**
- `i` (case-insensitive): Handles `<Modules>`, `<MODULES>`, etc.
- `s` (DOTALL): Allows tags to span multiple lines

**Example Match:**
```xml
<modules>
  <module>auth-service</module>
  <module>payment-service</module>
</modules>
```

`block[0]` = entire match  
`block[1]` = captured content (between tags)

---

##### 3. Individual Module Extraction
```go
regexp.MustCompile(`(?is)<module>(.*?)</module>`)
matches := moduleRegex.FindAllStringSubmatch(block[1], -1)
```

**Breakdown:**
- `FindAllStringSubmatch()` - Returns all matches (not just first)
- `-1` - No limit on number of matches

**Result Structure:**
```go
matches = [
    ["<module>auth-service</module>", "auth-service"],
    ["<module>payment-service</module>", "payment-service"],
]
```

Each match is `[fullMatch, capturedGroup1]`

---

**Why Regex Instead of XML Parser?**

**Pros:**
- ✅ Zero dependencies (no external XML library)
- ✅ Fast for simple operations (no DOM tree building)
- ✅ Handles malformed XML gracefully
- ✅ Small code footprint (~60 lines)

**Cons:**
- ❌ Can't handle complex nested structures
- ❌ Doesn't validate XML schema
- ❌ Difficult to modify XML and preserve formatting

**Decision:** Use regex for **read-only** operations, migrate to XML parser for **write** operations (Sprint 2).

---

**Edge Cases Handled:**

1. **Missing pom.xml:**
   ```go
   return nil, fmt.Errorf("pom.xml not found at: %s", pomPath)
   ```

2. **No modules section:**
   ```go
   if len(block) < 2 {
       return []string{}, nil // Empty slice, not error
   }
   ```

3. **Empty module tags:**
   ```xml
   <module>  </module>  <!-- Whitespace only -->
   ```
   ```go
   module := strings.TrimSpace(match[1])
   if module != "" {
       modules = append(modules, module)
   }
   ```

4. **Whitespace variations:**
   ```xml
   <module>
       auth-service
   </module>
   ```
   `TrimSpace()` handles leading/trailing newlines.

---

#### Gradle Adapter (`gradle/inspector.go`)

**Current Status:** Placeholder implementation

```go
package gradle

import "fmt"

type GradleProjectInspector struct{}

func NewGradleProjectInspector() *GradleProjectInspector {
    return &GradleProjectInspector{}
}

func (i *GradleProjectInspector) ListModules(projectRoot string) ([]string, error) {
    return nil, fmt.Errorf("gradle support not yet implemented")
}
```

**Future Implementation Strategy (Sprint 3):**

Gradle has two main files for multi-module projects:

1. **`settings.gradle` (Groovy):**
   ```groovy
   rootProject.name = 'my-project'
   include 'auth-service'
   include 'payment-service'
   ```

2. **`settings.gradle.kts` (Kotlin):**
   ```kotlin
   rootProject.name = "my-project"
   include("auth-service")
   include("payment-service")
   ```

**Parsing Approach:**

**Option 1: Regex (pragmatic for simple cases)**
```go
// Match Groovy: include 'module-name'
groovyRegex := regexp.MustCompile(`include\s+['"]([^'"]+)['"]`)

// Match Kotlin: include("module-name")
kotlinRegex := regexp.MustCompile(`include\s*\(\s*["']([^"']+)["']\s*\)`)
```

**Option 2: Call Gradle CLI (most reliable)**
```go
cmd := exec.Command("gradle", "-q", "projects")
output, err := cmd.Output()
// Parse output like:
// Root project 'my-project'
// +--- Project ':auth-service'
// +--- Project ':payment-service'
```

**Pros/Cons:**

| Approach | Pros | Cons |
|----------|------|------|
| Regex | Fast, no dependencies | Won't handle complex Gradle scripts |
| Gradle CLI | 100% accurate | Requires Gradle installed, slower |

**Recommendation:** Start with regex for MVP, add CLI fallback for complex cases.

---

### 3. CLI Layer (`cmd/jpm/`)

#### Root Command (`root.go`)

**Purpose:** Entry point for CLI, defines global flags.

```go
package main

import (
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "jpm",
    Short: "JPM - Java Project Manager",
    Long:  `JPM (Java Project Manager) is an open-source CLI tool...`,
}

func Execute() error {
    return rootCmd.Execute()
}

func init() {
    rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Show verbose output")
    rootCmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")
}
```

**Key Concepts:**

1. **PersistentFlags:**
   - Available to all subcommands
   - `jpm module find --verbose` ✅
   - `jpm deps add --verbose` ✅

2. **Short vs Long Description:**
   - Short: Shows in parent command help
   - Long: Shows in `jpm --help`

3. **Execute() function:**
   - Called by `main.go`
   - Returns error for graceful handling

---

#### Module Command (`module.go`)

**Purpose:** Parent command for module operations.

```go
package main

import (
    "github.com/spf13/cobra"
)

var moduleCmd = &cobra.Command{
    Use:   "module",
    Short: "Module operations",
    Long:  `Perform operations on project modules`,
}

func init() {
    rootCmd.AddCommand(moduleCmd)
}
```

**Why separate parent command?**

Enables logical grouping:
```bash
jpm module find
jpm module add     # Future
jpm module remove  # Future
```

vs flat structure:
```bash
jpm find-modules
jpm add-module
jpm remove-module
```

**Benefits:**
- Better discoverability (`jpm module --help`)
- Mirrors Git's UX (`git remote add`, `git remote remove`)

---

#### Find Command (`find.go`)

**Purpose:** List modules in a project.

**Full Implementation:**
```go
package main

import (
    "fmt"
    "path/filepath"
    "github.com/hicham-amazigh/jpm/internal/core"
    "github.com/spf13/cobra"
)

var findCmd = &cobra.Command{
    Use:   "find [path]",
    Short: "List modules in a project",
    Long:  `List all modules defined in a multi-module project`,
    Args:  cobra.MaximumNArgs(1), // Allow 0 or 1 positional args
    RunE: func(cmd *cobra.Command, args []string) error {
        // 1. Parse build tool flag
        buildToolStr, _ := cmd.Flags().GetString("build-tool")
        buildTool, err := core.ParseBuildTool(buildToolStr)
        if err != nil {
            return err // Cobra automatically prints to stderr
        }
        
        // 2. Determine project root
        projectRoot := "."
        if len(args) > 0 {
            projectRoot = args[0]
        }
        projectRoot, err = filepath.Abs(projectRoot)
        if err != nil {
            return fmt.Errorf("failed to get absolute path: %w", err)
        }
        
        // 3. Create inspector via factory
        factory := core.NewInspectorFactory()
        inspector, err := factory.ForTool(buildTool)
        if err != nil {
            return err
        }
        
        // 4. List modules
        modules, err := inspector.ListModules(projectRoot)
        if err != nil {
            return err
        }
        
        // 5. Format output
        fmt.Println("→ found:")
        if len(modules) == 0 {
            fmt.Println("  (none)")
        } else {
            for i, module := range modules {
                fmt.Printf("  %d) %s\n", i+1, module)
            }
        }
        
        return nil
    },
}

func init() {
    moduleCmd.AddCommand(findCmd)
    findCmd.Flags().StringP("build-tool", "b", "maven", "Build tool: maven|gradle")
}
```

**Design Details:**

1. **`RunE` vs `Run`:**
   - `Run`: No error return (must handle errors manually)
   - `RunE`: Returns error (Cobra prints nicely)
   - Choose `RunE` for better error UX

2. **Path Argument Handling:**
   ```go
   Args: cobra.MaximumNArgs(1)
   ```
   - Validates argument count
   - `0` args: Use current directory
   - `1` arg: Use provided path
   - `2+` args: Error automatically

3. **Absolute Path Resolution:**
   ```go
   projectRoot, err = filepath.Abs(projectRoot)
   ```
   - Converts `./my-project` → `/full/path/to/my-project`
   - Normalizes separators (Windows `\` vs Unix `/`)
   - Resolves `.` and `..`

4. **Error Wrapping:**
   ```go
   return fmt.Errorf("failed to get absolute path: %w", err)
   ```
   - `%w` preserves original error for `errors.Is()` / `errors.As()`
   - Provides context for debugging

5. **User-Friendly Output:**
   ```
   → found:
     1) module-one
     2) module-two
   ```
   - `→` visual indicator (progress)
   - Numbered list (easy to reference)
   - Indentation for hierarchy

---

#### Main Entry Point (`main.go`)

```go
package main

import (
    "fmt"
    "os"
)

func main() {
    if err := Execute(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

**Why separate Execute() from main()?**

1. **Testability:**
   ```go
   func TestCLI(t *testing.T) {
       // Can test Execute() without running main()
       err := Execute()
       assert.NoError(t, err)
   }
   ```

2. **Error Handling:**
   - Cobra handles errors internally
   - Main just exits with proper code

3. **Flexibility:**
   - Can call Execute() from tests, benchmarks, or other packages

---

## 🔍 Testing Strategy (Sprint 2)

### Unit Tests

**Example: BuildTool Parsing**
```go
func TestParseBuildTool(t *testing.T) {
    tests := []struct {
        input    string
        expected core.BuildTool
        wantErr  bool
    }{
        {"maven", core.Maven, false},
        {"gradle", core.Gradle, false},
        {"Maven", core.Maven, true},  // Case-sensitive!
        {"invalid", 0, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            got, err := core.ParseBuildTool(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseBuildTool(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
            }
            if got != tt.expected {
                t.Errorf("ParseBuildTool(%q) = %v, want %v", tt.input, got, tt.expected)
            }
        })
    }
}
```

---

### Integration Tests

**Example: Maven Inspector**
```go
func TestMavenInspector_ListModules(t *testing.T) {
    // Setup: Create temp directory with test pom.xml
    tmpDir := t.TempDir()
    pomContent := `
    <project>
      <modules>
        <module>service-a</module>
        <module>service-b</module>
      </modules>
    </project>
    `
    os.WriteFile(filepath.Join(tmpDir, "pom.xml"), []byte(pomContent), 0644)
    
    // Test
    inspector := maven.NewMavenProjectInspector()
    modules, err := inspector.ListModules(tmpDir)
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, []string{"service-a", "service-b"}, modules)
}
```

---

### CLI Tests

**Example: Module Find Command**
```go
func TestModuleFindCommand(t *testing.T) {
    // Capture stdout
    var buf bytes.Buffer
    rootCmd.SetOut(&buf)
    rootCmd.SetErr(&buf)
    
    // Execute command
    rootCmd.SetArgs([]string{"module", "find", "/path/to/test-project"})
    err := rootCmd.Execute()
    
    // Assert
    assert.NoError(t, err)
    output := buf.String()
    assert.Contains(t, output, "→ found:")
}
```

---

## 🏗 Build Process

### Development Build
```bash
go build -o jpm ./cmd/jpm
```

**Output:** `jpm` binary (~8-10MB)

---

### Production Build (Optimized)
```bash
go build -ldflags="-s -w" -o jpm ./cmd/jpm
```

**Flags:**
- `-s`: Strip symbol table
- `-w`: Strip DWARF debugging info

**Result:** ~6MB binary (25% size reduction)

---

### Cross-Compilation
```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o jpm-linux ./cmd/jpm

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o jpm-mac-intel ./cmd/jpm

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o jpm-mac-arm ./cmd/jpm

# Windows
GOOS=windows GOARCH=amd64 go build -o jpm.exe ./cmd/jpm
```

---

### Release Build (All Platforms)
```bash
# Using GoReleaser (future)
goreleaser release --snapshot --clean
```

**Produces:**
- Binaries for all platforms
- Checksums (SHA256)
- Archives (`.tar.gz`, `.zip`)
- Debian/RPM packages

---

## 📦 Dependency Management

### Current Dependencies

**Direct:**
- `github.com/spf13/cobra` v1.10.1 - CLI framework
  - Transitive: `github.com/spf13/pflag` (POSIX flags)
  - Transitive: `github.com/inconshreveable/mousetrap` (Windows support)

**Standard Library:**
- `fmt` - Formatting and printing
- `os` - File system operations
- `path/filepath` - Path manipulation
- `regexp` - Regular expressions
- `strings` - String utilities

### Why Minimal Dependencies?

1. **Security:** Fewer dependencies = smaller attack surface
2. **Maintenance:** Less dependency upgrade churn
3. **Binary Size:** Each dependency adds weight
4. **Build Time:** Faster compilation
5. **Reliability:** Standard library is battle-tested

---

## 🔐 Security Considerations

### Input Validation

**File Path Sanitization:**
```go
// Prevent directory traversal
projectRoot, err = filepath.Abs(projectRoot)
projectRoot = filepath.Clean(projectRoot)
```

**Flag Validation:**
```go
// Validate build tool before using
buildTool, err := core.ParseBuildTool(buildToolStr)
if err != nil {
    return err // Don't proceed with invalid input
}
```

---

### Error Information Disclosure

**Bad:**
```go
return fmt.Errorf("internal error: %v", internalDetails)
```

**Good:**
```go
return fmt.Errorf("failed to read project file")
// Log internal details separately if needed
```

---

### Future: Dependency Security

**Sprint 3+ Feature:**
```go
// Check dependencies against vulnerability databases
func (i *MavenProjectInspector) AuditDependencies(projectRoot string) ([]Vulnerability, error) {
    // Query OSS Index, Snyk, GitHub Advisory Database
}
```

---

## 🚀 Performance Optimizations

### Current Optimizations

1. **Lazy Initialization:**
   ```go
   // Don't create inspector until needed
   inspector, err := factory.ForTool(buildTool)
   ```

2. **Single File Read:**
   ```go
   // Read entire file once, not line-by-line
   content, err := os.ReadFile(pomPath)
   ```

3. **Compiled Regex:**
   ```go
   // Future: Compile regexes once at package level
   var moduleRegex = regexp.MustCompile(`<module>(.*?)</module>`)
   ```

### Future Optimizations (Sprint 2+)

1. **Caching:**
   ```go
   // Cache Maven Central queries
   type VersionCache struct {
       sync.RWMutex
       data map[string][]Version
   }
   ```

2. **Parallel Operations:**
   ```go
   // List modules in multiple subprojects concurrently
   var wg sync.WaitGroup
   for _, subproject := range subprojects {
       wg.Add(1)
       go func(p string) {
           defer wg.Done()
           modules, _ := inspector.ListModules(p)
       }(subproject)
   }
   wg.Wait()
   ```

3. **Streaming XML Parser:**
   ```go
   // For huge POM files, use streaming instead of regex
   decoder := xml.NewDecoder(file)
   for {
       token, _ := decoder.Token()
       // Process incrementally
   }
   ```

---

## 📊 Code Metrics

### Current Codebase (Sprint 1)

| Metric | Value |
|--------|-------|
| Total Lines of Code | ~350 |
| Packages | 4 (core, maven, gradle, main) |
| Files | 7 |
| External Dependencies | 1 (Cobra) |
| Test Coverage | 0% (Sprint 2 priority) |
| Cyclomatic Complexity | Low (< 10 per function) |

### Quality Goals

- **Test Coverage:** 80%+ by Sprint 3
- **Cyclomatic Complexity:** < 15 per function
- **Function Length:** < 50 lines (except generated code)
- **Package Cohesion:** High (single responsibility)

---

## 🔄 Future Architecture Evolution

### Planned Enhancements

1. **Plugin System:**
   ```go
   // Allow external adapters via plugins
   type PluginAdapter interface {
       Name() string
       Inspector() ProjectInspector
   }
   ```

2. **Configuration File:**
   ```yaml
   # ~/.jpm/config.yaml
   default_build_tool: maven
   cache_ttl: 24h
   aliases:
     lg: deps show --scope compile
   ```

3. **API Server Mode:**
   ```go
   // Run JPM as HTTP service for IDE integration
   jpm serve --port 8080
   ```

4. **Language Server Protocol (LSP):**
   ```go
   // Provide editor integration via LSP
   // Autocomplete dependencies, version suggestions, etc.
   ```

---

This deep dive should provide a comprehensive understanding of JPM's implementation. All design decisions are intentional and optimized for the current requirements while allowing future expansion.