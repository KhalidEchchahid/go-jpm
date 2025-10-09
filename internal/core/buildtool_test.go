package core

import "testing"

// TestParseBuildTool covers the string-to-enum parsing helper used throughout
// the CLI for validating the --build-tool flag.
func TestParseBuildTool(t *testing.T) {
	tool, err := ParseBuildTool("maven")
	if err != nil {
		t.Fatalf("expected no error parsing maven, got %v", err)
	}
	if tool != Maven {
		t.Fatalf("expected Maven, got %v", tool)
	}

	tool, err = ParseBuildTool("gradle")
	if err != nil {
		t.Fatalf("expected no error parsing gradle, got %v", err)
	}
	if tool != Gradle {
		t.Fatalf("expected Gradle, got %v", tool)
	}

	if _, err = ParseBuildTool("ant"); err == nil {
		t.Fatalf("expected error for unsupported tool")
	}
}

// TestBuildToolString ensures the enum stringer stays aligned with the values
// exposed to end users and future log output.
func TestBuildToolString(t *testing.T) {
	if Maven.String() != "maven" {
		t.Fatalf("Maven.String() mismatch: %s", Maven.String())
	}
	if Gradle.String() != "gradle" {
		t.Fatalf("Gradle.String() mismatch: %s", Gradle.String())
	}

	unknown := BuildTool(42)
	if unknown.String() != "unknown" {
		t.Fatalf("unexpected string for unknown build tool: %s", unknown.String())
	}
}
