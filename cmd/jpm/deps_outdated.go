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

// deps_outdated.go implements "jpm deps outdated" which shows dependencies
// with newer versions available.

var depsOutdatedCmd = &cobra.Command{
	Use:   "outdated",
	Short: "Show outdated dependencies",
	Long: `Display dependencies that have newer versions available on Maven Central.

This command checks each dependency against Maven Central and shows:
  - Current version
  - Latest available version
  - Whether it's a major, minor, or patch update`,
	Args: cobra.NoArgs,
	RunE: runDepsOutdated,
}

func init() {
	depsCmd.AddCommand(depsOutdatedCmd)

	depsOutdatedCmd.Flags().String("path", ".", "Project directory containing jpm.yaml")
	depsOutdatedCmd.Flags().Bool("major", false, "Include major version updates")
	depsOutdatedCmd.Flags().Bool("json", false, "Output in JSON format")
}

func runDepsOutdated(cmd *cobra.Command, args []string) error {
	projectPath, _ := cmd.Flags().GetString("path")
	projectPath = strings.TrimSpace(projectPath)
	if projectPath == "" {
		projectPath = "."
	}
	projectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("failed to resolve project path: %w", err)
	}

	includeMajor, _ := cmd.Flags().GetBool("major")
	jsonOutput, _ := cmd.Flags().GetBool("json")

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
		fmt.Println("No dependencies to check")
		return nil
	}

	type outdatedDep struct {
		GroupID    string `json:"groupId"`
		ArtifactID string `json:"artifactId"`
		Current    string `json:"current"`
		Latest     string `json:"latest"`
		UpdateType string `json:"updateType"` // major, minor, patch
	}

	var outdated []outdatedDep
	var upToDate int

	if !jsonOutput {
		fmt.Println(headerStyle("→ checking for outdated dependencies:"))
	}

	for _, dep := range manifest.Dependencies {
		if !jsonOutput {
			fmt.Printf("  checking %s:%s...", dep.GroupID, dep.ArtifactID)
		}

		latestVersion, err := fetchLatestVersion(dep.GroupID, dep.ArtifactID)
		if err != nil {
			if !jsonOutput {
				fmt.Printf(" error: %v\n", err)
			}
			continue
		}

		if latestVersion == dep.Version {
			upToDate++
			if !jsonOutput {
				fmt.Printf(" ✓ up to date\n")
			}
			continue
		}

		updateType := getUpdateType(dep.Version, latestVersion)

		// Skip major updates unless --major flag is set
		if updateType == "major" && !includeMajor {
			if !jsonOutput {
				fmt.Printf(" %s (major update available, use --major to show)\n", dep.Version)
			}
			continue
		}

		outdated = append(outdated, outdatedDep{
			GroupID:    dep.GroupID,
			ArtifactID: dep.ArtifactID,
			Current:    dep.Version,
			Latest:     latestVersion,
			UpdateType: updateType,
		})

		if !jsonOutput {
			fmt.Printf(" %s → %s (%s)\n", dep.Version, latestVersion, updateType)
		}
	}

	if jsonOutput {
		// Output JSON
		fmt.Println("[")
		for i, d := range outdated {
			comma := ","
			if i == len(outdated)-1 {
				comma = ""
			}
			fmt.Printf(`  {"groupId": "%s", "artifactId": "%s", "current": "%s", "latest": "%s", "updateType": "%s"}%s`+"\n",
				d.GroupID, d.ArtifactID, d.Current, d.Latest, d.UpdateType, comma)
		}
		fmt.Println("]")
		return nil
	}

	// Summary
	fmt.Println()
	if len(outdated) == 0 {
		fmt.Println(headerStyle("✓ all dependencies are up to date!"))
	} else {
		fmt.Printf(headerStyle("→ summary: ")+"%d outdated, %d up to date\n", len(outdated), upToDate)
		fmt.Println("\n  Run 'jpm deps update' to update all dependencies")
		fmt.Println("  Run 'jpm deps update <group:artifact>' to update a specific dependency")
	}

	return nil
}

// getUpdateType determines if the update is major, minor, or patch
func getUpdateType(oldVersion, newVersion string) string {
	oldParts := parseVersionParts(oldVersion)
	newParts := parseVersionParts(newVersion)

	if len(oldParts) == 0 || len(newParts) == 0 {
		return "unknown"
	}

	if oldParts[0] != newParts[0] {
		return "major"
	}
	if len(oldParts) > 1 && len(newParts) > 1 && oldParts[1] != newParts[1] {
		return "minor"
	}
	return "patch"
}

// parseVersionParts splits a version string into numeric parts
func parseVersionParts(version string) []string {
	// Handle versions like "33.1.0-jre", "2.17.0", "4.13.2"
	// Remove any suffix like "-jre", "-android", "-SNAPSHOT"
	version = strings.Split(version, "-")[0]
	return strings.Split(version, ".")
}
