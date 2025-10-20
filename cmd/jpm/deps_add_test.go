package main

import (
	"strings"
	"testing"

	"github.com/KhalidEchchahid/go-jpm/internal/catalog"
)

func TestParseCoordinate(t *testing.T) {
	group, artifact, version, err := parseCoordinate("com.example:demo@1.2.3")
	if err != nil {
		t.Fatalf("parseCoordinate returned error: %v", err)
	}

	if group != "com.example" || artifact != "demo" || version != "1.2.3" {
		t.Fatalf("unexpected parse result: %s %s %s", group, artifact, version)
	}

	_, _, _, err = parseCoordinate("invalid")
	if err == nil {
		t.Fatalf("expected error for invalid coordinate")
	}

	_, _, _, err = parseCoordinate("group:artifact@v1@extra")
	if err == nil {
		t.Fatalf("expected error for coordinate with multiple @ segments")
	}
}

func TestBuildVersionCompletions_filtersAndAnnotates(t *testing.T) {
	meta := &catalog.ArtifactMetadata{
		GroupID:       "org.example",
		ArtifactID:    "demo",
		LatestVersion: "2.0.0",
		Versions:      []string{"2.0.0", "1.5.1", "1.5.0", "1.4.0"},
	}

	completions := buildVersionCompletions(meta, "org.example:demo", "1.5")
	if len(completions) != 2 {
		t.Fatalf("expected 2 completions, got %d", len(completions))
	}

	if completions[0] != "org.example:demo@1.5.1" {
		t.Fatalf("unexpected first completion: %s", completions[0])
	}

	if completions[1] != "org.example:demo@1.5.0" {
		t.Fatalf("unexpected second completion: %s", completions[1])
	}

	all := buildVersionCompletions(meta, "org.example:demo", "")
	if !strings.Contains(all[0], "latest release") {
		t.Fatalf("expected latest release annotation, got %s", all[0])
	}
}

func TestBuildArtifactCompletions_formatsAnnotations(t *testing.T) {
	suggestions := []catalog.ArtifactSuggestion{
		{GroupID: "com.example", ArtifactID: "lib", LatestVersion: "1.0.0", Description: "Test artifact"},
	}

	entries := buildArtifactCompletions(suggestions)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if !strings.HasPrefix(entries[0], "com.example:lib\t") {
		t.Fatalf("expected annotation formatting, got %s", entries[0])
	}

	parts := strings.Split(entries[0], "\t")
	if len(parts) != 2 {
		t.Fatalf("expected two parts separated by tab, got %v", parts)
	}

	if !strings.Contains(parts[1], "latest: 1.0.0") || !strings.Contains(parts[1], "Test artifact") {
		t.Fatalf("annotation missing expected content: %s", parts[1])
	}
}
