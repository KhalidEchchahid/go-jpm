package java

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ManualPathEntry handles user-provided Java installation paths.
type ManualPathEntry struct {
	maxAttempts int
	verbose     bool
}

// NewManualPathEntry creates a new manual path entry handler.
func NewManualPathEntry(verbose bool) *ManualPathEntry {
	return &ManualPathEntry{
		maxAttempts: 3,
		verbose:     verbose,
	}
}

// PromptAndValidate asks the user for a Java path and validates it.
// Returns (success bool, javaHome string, error string).
func (m *ManualPathEntry) PromptAndValidate() (bool, string, string) {
	reader := bufio.NewReader(os.Stdin)

	for attempt := 1; attempt <= m.maxAttempts; attempt++ {
		fmt.Printf("\n• Enter Java installation path (attempt %d/%d):\n", attempt, m.maxAttempts)
		fmt.Print("> ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			fmt.Println("! Path cannot be empty")
			continue
		}

		// Expand ~ to home directory
		if strings.HasPrefix(input, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				fmt.Println("! Failed to expand home directory")
				continue
			}
			input = filepath.Join(home, input[1:])
		}

		// Validate: check if java executable exists
		result := m.validatePath(input)
		if !result.Valid {
			fmt.Printf("! %s\n", result.Error)
			if attempt < m.maxAttempts {
				fmt.Println("Please try again.")
			}
			continue
		}

		// Success
		if m.verbose {
			fmt.Printf("✔ Valid Java installation at: %s\n", input)
			fmt.Printf("  Version: %s (major: %d)\n", result.FullVersion, result.MajorVersion)
		}

		return true, input, ""
	}

	return false, "", fmt.Sprintf("Failed to find valid Java path after %d attempts", m.maxAttempts)
}

// ValidationResult holds the outcome of path validation.
type ValidationResult struct {
	Valid        bool
	JavaHome     string
	FullVersion  string
	MajorVersion int
	Error        string
}

// validatePath checks if a path contains a valid Java installation.
func (m *ManualPathEntry) validatePath(path string) *ValidationResult {
	result := &ValidationResult{JavaHome: path}

	// Check if java executable exists
	javaBin := filepath.Join(path, "bin", "java")
	_, err := os.Stat(javaBin)
	if err != nil {
		result.Error = fmt.Sprintf("java executable not found at %s", javaBin)
		return result
	}

	// Run java -version to validate
	detector := &Detector{verbose: m.verbose}
	tempResult := &DetectionResult{
		JavaPath: javaBin,
	}
	detector.extractVersion(tempResult)

	if tempResult.Error != "" {
		result.Error = tempResult.Error
		return result
	}

	if tempResult.MajorVersion == 0 {
		result.Error = "could not parse Java version"
		return result
	}

	if !IsValidVersion(tempResult.MajorVersion) {
		result.Error = fmt.Sprintf("Java version %d is too old (requires 17 or later)", tempResult.MajorVersion)
		return result
	}

	result.Valid = true
	result.FullVersion = tempResult.FullVersion
	result.MajorVersion = tempResult.MajorVersion
	return result
}
