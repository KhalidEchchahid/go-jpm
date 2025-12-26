package java

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Detector finds and validates Java installations.
type Detector struct {
	verbose bool
}

// NewDetector creates a new Java detector.
func NewDetector(verbose bool) *Detector {
	return &Detector{verbose: verbose}
}

// DetectionResult holds the outcome of Java detection.
type DetectionResult struct {
	Found        bool
	JavaHome     string // Path to JAVA_HOME
	JavaPath     string // Path to java executable
	MajorVersion int    // 17, 21, 23, etc.
	FullVersion  string // e.g., "21.0.1"
	Error        string // If detection failed
}

// Detect searches for Java in PATH, JAVA_HOME, and common locations.
func (d *Detector) Detect() *DetectionResult {
	result := &DetectionResult{}

	// Step 1: Try JAVA_HOME env var
	if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
		if d.validateJavaHome(javaHome) {
			result.JavaHome = javaHome
			result.JavaPath = filepath.Join(javaHome, "bin", "java")
			result.Found = true
			d.extractVersion(result)
			return result
		}
	}

	// Step 2: Try java on PATH
	javaPath, err := exec.LookPath("java")
	if err == nil {
		result.JavaPath = javaPath
		result.Found = true
		d.extractVersion(result)
		return result
	}

	// Step 3: Try common locations
	commonPaths := d.commonJavaLocations()
	for _, path := range commonPaths {
		if d.validateJavaHome(path) {
			result.JavaHome = path
			result.JavaPath = filepath.Join(path, "bin", "java")
			result.Found = true
			d.extractVersion(result)
			return result
		}
	}

	result.Found = false
	result.Error = "java not found in PATH, JAVA_HOME, or common locations"
	return result
}

// validateJavaHome checks if a directory looks like a valid JAVA_HOME.
func (d *Detector) validateJavaHome(path string) bool {
	javaBin := filepath.Join(path, "bin", "java")
	_, err := os.Stat(javaBin)
	return err == nil
}

// commonJavaLocations returns platform-specific common Java install paths.
func (d *Detector) commonJavaLocations() []string {
	locations := []string{
		"/usr/lib/jvm",
		"/usr/local/opt/java",
		"/usr/local/opt/openjdk",
		"/usr/local/opt/temurin",
	}

	// On macOS, also check Homebrew
	if isHomebrewInstalled() {
		brewPrefixes := []string{
			"/opt/homebrew/opt/java",
			"/opt/homebrew/opt/openjdk",
			"/opt/homebrew/opt/temurin",
			"/usr/local/opt/java",
		}
		locations = append(locations, brewPrefixes...)
	}

	// On Linux, expand /usr/lib/jvm entries
	if entries, err := os.ReadDir("/usr/lib/jvm"); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				locations = append(locations, filepath.Join("/usr/lib/jvm", entry.Name()))
			}
		}
	}

	return locations
}

// extractVersion runs `java -version` and parses the major version.
func (d *Detector) extractVersion(result *DetectionResult) {
	cmd := exec.Command(result.JavaPath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Error = fmt.Sprintf("failed to run java -version: %v", err)
		result.Found = false
		return
	}

	// Parse version from output
	// Examples:
	// "openjdk version "21.0.1" 2023-10-17 LTS"
	// "java version "17.0.8" 2023-07-18 LTS"
	versionStr := parseJavaVersion(string(output))
	result.FullVersion = versionStr
	result.MajorVersion = extractMajorVersion(versionStr)

	if d.verbose {
		fmt.Printf("Detected Java: %s (major: %d)\n", versionStr, result.MajorVersion)
	}
}

// parseJavaVersion extracts the version string from java -version output.
func parseJavaVersion(output string) string {
	// Look for quoted version like "21.0.1" or "17.0.8"
	re := regexp.MustCompile(`"([0-9]+\.[0-9]+(\.[0-9]+)?)"`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		return matches[1]
	}
	// Fallback: return first line
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

// extractMajorVersion extracts the major version (e.g., 21 from "21.0.1").
func extractMajorVersion(versionStr string) int {
	parts := strings.Split(versionStr, ".")
	if len(parts) == 0 {
		return 0
	}
	var major int
	fmt.Sscanf(parts[0], "%d", &major)
	return major
}

// isValidVersion checks if the detected version is acceptable.
func (d *Detector) isValidVersion(major int) bool {
	// Acceptable: 17, 21, 23 (and future versions)
	// Reject: < 17
	return major >= 17
}

// IsValidVersion is a public helper to validate a major version.
func IsValidVersion(major int) bool {
	return major >= 17
}

// isHomebrewInstalled checks if Homebrew is available.
func isHomebrewInstalled() bool {
	_, err := exec.LookPath("brew")
	return err == nil
}
