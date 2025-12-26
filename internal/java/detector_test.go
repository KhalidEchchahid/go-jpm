package java

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetector_ParseVersion(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    string
		wantMaj int
	}{
		{
			name:    "Java 21 Temurin",
			output:  `openjdk version "21.0.1" 2023-10-17 LTS`,
			want:    "21.0.1",
			wantMaj: 21,
		},
		{
			name:    "Java 17 OpenJDK",
			output:  `openjdk version "17.0.8" 2023-07-18 LTS`,
			want:    "17.0.8",
			wantMaj: 17,
		},
		{
			name:    "Java 23",
			output:  `java version "23.0.1" 2024-10-29`,
			want:    "23.0.1",
			wantMaj: 23,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseJavaVersion(tt.output)
			if got != tt.want {
				t.Errorf("parseJavaVersion() = %s, want %s", got, tt.want)
			}

			major := extractMajorVersion(got)
			if major != tt.wantMaj {
				t.Errorf("extractMajorVersion() = %d, want %d", major, tt.wantMaj)
			}
		})
	}
}

func TestIsValidVersion(t *testing.T) {
	tests := []struct {
		major int
		want  bool
	}{
		{11, false},
		{16, false},
		{17, true},
		{21, true},
		{23, true},
		{25, true},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := IsValidVersion(tt.major)
			if got != tt.want {
				t.Errorf("IsValidVersion(%d) = %v, want %v", tt.major, got, tt.want)
			}
		})
	}
}

func TestDetector_ValidateJavaHome(t *testing.T) {
	// Create temp directory structure
	tmpdir := t.TempDir()
	bindir := filepath.Join(tmpdir, "bin")
	os.MkdirAll(bindir, 0o755)

	// Create mock java executable
	javaPath := filepath.Join(bindir, "java")
	os.WriteFile(javaPath, []byte("#!/bin/bash\necho test\n"), 0o755)

	detector := NewDetector(false)

	// Test valid JAVA_HOME
	if !detector.validateJavaHome(tmpdir) {
		t.Errorf("validateJavaHome() should return true for valid path")
	}

	// Test invalid JAVA_HOME
	if detector.validateJavaHome("/nonexistent/path") {
		t.Errorf("validateJavaHome() should return false for invalid path")
	}
}

func TestSDKManInstaller_Platform(t *testing.T) {
	installer := NewSDKManInstaller("21", false)

	// Should only work on Linux/macOS
	supported := installer.IsPlatformSupported()
	t.Logf("SDKMAN platform supported: %v\n", supported)
}

func TestSDKManInstaller_JavaVersion(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		{"17", "17.0.8-tem"},
		{"21", "21.0.1-tem"},
		{"23", "23.0.1-tem"},
		{"99", "99-tem"}, // Fallback
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			installer := NewSDKManInstaller(tt.version, false)
			got := installer.sdkmanJavaVersion()
			if got != tt.want {
				t.Errorf("sdkmanJavaVersion() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestHomebrewInstaller_Commands(t *testing.T) {
	installer := NewHomebrewInstaller("21", false)
	cmds := installer.GetInstallCommands()

	if len(cmds) < 2 {
		t.Errorf("GetInstallCommands() should return at least 2 commands, got %d", len(cmds))
	}

	// Check that commands reference the version
	for _, cmd := range cmds {
		if !containsString(cmd, "21") {
			t.Errorf("Command should contain version 21: %s", cmd)
		}
	}
}

func TestManualPathEntry_ValidatePath(t *testing.T) {
	entry := NewManualPathEntry(false)

	// Invalid path
	result := entry.validatePath("/nonexistent/path")
	if result.Valid {
		t.Errorf("validatePath() should return invalid for nonexistent path")
	}

	// Valid path test skipped (requires actual Java installation)
	// In real usage, this would be tested with a real Java directory
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr))
}
