package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// deps_rm.go implements "jpm deps rm" which removes a dependency from the project's
// jpm.yaml manifest file.

var depsRmCmd = &cobra.Command{
	Use:     "rm <group:artifact>",
	Aliases: []string{"remove", "delete"},
	Short:   "Remove a dependency from the project",
	Long:    "Remove a dependency declaration from the project's jpm.yaml manifest.",
	Args:    cobra.ExactArgs(1),
	RunE:    runDepsRm,
}

func init() {
	depsCmd.AddCommand(depsRmCmd)

	depsRmCmd.Flags().String("path", ".", "Project directory containing jpm.yaml")
	depsRmCmd.Flags().Bool("dry-run", false, "Preview changes without modifying files")
}

func runDepsRm(cmd *cobra.Command, args []string) error {
	coordinate := args[0]

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

	// Parse coordinate (group:artifact, version is ignored for removal)
	groupID, artifactID, err := parseRmCoordinate(coordinate)
	if err != nil {
		return err
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

	// Find and remove the dependency
	found := false
	var newDeps []core.Dependency
	var removedDep core.Dependency

	for _, dep := range manifest.Dependencies {
		if dep.GroupID == groupID && dep.ArtifactID == artifactID {
			found = true
			removedDep = dep
		} else {
			newDeps = append(newDeps, dep)
		}
	}

	if !found {
		return fmt.Errorf("dependency %s:%s not found in jpm.yaml", groupID, artifactID)
	}

	manifest.Dependencies = newDeps

	// Write back to jpm.yaml
	if dryRun {
		fmt.Println(headerStyle("→ dependency removal:"))
		fmt.Printf("  would remove %s:%s:%s\n", removedDep.GroupID, removedDep.ArtifactID, removedDep.Version)
		fmt.Println("  dry run: jpm.yaml was not modified")
		return nil
	}

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

	fmt.Println(headerStyle("→ dependency removed:"))
	fmt.Printf("  removed %s:%s:%s\n", removedDep.GroupID, removedDep.ArtifactID, removedDep.Version)
	fmt.Printf("  file %s\n", manifestPath)

	return nil
}

// parseRmCoordinate parses group:artifact (version optional, ignored)
func parseRmCoordinate(coord string) (groupID, artifactID string, err error) {
	// Remove version if present (group:artifact@version or group:artifact:version)
	coord = strings.Split(coord, "@")[0]

	parts := strings.Split(coord, ":")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid coordinate %q: expected group:artifact", coord)
	}

	groupID = strings.TrimSpace(parts[0])
	artifactID = strings.TrimSpace(parts[1])

	if groupID == "" || artifactID == "" {
		return "", "", fmt.Errorf("invalid coordinate %q: group and artifact cannot be empty", coord)
	}

	return groupID, artifactID, nil
}

// syncManifestToPom updates pom.xml to match the manifest dependencies
func syncManifestToPom(manifest core.Manifest, pomPath string) error {
	// Read existing pom.xml
	data, err := os.ReadFile(pomPath)
	if err != nil {
		return err
	}

	pomContent := string(data)

	// Build new dependencies XML
	var depsXML strings.Builder
	depsXML.WriteString("\n    <dependencies>\n")
	for _, dep := range manifest.Dependencies {
		depsXML.WriteString("        <dependency>\n")
		depsXML.WriteString(fmt.Sprintf("            <groupId>%s</groupId>\n", dep.GroupID))
		depsXML.WriteString(fmt.Sprintf("            <artifactId>%s</artifactId>\n", dep.ArtifactID))
		depsXML.WriteString(fmt.Sprintf("            <version>%s</version>\n", dep.Version))
		if dep.Scope != "" {
			depsXML.WriteString(fmt.Sprintf("            <scope>%s</scope>\n", dep.Scope))
		}
		if dep.Type != "" {
			depsXML.WriteString(fmt.Sprintf("            <type>%s</type>\n", dep.Type))
		}
		if dep.Classifier != "" {
			depsXML.WriteString(fmt.Sprintf("            <classifier>%s</classifier>\n", dep.Classifier))
		}
		if dep.Optional {
			depsXML.WriteString("            <optional>true</optional>\n")
		}
		depsXML.WriteString("        </dependency>\n")
	}
	depsXML.WriteString("    </dependencies>")

	// Replace dependencies section in pom.xml
	startIdx := strings.Index(pomContent, "<dependencies>")
	endIdx := strings.Index(pomContent, "</dependencies>")

	if startIdx != -1 && endIdx != -1 {
		// Find the start of the line containing <dependencies>
		lineStart := strings.LastIndex(pomContent[:startIdx], "\n") + 1
		newPom := pomContent[:lineStart] + depsXML.String() + pomContent[endIdx+len("</dependencies>"):]
		return os.WriteFile(pomPath, []byte(newPom), 0644)
	}

	return nil
}
