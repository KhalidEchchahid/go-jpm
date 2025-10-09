package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These integration tests exercise the Cobra command wiring against the
// java-legacy fixtures to ensure flag parsing and output formatting behave the
// same way the manual CLI does.
func TestCLI_ModuleFindIntegration(t *testing.T) {
	output, err := executeCommand(t, "module", "find", filepath.Join("..", "..", "java-legacy", "JPM"))
	if err != nil {
		t.Fatalf("command returned error: %v\noutput: %s", err, output)
	}

	expected := []string{"jpm-core", "jpm-adapter-maven", "jpm-adapter-gradle", "jpm-init", "jpm-cli"}
	for _, module := range expected {
		if !strings.Contains(output, module) {
			t.Fatalf("expected output to contain %q\noutput: %s", module, output)
		}
	}
}

func TestCLI_DepsShowIntegration(t *testing.T) {
	cliModule := filepath.Join("..", "..", "java-legacy", "JPM", "jpm-cli")
	output, err := executeCommand(t, "deps", "show", cliModule)
	if err != nil {
		t.Fatalf("command returned error: %v\noutput: %s", err, output)
	}

	checks := []string{
		"com.jpm:jpm-core:0.0.1-SNAPSHOT",
		"info.picocli:picocli:4.7.6",
	}
	for _, token := range checks {
		if !strings.Contains(output, token) {
			t.Fatalf("expected dependency %q in output\noutput: %s", token, output)
		}
	}
}

func TestCLI_DepsShowAggregatorHasNoDeps(t *testing.T) {
	output, err := executeCommand(t, "deps", "show", filepath.Join("..", "..", "java-legacy", "JPM"))
	if err != nil {
		t.Fatalf("command returned error: %v\noutput: %s", err, output)
	}

	if !strings.Contains(output, "(none)") {
		t.Fatalf("expected output to mention '(none)' for empty dependencies\noutput: %s", output)
	}
}

func TestCLI_GradleNotImplemented(t *testing.T) {
	_, err := executeCommand(t, "module", "find", "--build-tool", "gradle", filepath.Join("..", "..", "java-legacy", "JPM"))
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
