package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [project name]",
	Short: "Initialize a simple Java project",
	Long:  "Initialize a Java project with a flat src/ layout and a hidden .jpm build folder (Maven prototype)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1) Java presence check (warn but allow proceed)
		if _, err := exec.LookPath("java"); err != nil {
			fmt.Fprintln(os.Stderr, "! Java not found on PATH. You can continue to scaffold; install JDK before building.")
		}

		// 2) Parse flags
		buildToolStr, _ := cmd.Flags().GetString("build-tool")
		buildTool, err := core.ParseBuildTool(buildToolStr)
		if err != nil {
			return err
		}
		groupID, _ := cmd.Flags().GetString("group-id")
		artifactIDFlag, _ := cmd.Flags().GetString("artifact-id")
		version, _ := cmd.Flags().GetString("version")
		javaVersion, _ := cmd.Flags().GetString("java-version")
		force, _ := cmd.Flags().GetBool("force")

		// 3) Determine project directory and name
		projectDir := "."
		if len(args) == 1 {
			projectDir = args[0]
		}
		absDir, err := filepath.Abs(projectDir)
		if err != nil {
			return fmt.Errorf("failed to resolve project path: %w", err)
		}
		projectName := filepath.Base(absDir)
		artifactID := artifactIDFlag
		if artifactID == "" {
			artifactID = sanitizeArtifactID(projectName)
		}

		// Interactive prompts (prompt-first UX) when flags not provided
		reader := bufio.NewReader(os.Stdin)
		if len(args) == 0 {
			choice := strings.ToLower(prompt(reader, "Create project here? (Y/n)", "Y"))
			if choice == "n" || choice == "no" || choice == "2" {
				name := prompt(reader, "Project folder name", projectName)
				if name != "." && name != "" {
					absDir = filepath.Join(absDir, name)
					projectName = name
					artifactID = sanitizeArtifactID(projectName)
				}
			}
		}

		if !cmd.Flags().Changed("group-id") {
			groupID = prompt(reader, "Group ID", defaultString(groupID, "app"))
		}
		if !cmd.Flags().Changed("artifact-id") {
			artifactID = prompt(reader, "Artifact ID", artifactID)
		}
		if !cmd.Flags().Changed("version") {
			version = prompt(reader, "Version", defaultString(version, "0.1.0-SNAPSHOT"))
		}
		if !cmd.Flags().Changed("java-version") {
			javaVersion = prompt(reader, "Java version", defaultString(javaVersion, "21"))
		}

		// Preview and confirm
		fmt.Println("\nPreview")
		fmt.Printf("  - Create: %s/\n", projectName)
		fmt.Println("  - Create: src/")
		fmt.Println("  - Create: .jpm/maven/pom.xml")
		fmt.Println("  - Link:   .jpm/maven/src/main/java -> ./src")
		fmt.Println("  - Create: jpm.yaml")
		if empty, _ := dirEmpty(filepath.Join(absDir, "src")); empty {
			fmt.Println("  - Create: src/Main.java (hello world)")
		}
		proceed := strings.ToLower(prompt(reader, "Proceed? (Y/n)", "Y"))
		if proceed == "n" || proceed == "no" {
			return fmt.Errorf("init cancelled")
		}

		// 4) Validate directory (create if missing)
		if _, err := os.Stat(absDir); os.IsNotExist(err) {
			if err := os.MkdirAll(absDir, 0o755); err != nil {
				return fmt.Errorf("failed to create project directory: %w", err)
			}
		} else if err == nil {
			// If exists and not empty, require --force
			entries, _ := os.ReadDir(absDir)
			if len(entries) > 0 && !force {
				return fmt.Errorf("directory '%s' is not empty (use --force to proceed)", absDir)
			}
		} else {
			return err
		}

		// 5) Create src/ (flat, user-controlled)
		srcDir := filepath.Join(absDir, "src")
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			return fmt.Errorf("failed to create src/: %w", err)
		}

		// Optional hello world if src is empty
		if empty, _ := dirEmpty(srcDir); empty {
			helloPath := filepath.Join(srcDir, "Main.java")
			_ = os.WriteFile(helloPath, []byte(helloWorldJava()), 0o644)
		}

		// 6) Initialize hidden build folder based on build tool (Maven prototype)
		dotRoot := filepath.Join(absDir, ".jpm")
		if err := os.MkdirAll(dotRoot, 0o755); err != nil {
			return fmt.Errorf("failed to create .jpm folder: %w", err)
		}

		switch buildTool {
		case core.Maven:
			mavenRoot := filepath.Join(dotRoot, "maven")
			if err := os.MkdirAll(filepath.Join(mavenRoot, "src", "main"), 0o755); err != nil {
				return fmt.Errorf("failed to create maven layout: %w", err)
			}
			// Symlink .jpm/maven/src/main/java -> <project>/src (with fallback)
			if err := ensureJavaLink(absDir, mavenRoot); err != nil {
				return fmt.Errorf("failed to prepare java link: %w", err)
			}
			// Write minimal pom.xml
			pomPath := filepath.Join(mavenRoot, "pom.xml")
			if err := writeFileIfAbsent(pomPath, []byte(minimalPom(groupID, artifactID, version, javaVersion))); err != nil {
				return err
			}
		default:
			return fmt.Errorf("build tool '%s' not yet supported in init", buildTool.String())
		}

		// 7) Create project manifest (YAML)
		manifestPath := filepath.Join(absDir, "jpm.yaml")
		if err := writeFileIfAbsent(manifestPath, []byte(projectManifestYAML(projectName, buildTool.String(), groupID, artifactID, version, javaVersion))); err != nil {
			return err
		}

		fmt.Printf("\n✔ Project initialized at %s\n", absDir)
		fmt.Println("Next steps:")
		fmt.Println("  - Add your Java files under src/")
		fmt.Println("  - jpm build")
		fmt.Println("  - jpm run")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringP("build-tool", "b", "maven", "Build tool to set up (maven)")
	initCmd.Flags().String("group-id", "app", "Group ID for the project")
	initCmd.Flags().String("artifact-id", "", "Artifact ID (defaults to sanitized project/folder name)")
	initCmd.Flags().String("version", "0.1.0-SNAPSHOT", "Initial project version")
	initCmd.Flags().String("java-version", "21", "Target Java version for compiler")
	initCmd.Flags().Bool("force", false, "Proceed even if target directory is not empty")
}

// sanitizeArtifactID makes a simple, Maven-friendly artifactId from a name.
func sanitizeArtifactID(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}

// defaultString returns b if a is empty, otherwise a.
func defaultString(a, b string) string {
	if strings.TrimSpace(a) == "" {
		return b
	}
	return a
}

func dirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	if err == nil {
		return false, nil
	}
	return true, nil
}

func writeFileIfAbsent(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, data, 0o644)
}

func minimalPom(groupID, artifactID, version, javaVersion string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
  <modelVersion>4.0.0</modelVersion>
  <groupId>%s</groupId>
  <artifactId>%s</artifactId>
  <version>%s</version>
  <properties>
    <maven.compiler.source>%s</maven.compiler.source>
    <maven.compiler.target>%s</maven.compiler.target>
  </properties>
</project>
`, groupID, artifactID, version, javaVersion, javaVersion)
}

func projectManifestYAML(name, tool, groupID, artifactID, version, javaVersion string) string {
	return fmt.Sprintf(`# JPM project manifest (prototype)
name: %s
build_tool: %s
engine: maven
java:
  version: "%s"
project:
  group_id: %s
  artifact_id: %s
  version: %s
app:
  main_class: Main
dependencies: []
`, name, tool, javaVersion, groupID, artifactID, version)
}

func helloWorldJava() string {
	return "" +
		"public class Main {\n" +
		"  public static void main(String[] args) {\n" +
		"    System.out.println(\"Hello, JPM!\");\n" +
		"  }\n" +
		"}\n"
}
