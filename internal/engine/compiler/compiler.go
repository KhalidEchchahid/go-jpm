package compiler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CompileResult represents the output of compilation.
type CompileResult struct {
	Sources   int           `json:"sources"`
	Classes   int           `json:"classes"`
	Duration  time.Duration `json:"duration"`
	Warnings  []string      `json:"warnings"`
	Errors    []string      `json:"errors"`
}

// Compiler compiles Java sources using javac.
type Compiler struct {
	verbose bool
	version string
}

// NewCompiler creates a new compiler instance.
func NewCompiler(verbose bool, version string) *Compiler {
	return &Compiler{
		verbose: verbose,
		version: version,
	}
}

// Compile compiles Java sources from srcDir to outDir with the given classpath.
func (c *Compiler) Compile(
	ctx context.Context,
	srcDir, outDir string,
	classpath []string,
	javaVersion string,
) (*CompileResult, error) {
	start := time.Now()
	result := &CompileResult{
		Warnings: []string{},
		Errors:   []string{},
	}

	// Validate Java version
	if err := validateJavaVersion(javaVersion); err != nil {
		return nil, err
	}

	// Discover source files
	sources, err := discoverSources(srcDir)
	if err != nil {
		return nil, fmt.Errorf("discovering sources: %w", err)
	}
	result.Sources = len(sources)

	// If no sources, return early
	if len(sources) == 0 {
		result.Duration = time.Since(start)
		if c.verbose {
			fmt.Printf("No Java sources found in %s; skipping compilation\n", srcDir)
		}
		return result, nil
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	// Build classpath string
	classpathStr := buildClasspath(classpath, outDir)

	// Build javac command
	args := []string{
		"-encoding", "UTF-8",
		"-g",
		"-Xlint:deprecation",
		"-Xlint:unchecked",
		"-source", javaVersion,
		"-target", javaVersion,
		"-d", outDir,
	}

	if classpathStr != "" {
		args = append(args, "-cp", classpathStr)
	}

	// Add source files (sorted for determinism)
	args = append(args, sources...)

	// Execute javac
	cmd := exec.CommandContext(ctx, "javac", args...)
	if c.verbose {
		fmt.Printf("Running: javac %s\n", strings.Join(args, " "))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Errors = parseCompilerOutput(string(output))
		result.Duration = time.Since(start)
		return result, fmt.Errorf("javac failed: %w\nOutput:\n%s", err, string(output))
	}

	// Parse warnings from output
	result.Warnings = parseCompilerOutput(string(output))

	// Count compiled classes
	classCount, err := countClasses(outDir)
	if err != nil {
		return nil, fmt.Errorf("counting classes: %w", err)
	}
	result.Classes = classCount

	result.Duration = time.Since(start)
	return result, nil
}

// discoverSources finds all .java files under srcDir, sorted lexicographically.
func discoverSources(srcDir string) ([]string, error) {
	var sources []string

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".java") {
			sources = append(sources, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Strings(sources)
	return sources, nil
}

// buildClasspath constructs a classpath string from artifacts and the output directory.
func buildClasspath(classpath []string, outDir string) string {
	if len(classpath) == 0 && outDir == "" {
		return ""
	}

	var parts []string
	parts = append(parts, outDir) // Include work classes for incremental compile
	parts = append(parts, classpath...)

	sep := string(os.PathListSeparator)
	return strings.Join(parts, sep)
}

// parseCompilerOutput extracts warnings and errors from javac output.
func parseCompilerOutput(output string) []string {
	if output == "" {
		return []string{}
	}

	var messages []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Include lines that contain "warning:" or "error:"
		if strings.Contains(line, "warning:") || strings.Contains(line, "error:") {
			messages = append(messages, line)
		}
	}
	return messages
}

// countClasses counts the number of .class files in outDir.
func countClasses(outDir string) (int, error) {
	var count int

	err := filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".class") {
			count++
		}
		return nil
	})

	return count, err
}

// validateJavaVersion ensures javaVersion is valid.
func validateJavaVersion(version string) error {
	valid := map[string]bool{"17": true, "21": true, "23": true}
	if !valid[version] {
		return fmt.Errorf("unsupported Java version: %s (valid: 17, 21, 23)", version)
	}
	return nil
}

// String returns a JSON representation of CompileResult.
func (cr *CompileResult) String() string {
	data, _ := json.MarshalIndent(cr, "", "  ")
	return string(data)
}
