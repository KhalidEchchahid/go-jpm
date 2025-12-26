package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/KhalidEchchahid/go-jpm/internal/engine/native"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// exec.go implements "jpm exec" to run a specific Java class.

var execCmd = &cobra.Command{
	Use:   "exec <class>",
	Short: "Run a specific Java class",
	Long: `Execute a specific Java class from the project.

Unlike 'jpm run' which executes the main class defined in jpm.yaml,
'jpm exec' allows running any class with a main method.`,
	Example: `  jpm exec com.example.tools.Migrator
  jpm exec com.example.Server -- --port 8080
  jpm exec MainClass`,
	Args: cobra.MinimumNArgs(1),
	RunE: runExec,
}

func init() {
	rootCmd.AddCommand(execCmd)

	execCmd.Flags().StringP("path", "p", ".", "Project directory")
	execCmd.Flags().Bool("no-build", false, "Skip building before execution")
}

func runExec(cmd *cobra.Command, args []string) error {
	className := args[0]
	classArgs := args[1:]

	projectPath, _ := cmd.Flags().GetString("path")
	noBuild, _ := cmd.Flags().GetBool("no-build")

	projectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return err
	}

	// Load manifest
	manifestPath := filepath.Join(projectPath, "jpm.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("no jpm.yaml found (run 'jpm init')")
	}

	var manifest core.Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse jpm.yaml: %w", err)
	}

	// Build if needed
	if !noBuild {
		fmt.Println(headerStyle("→ building project:"))
		eng := native.New()
		result, err := eng.Build(projectPath, &manifest)
		if err != nil {
			return fmt.Errorf("build failed: %w\n%s", err, result.Logs)
		}
		fmt.Println("  ✓ build complete")
	}

	// Build classpath
	classesDir := filepath.Join(projectPath, ".jpm", "tmp", "classes")
	classpathFile := filepath.Join(projectPath, ".jpm", "out", "classpath")

	classpath := classesDir
	if cpData, err := os.ReadFile(classpathFile); err == nil && len(cpData) > 0 {
		classpath = classesDir + string(os.PathListSeparator) + string(cpData)
	}

	// Execute
	fmt.Printf(headerStyle("→ executing %s:\n"), className)
	fmt.Println()

	javaArgs := []string{"-cp", classpath, className}
	javaArgs = append(javaArgs, classArgs...)

	javaCmd := exec.Command("java", javaArgs...)
	javaCmd.Dir = projectPath
	javaCmd.Stdout = os.Stdout
	javaCmd.Stderr = os.Stderr
	javaCmd.Stdin = os.Stdin

	if err := javaCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("execution failed: %w", err)
	}

	return nil
}

// findClassFile looks for a class file matching the class name
func findClassFile(classesDir, className string) (string, error) {
	// Convert class name to path
	classPath := strings.ReplaceAll(className, ".", string(os.PathSeparator)) + ".class"
	fullPath := filepath.Join(classesDir, classPath)

	if _, err := os.Stat(fullPath); err == nil {
		return fullPath, nil
	}

	return "", fmt.Errorf("class not found: %s", className)
}
