package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/KhalidEchchahid/go-jpm/internal/engine/native"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// test.go implements "jpm test" which compiles and runs JUnit tests.

var testCmd = &cobra.Command{
	Use:   "test [path]",
	Short: "Run project tests",
	Long: `Run JUnit tests for the project.

This command compiles test sources and runs them using JUnit.
It automatically detects test classes (files ending in Test.java or Tests.java)
and runs them with the appropriate test runner.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)

	testCmd.Flags().String("class", "", "Run specific test class (e.g., com.example.MyTest)")
	testCmd.Flags().Bool("verbose", false, "Show verbose test output")
}

func runTest(cmd *cobra.Command, args []string) error {
	projectPath := "."
	if len(args) > 0 {
		projectPath = args[0]
	}

	projectPath = strings.TrimSpace(projectPath)
	projectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("failed to resolve project path: %w", err)
	}

	// Load manifest
	manifestPath := filepath.Join(projectPath, "jpm.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("no jpm.yaml found in %s (run 'jpm init')", projectPath)
	}

	var manifest core.Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse jpm.yaml: %w", err)
	}

	specificClass, _ := cmd.Flags().GetString("class")
	verbose, _ := cmd.Flags().GetBool("verbose")

	fmt.Println(headerStyle("→ running tests:"))

	// Check for test sources
	testSrcDir := filepath.Join(projectPath, "src", "test", "java")
	testSrcFlat := filepath.Join(projectPath, "test") // Alternative flat structure

	var testDir string
	if info, err := os.Stat(testSrcDir); err == nil && info.IsDir() {
		testDir = testSrcDir
	} else if info, err := os.Stat(testSrcFlat); err == nil && info.IsDir() {
		testDir = testSrcFlat
	} else {
		return fmt.Errorf("no test sources found (expected src/test/java/ or test/)")
	}

	// Find test files
	var testFiles []string
	err = filepath.Walk(testDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".java") {
			name := info.Name()
			if strings.HasSuffix(name, "Test.java") || strings.HasSuffix(name, "Tests.java") {
				testFiles = append(testFiles, path)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan test directory: %w", err)
	}

	if len(testFiles) == 0 {
		fmt.Println("  no test files found")
		return nil
	}

	fmt.Printf("  found %d test file(s)\n", len(testFiles))

	// Build classpath (include test dependencies)
	var classpath []string

	// Add main compiled classes
	mainOutDir := filepath.Join(projectPath, ".jpm", "out", "classes")
	if _, err := os.Stat(mainOutDir); err == nil {
		classpath = append(classpath, mainOutDir)
	}

	// Resolve all dependencies (including test scope)
	allDeps := manifest.Dependencies

	if len(allDeps) > 0 {
		cacheDir := filepath.Join(projectPath, ".jpm", "cache")
		cache := native.NewCache(cacheDir)
		downloader := native.NewDownloader("", cache)
		resolver := native.NewResolver(downloader, cache)

		coreDeps := make([]core.Dependency, len(allDeps))
		for i, d := range allDeps {
			coreDeps[i] = core.Dependency{
				GroupID:    d.GroupID,
				ArtifactID: d.ArtifactID,
				Version:    d.Version,
				Scope:      d.Scope,
			}
		}

		resolved, err := resolver.Resolve(coreDeps)
		if err != nil {
			return fmt.Errorf("failed to resolve dependencies: %w", err)
		}

		for _, dep := range resolved {
			classpath = append(classpath, dep.LocalPath)
		}
	}

	// Check if JUnit is in dependencies
	hasJunit := false
	for _, dep := range manifest.Dependencies {
		if strings.Contains(dep.ArtifactID, "junit") {
			hasJunit = true
			break
		}
	}

	if !hasJunit {
		fmt.Println(warningTextStyle("  warning: JUnit not found in dependencies"))
		fmt.Println("  hint: run 'jpm deps add junit:junit@4.13.2 --scope test'")
	}

	// Compile test sources
	testOutDir := filepath.Join(projectPath, ".jpm", "out", "test-classes")
	if err := os.MkdirAll(testOutDir, 0755); err != nil {
		return fmt.Errorf("failed to create test output directory: %w", err)
	}

	// Add test output to classpath for running
	classpath = append(classpath, testOutDir)

	fmt.Println("  compiling tests...")

	// Build classpath string
	cpString := strings.Join(classpath, string(os.PathListSeparator))

	// Compile test files
	compileArgs := []string{
		"-d", testOutDir,
		"-cp", cpString,
		"-source", manifest.Java.Version,
		"-target", manifest.Java.Version,
	}
	compileArgs = append(compileArgs, testFiles...)

	compileCmd := exec.Command("javac", compileArgs...)
	compileCmd.Dir = projectPath
	if verbose {
		compileCmd.Stdout = os.Stdout
		compileCmd.Stderr = os.Stderr
	}

	if err := compileCmd.Run(); err != nil {
		return fmt.Errorf("test compilation failed: %w", err)
	}

	fmt.Println("  running tests...")

	// Determine test classes to run
	var testClasses []string
	if specificClass != "" {
		testClasses = []string{specificClass}
	} else {
		// Convert test files to class names by reading the package declaration
		for _, tf := range testFiles {
			className, err := extractFullClassName(tf)
			if err != nil {
				// Fall back to relative path conversion
				relPath, _ := filepath.Rel(testDir, tf)
				className = strings.TrimSuffix(relPath, ".java")
				className = strings.ReplaceAll(className, string(os.PathSeparator), ".")
			}
			testClasses = append(testClasses, className)
		}
	}

	// Run tests with JUnit
	// Try JUnit 4 runner first, fall back to simple java execution
	runnerClass := "org.junit.runner.JUnitCore"

	runArgs := []string{
		"-cp", cpString,
		runnerClass,
	}
	runArgs = append(runArgs, testClasses...)

	runCmd := exec.Command("java", runArgs...)
	runCmd.Dir = projectPath
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr

	if err := runCmd.Run(); err != nil {
		// Check if it's just test failures vs actual errors
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Printf("\n  tests completed with failures (exit code %d)\n", exitErr.ExitCode())
			return nil // Don't return error for test failures
		}
		return fmt.Errorf("test execution failed: %w", err)
	}

	fmt.Println("\n  " + headerStyle("✓ all tests passed"))

	return nil
}

// extractFullClassName reads a Java file and extracts the fully qualified class name
func extractFullClassName(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	var packageName string
	var className string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for package declaration
		if strings.HasPrefix(line, "package ") {
			packageName = strings.TrimPrefix(line, "package ")
			packageName = strings.TrimSuffix(packageName, ";")
			packageName = strings.TrimSpace(packageName)
		}

		// Look for class declaration
		if strings.Contains(line, "class ") && !strings.HasPrefix(line, "//") {
			// Extract class name from "public class Foo" or "class Foo"
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "class" && i+1 < len(parts) {
					className = parts[i+1]
					// Remove any trailing { or extends/implements
					className = strings.Split(className, "{")[0]
					className = strings.Split(className, " ")[0]
					break
				}
			}
			if className != "" {
				break
			}
		}
	}

	if className == "" {
		// Fall back to filename
		base := filepath.Base(filePath)
		className = strings.TrimSuffix(base, ".java")
	}

	if packageName != "" {
		return packageName + "." + className, nil
	}
	return className, nil
}
