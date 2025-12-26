package main

import (
	"strings"
	"testing"

	"github.com/KhalidEchchahid/go-jpm/internal/catalog"
	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/spf13/cobra"
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

func TestRunDepsAddManifestAddsDependency(t *testing.T) {
	tmp := t.TempDir()
	manifest := &core.Manifest{}
	manifest.BuildTool = "maven"
	manifest.Engine = "maven"
	manifest.Project.GroupID = "com.example"
	manifest.Project.ArtifactID = "demo"
	manifest.Project.Version = "0.1.0"
	if _, err := core.SaveManifest(tmp, manifest); err != nil {
		t.Fatalf("SaveManifest returned error: %v", err)
	}

	cmd := newTestDepsAddCommand()
	if err := cmd.Flags().Set("path", tmp); err != nil {
		t.Fatalf("failed to set path flag: %v", err)
	}
	if err := cmd.Flags().Set("scope", "test"); err != nil {
		t.Fatalf("failed to set scope flag: %v", err)
	}
	if err := cmd.Flags().Set("type", "jar"); err != nil {
		t.Fatalf("failed to set type flag: %v", err)
	}
	if err := cmd.Flags().Set("classifier", "tests"); err != nil {
		t.Fatalf("failed to set classifier flag: %v", err)
	}
	if err := cmd.Flags().Set("optional", "true"); err != nil {
		t.Fatalf("failed to set optional flag: %v", err)
	}

	if err := runDepsAdd(cmd, []string{"org.junit.jupiter:junit-jupiter@5.11.0"}); err != nil {
		t.Fatalf("runDepsAdd returned error: %v", err)
	}

	updated, _, err := core.LoadManifest(tmp)
	if err != nil {
		t.Fatalf("LoadManifest returned error: %v", err)
	}
	if len(updated.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(updated.Dependencies))
	}
	dep := updated.Dependencies[0]
	if dep.GroupID != "org.junit.jupiter" || dep.ArtifactID != "junit-jupiter" {
		t.Fatalf("unexpected dependency coordinates: %+v", dep)
	}
	if dep.Version != "5.11.0" {
		t.Fatalf("expected version 5.11.0, got %s", dep.Version)
	}
	if dep.Scope != "test" {
		t.Fatalf("expected scope test, got %s", dep.Scope)
	}
	if dep.Type != "jar" {
		t.Fatalf("expected type jar, got %s", dep.Type)
	}
	if dep.Classifier != "tests" {
		t.Fatalf("expected classifier tests, got %s", dep.Classifier)
	}
	if !dep.Optional {
		t.Fatalf("expected optional true")
	}
}

func TestRunDepsAddManifestUpdatesDependency(t *testing.T) {
	tmp := t.TempDir()
	manifest := &core.Manifest{}
	manifest.BuildTool = "maven"
	manifest.Engine = "maven"
	manifest.Project.GroupID = "com.example"
	manifest.Project.ArtifactID = "demo"
	manifest.Project.Version = "0.1.0"
	manifest.Dependencies = []core.Dependency{{
		GroupID:    "org.junit.jupiter",
		ArtifactID: "junit-jupiter",
		Version:    "5.10.0",
		Scope:      "test",
		Optional:   true,
	}}
	if _, err := core.SaveManifest(tmp, manifest); err != nil {
		t.Fatalf("SaveManifest returned error: %v", err)
	}

	cmd := newTestDepsAddCommand()
	if err := cmd.Flags().Set("path", tmp); err != nil {
		t.Fatalf("failed to set path flag: %v", err)
	}
	if err := cmd.Flags().Set("version", "5.12.0"); err != nil {
		t.Fatalf("failed to set version flag: %v", err)
	}

	if err := runDepsAdd(cmd, []string{"org.junit.jupiter:junit-jupiter"}); err != nil {
		t.Fatalf("runDepsAdd returned error: %v", err)
	}

	updated, _, err := core.LoadManifest(tmp)
	if err != nil {
		t.Fatalf("LoadManifest returned error: %v", err)
	}
	if len(updated.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(updated.Dependencies))
	}
	dep := updated.Dependencies[0]
	if dep.Version != "5.12.0" {
		t.Fatalf("expected version 5.12.0, got %s", dep.Version)
	}
	if dep.Scope != "test" {
		t.Fatalf("expected scope to remain test, got %s", dep.Scope)
	}
	if !dep.Optional {
		t.Fatalf("expected optional to remain true")
	}
}

func TestRunDepsAddManifestDryRun(t *testing.T) {
	tmp := t.TempDir()
	manifest := &core.Manifest{}
	manifest.BuildTool = "maven"
	manifest.Engine = "maven"
	manifest.Project.GroupID = "com.example"
	manifest.Project.ArtifactID = "demo"
	manifest.Project.Version = "0.1.0"
	manifest.Dependencies = []core.Dependency{{
		GroupID:    "org.assertj",
		ArtifactID: "assertj-core",
		Version:    "3.24.0",
	}}
	if _, err := core.SaveManifest(tmp, manifest); err != nil {
		t.Fatalf("SaveManifest returned error: %v", err)
	}

	cmd := newTestDepsAddCommand()
	if err := cmd.Flags().Set("path", tmp); err != nil {
		t.Fatalf("failed to set path flag: %v", err)
	}
	if err := cmd.Flags().Set("version", "3.25.0"); err != nil {
		t.Fatalf("failed to set version flag: %v", err)
	}
	if err := cmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatalf("failed to set dry-run flag: %v", err)
	}

	if err := runDepsAdd(cmd, []string{"org.assertj:assertj-core"}); err != nil {
		t.Fatalf("runDepsAdd returned error: %v", err)
	}

	reloaded, _, err := core.LoadManifest(tmp)
	if err != nil {
		t.Fatalf("LoadManifest returned error: %v", err)
	}
	if reloaded.Dependencies[0].Version != "3.24.0" {
		t.Fatalf("expected version to remain 3.24.0, got %s", reloaded.Dependencies[0].Version)
	}
}

func newTestDepsAddCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().StringP("build-tool", "b", "maven", "")
	cmd.Flags().String("path", ".", "")
	cmd.Flags().String("version", "", "")
	cmd.Flags().StringP("scope", "s", "", "")
	cmd.Flags().String("type", "", "")
	cmd.Flags().String("classifier", "", "")
	cmd.Flags().Bool("optional", false, "")
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("list-versions", false, "")
	return cmd
}
