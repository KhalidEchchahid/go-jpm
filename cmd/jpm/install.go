package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/KhalidEchchahid/go-jpm/internal/engine/native"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// install.go implements "jpm install" which installs the project to the local repository.

var installCmd = &cobra.Command{
	Use:   "install [path]",
	Short: "Install project to local repository",
	Long: `Install the project artifact to the local Maven repository (~/.m2/repository).

This makes the artifact available for other local projects to use as a dependency.

The install command:
  1. Builds the project (if not already built)
  2. Copies the JAR to ~/.m2/repository/<groupId>/<artifactId>/<version>/
  3. Generates and installs the POM file

Examples:
  jpm install           Install current project
  jpm install ./mylib   Install project in mylib directory
  jpm install --skip-build  Install without rebuilding`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)

	installCmd.Flags().Bool("skip-build", false, "Skip building before install")
	installCmd.Flags().StringP("path", "p", ".", "Project path")
}

func runInstall(cmd *cobra.Command, args []string) error {
	projectPath := "."
	if len(args) > 0 {
		projectPath = args[0]
	}

	projectPath = strings.TrimSpace(projectPath)
	projectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("failed to resolve project path: %w", err)
	}

	skipBuild, _ := cmd.Flags().GetBool("skip-build")

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

	// Validate required fields
	if manifest.Project.GroupID == "" {
		return fmt.Errorf("project.group-id is required for installation")
	}
	if manifest.Project.ArtifactID == "" {
		return fmt.Errorf("project.artifact-id is required for installation")
	}
	if manifest.Project.Version == "" {
		manifest.Project.Version = "0.1.0-SNAPSHOT"
	}

	jarName := fmt.Sprintf("%s-%s.jar", manifest.Project.ArtifactID, manifest.Project.Version)
	jarPath := filepath.Join(projectPath, ".jpm", "out", jarName)

	// Build if needed
	if !skipBuild {
		fmt.Println(headerStyle("→ building project:"))

		eng := native.New()
		result, err := eng.Build(projectPath, &manifest)
		if err != nil {
			return fmt.Errorf("build failed: %w\n%s", err, result.Logs)
		}

		jarPath = result.ArtifactPath
		fmt.Println("  ✓ build complete")
	} else {
		// Check if JAR exists
		if _, err := os.Stat(jarPath); os.IsNotExist(err) {
			return fmt.Errorf("no JAR found at %s (run without --skip-build)", jarPath)
		}
		fmt.Println(headerStyle("→ skipping build"))
	}

	// Determine local repository path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine home directory: %w", err)
	}

	m2RepoPath := filepath.Join(homeDir, ".m2", "repository")

	// Build repository path: groupId/artifactId/version/
	groupPath := strings.ReplaceAll(manifest.Project.GroupID, ".", string(os.PathSeparator))
	artifactDir := filepath.Join(m2RepoPath, groupPath, manifest.Project.ArtifactID, manifest.Project.Version)

	fmt.Println(headerStyle("→ installing to local repository:"))

	// Create directory structure
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return fmt.Errorf("failed to create repository directory: %w", err)
	}

	// Copy JAR file
	destJarPath := filepath.Join(artifactDir, jarName)
	if err := copyFileForInstall(jarPath, destJarPath); err != nil {
		return fmt.Errorf("failed to copy JAR: %w", err)
	}
	fmt.Printf("  installed: %s\n", destJarPath)

	// Generate and write POM file
	pomPath := filepath.Join(artifactDir, fmt.Sprintf("%s-%s.pom", manifest.Project.ArtifactID, manifest.Project.Version))
	pomContent := generatePOM(manifest)
	if err := os.WriteFile(pomPath, []byte(pomContent), 0644); err != nil {
		return fmt.Errorf("failed to write POM: %w", err)
	}
	fmt.Printf("  installed: %s\n", pomPath)

	// Print summary
	fmt.Println()
	fmt.Printf("  "+headerStyle("✓ installed:")+" %s:%s:%s\n",
		manifest.Project.GroupID, manifest.Project.ArtifactID, manifest.Project.Version)
	fmt.Printf("  location: %s\n", artifactDir)

	return nil
}

// copyFileForInstall copies a file from src to dst
func copyFileForInstall(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = dstFile.ReadFrom(srcFile)
	return err
}

// generatePOM generates a Maven POM file from the manifest
func generatePOM(manifest core.Manifest) string {
	var sb strings.Builder

	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

`)

	sb.WriteString(fmt.Sprintf("    <groupId>%s</groupId>\n", manifest.Project.GroupID))
	sb.WriteString(fmt.Sprintf("    <artifactId>%s</artifactId>\n", manifest.Project.ArtifactID))
	sb.WriteString(fmt.Sprintf("    <version>%s</version>\n", manifest.Project.Version))
	sb.WriteString("    <packaging>jar</packaging>\n")

	sb.WriteString("\n")

	// Java version properties
	if manifest.Java.Version != "" {
		sb.WriteString("    <properties>\n")
		sb.WriteString(fmt.Sprintf("        <maven.compiler.source>%s</maven.compiler.source>\n", manifest.Java.Version))
		sb.WriteString(fmt.Sprintf("        <maven.compiler.target>%s</maven.compiler.target>\n", manifest.Java.Version))
		sb.WriteString("    </properties>\n\n")
	}

	// Dependencies
	if len(manifest.Dependencies) > 0 {
		sb.WriteString("    <dependencies>\n")
		for _, dep := range manifest.Dependencies {
			sb.WriteString("        <dependency>\n")
			sb.WriteString(fmt.Sprintf("            <groupId>%s</groupId>\n", dep.GroupID))
			sb.WriteString(fmt.Sprintf("            <artifactId>%s</artifactId>\n", dep.ArtifactID))
			sb.WriteString(fmt.Sprintf("            <version>%s</version>\n", dep.Version))
			if dep.Scope != "" && dep.Scope != "compile" {
				sb.WriteString(fmt.Sprintf("            <scope>%s</scope>\n", dep.Scope))
			}
			sb.WriteString("        </dependency>\n")
		}
		sb.WriteString("    </dependencies>\n")
	}

	sb.WriteString("</project>\n")

	return sb.String()
}
