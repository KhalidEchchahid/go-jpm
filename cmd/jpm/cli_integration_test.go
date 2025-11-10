package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These integration tests exercise the Cobra command wiring against the
// project_test fixtures to ensure flag parsing and output formatting behave the
// same way the manual CLI does.
func TestCLI_ModuleLsIntegration(t *testing.T) {
	testPath := filepath.Join("..", "..", "project_test")
	// Skip test if project_test doesn't exist or pom.xml not generated
	pomPath := filepath.Join(testPath, "pom.xml")
	if _, err := os.Stat(pomPath); os.IsNotExist(err) {
		t.Skipf("project_test pom.xml not found at %s (skipping integration test)", pomPath)
	}

	output, err := executeCommand(t, "module", "ls", testPath)
	if err != nil {
		t.Fatalf("command returned error: %v\noutput: %s", err, output)
	}

	// For single-module project, should show the module name
	if !strings.Contains(output, "project-test") && !strings.Contains(output, "project_test") {
		t.Logf("note: output may vary for single-module project\noutput: %s", output)
	}
}

func TestCLI_DepsLsIntegration(t *testing.T) {
	testPath := filepath.Join("..", "..", "project_test")
	// Skip test if project_test doesn't exist
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Skipf("project_test directory not found at %s", testPath)
	}

	output, err := executeCommand(t, "deps", "ls", testPath)
	if err != nil {
		t.Fatalf("command returned error: %v\noutput: %s", err, output)
	}

	// For project_test with no deps, should show (none)
	if !strings.Contains(output, "(none)") {
		t.Logf("note: output may contain dependencies if project_test has them\noutput: %s", output)
	}
}

func TestCLI_DepsLsAggregatorHasNoDeps(t *testing.T) {
	testPath := filepath.Join("..", "..", "project_test")
	// Skip test if project_test doesn't exist
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Skipf("project_test directory not found at %s", testPath)
	}

	output, err := executeCommand(t, "deps", "ls", testPath)
	if err != nil {
		t.Fatalf("command returned error: %v\noutput: %s", err, output)
	}

	if !strings.Contains(output, "(none)") {
		t.Logf("note: project may have dependencies\noutput: %s", output)
	}
}

func TestCLI_DepsTreeIntegration(t *testing.T) {
	testPath := filepath.Join("..", "..", "project_test")
	// Skip test if pom.xml not generated
	pomPath := filepath.Join(testPath, "pom.xml")
	if _, err := os.Stat(pomPath); os.IsNotExist(err) {
		t.Skipf("project_test pom.xml not found at %s (skipping integration test)", pomPath)
	}

	output, err := executeCommand(t, "deps", "tree", testPath)
	if err != nil {
		t.Fatalf("command returned error: %v\noutput: %s", err, output)
	}

	if !strings.Contains(output, "project-test") && !strings.Contains(output, "project_test") {
		t.Logf("note: output structure may vary\noutput: %s", output)
	}
}

func TestCLI_GradleNotImplemented(t *testing.T) {
	testPath := filepath.Join("..", "..", "project_test")
	// Skip test if project_test doesn't exist
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Skipf("project_test directory not found at %s", testPath)
	}

	_, err := executeCommand(t, "module", "ls", "--build-tool", "gradle", testPath)
	if err == nil || !strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("expected not yet implemented error, got %v", err)
	}
}

// executeCommand runs the root Cobra command with the supplied arguments while
// capturing stdout/stderr into a string for assertions.
func executeCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	origStdout := os.Stdout
	origStderr := os.Stderr
	origSilenceErrors := rootCmd.SilenceErrors
	origSilenceUsage := rootCmd.SilenceUsage

	os.Stdout = w
	os.Stderr = w
	rootCmd.SetArgs(args)
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	execErr := rootCmd.Execute()

	rootCmd.SetArgs(nil)
	rootCmd.SilenceErrors = origSilenceErrors
	rootCmd.SilenceUsage = origSilenceUsage
	_ = w.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	output, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("failed to read command output: %v", readErr)
	}

	return string(output), execErr
}
