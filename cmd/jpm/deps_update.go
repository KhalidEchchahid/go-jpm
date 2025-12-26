package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// deps_update.go implements "jpm deps update" which updates dependencies to their
// latest versions from Maven Central.

var depsUpdateCmd = &cobra.Command{
	Use:     "update [group:artifact]",
	Aliases: []string{"upgrade"},
	Short:   "Update dependencies to latest versions",
	Long: `Update one or all dependencies to their latest available versions from Maven Central.

If no argument is provided, all dependencies are updated.
If a specific dependency is provided (group:artifact), only that one is updated.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDepsUpdate,
}

func init() {
	depsCmd.AddCommand(depsUpdateCmd)

	depsUpdateCmd.Flags().String("path", ".", "Project directory containing jpm.yaml")
	depsUpdateCmd.Flags().Bool("dry-run", false, "Preview changes without modifying files")
	depsUpdateCmd.Flags().Bool("major", false, "Include major version updates (may break compatibility)")
}

func runDepsUpdate(cmd *cobra.Command, args []string) error {
	projectPath, _ := cmd.Flags().GetString("path")
	projectPath = strings.TrimSpace(projectPath)
	if projectPath == "" {
		projectPath = "."
	}
	projectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("failed to resolve project path: %w", err)
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	includeMajor, _ := cmd.Flags().GetBool("major")

	// Parse optional specific dependency
	var targetGroup, targetArtifact string
	if len(args) > 0 {
		targetGroup, targetArtifact, err = parseRmCoordinate(args[0])
		if err != nil {
			return err
		}
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

	if len(manifest.Dependencies) == 0 {
		fmt.Println("No dependencies to update")
		return nil
	}

	fmt.Println(headerStyle("→ checking for updates:"))

	var updates []dependencyUpdate
	for i, dep := range manifest.Dependencies {
		// Skip if targeting specific dependency and this isn't it
		if targetGroup != "" && (dep.GroupID != targetGroup || dep.ArtifactID != targetArtifact) {
			continue
		}

		fmt.Printf("  checking %s:%s...", dep.GroupID, dep.ArtifactID)

		latestVersion, err := fetchLatestVersion(dep.GroupID, dep.ArtifactID)
		if err != nil {
			fmt.Printf(" error: %v\n", err)
			continue
		}

		if latestVersion == dep.Version {
			fmt.Printf(" up to date (%s)\n", dep.Version)
			continue
		}

		// Check if it's a major version change
		if !includeMajor && isMajorUpdate(dep.Version, latestVersion) {
			fmt.Printf(" %s → %s (major update, use --major to include)\n", dep.Version, latestVersion)
			continue
		}

		fmt.Printf(" %s → %s\n", dep.Version, latestVersion)
		updates = append(updates, dependencyUpdate{
			index:      i,
			dep:        dep,
			newVersion: latestVersion,
		})
	}

	if len(updates) == 0 {
		fmt.Println("\nAll dependencies are up to date!")
		return nil
	}

	if dryRun {
		fmt.Println("\n" + headerStyle("→ would update:"))
		for _, u := range updates {
			fmt.Printf("  %s:%s %s → %s\n", u.dep.GroupID, u.dep.ArtifactID, u.dep.Version, u.newVersion)
		}
		fmt.Println("  dry run: jpm.yaml was not modified")
		return nil
	}

	// Apply updates
	for _, u := range updates {
		manifest.Dependencies[u.index].Version = u.newVersion
	}

	// Write back to jpm.yaml
	newData, err := yaml.Marshal(&manifest)
	if err != nil {
		return fmt.Errorf("failed to serialize manifest: %w", err)
	}

	if err := os.WriteFile(manifestPath, newData, 0644); err != nil {
		return fmt.Errorf("failed to write jpm.yaml: %w", err)
	}

	// Also update pom.xml if it exists
	pomPath := filepath.Join(projectPath, ".jpm", "maven", "pom.xml")
	if _, err := os.Stat(pomPath); err == nil {
		if err := syncManifestToPom(manifest, pomPath); err != nil {
			fmt.Printf("  warning: failed to sync pom.xml: %v\n", err)
		}
	}

	fmt.Println("\n" + headerStyle("→ updated:"))
	for _, u := range updates {
		fmt.Printf("  %s:%s %s → %s\n", u.dep.GroupID, u.dep.ArtifactID, u.dep.Version, u.newVersion)
	}
	fmt.Printf("  file %s\n", manifestPath)

	return nil
}

type dependencyUpdate struct {
	index      int
	dep        core.Dependency
	newVersion string
}

// fetchLatestVersion gets the latest version from Maven Central
func fetchLatestVersion(groupID, artifactID string) (string, error) {
	// Use Maven Central metadata API
	groupPath := strings.ReplaceAll(groupID, ".", "/")
	metadataURL := fmt.Sprintf("https://repo1.maven.org/maven2/%s/%s/maven-metadata.xml",
		groupPath, artifactID)

	resp, err := http.Get(metadataURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("metadata not found (HTTP %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read metadata: %w", err)
	}

	var metadata mavenMetadata
	if err := xml.Unmarshal(body, &metadata); err != nil {
		return "", fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Try to get the release version first, then latest, then newest from versions
	if metadata.Versioning.Release != "" {
		return metadata.Versioning.Release, nil
	}
	if metadata.Versioning.Latest != "" {
		return metadata.Versioning.Latest, nil
	}
	if len(metadata.Versioning.Versions) > 0 {
		// Sort and get the latest
		versions := metadata.Versioning.Versions
		sort.Strings(versions)
		return versions[len(versions)-1], nil
	}

	return "", fmt.Errorf("no versions found")
}

type mavenMetadata struct {
	XMLName    xml.Name `xml:"metadata"`
	GroupID    string   `xml:"groupId"`
	ArtifactID string   `xml:"artifactId"`
	Versioning struct {
		Latest   string   `xml:"latest"`
		Release  string   `xml:"release"`
		Versions []string `xml:"versions>version"`
	} `xml:"versioning"`
}

// isMajorUpdate checks if the version change is a major version bump
func isMajorUpdate(oldVersion, newVersion string) bool {
	oldMajor := extractMajorVersion(oldVersion)
	newMajor := extractMajorVersion(newVersion)
	return oldMajor != "" && newMajor != "" && oldMajor != newMajor
}

func extractMajorVersion(version string) string {
	// Handle versions like "33.1.0-jre", "2.17.0", "4.13.2"
	parts := strings.Split(version, ".")
	if len(parts) > 0 {
		// Remove any prefix characters (like 'v')
		major := strings.TrimLeft(parts[0], "v")
		return major
	}
	return ""
}
