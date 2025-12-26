package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/KhalidEchchahid/go-jpm/internal/engine/native"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// repl.go implements "jpm repl" to start JShell with project dependencies.

var replCmd = &cobra.Command{
	Use:   "repl",
	Short: "Start JShell with project dependencies",
	Long: `Start an interactive JShell session with all project dependencies
pre-loaded in the classpath.

This allows you to experiment with your project's dependencies
in an interactive REPL environment.`,
	Example: `  jpm repl
  # Then in JShell:
  # jshell> import com.google.common.collect.*;
  # jshell> var list = ImmutableList.of(1, 2, 3);`,
	RunE: runRepl,
}

func init() {
	rootCmd.AddCommand(replCmd)

	replCmd.Flags().StringP("path", "p", ".", "Project directory")
	replCmd.Flags().Bool("no-startup", false, "Don't show JShell startup message")
}

func runRepl(cmd *cobra.Command, args []string) error {
	projectPath, _ := cmd.Flags().GetString("path")
	noStartup, _ := cmd.Flags().GetBool("no-startup")

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

	// Resolve dependencies
	cacheDir := filepath.Join(projectPath, ".jpm", "cache")
	cache := native.NewCache(cacheDir)
	downloader := native.NewDownloader("", cache)
	resolver := native.NewResolver(downloader, cache)

	coreDeps := make([]core.Dependency, 0)
	for _, d := range manifest.Dependencies {
		if d.Scope != "test" {
			coreDeps = append(coreDeps, core.Dependency{
				GroupID:    d.GroupID,
				ArtifactID: d.ArtifactID,
				Version:    d.Version,
				Scope:      d.Scope,
			})
		}
	}

	fmt.Println(headerStyle("→ resolving dependencies for REPL:"))

	resolved, err := resolver.Resolve(coreDeps)
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Build classpath from resolved dependencies
	classpathJars := native.ClasspathJars(resolved, "compile", "runtime")

	// Also add project classes if they exist
	classesDir := filepath.Join(projectPath, ".jpm", "tmp", "classes")
	if _, err := os.Stat(classesDir); err == nil {
		classpathJars = append([]string{classesDir}, classpathJars...)
	}

	classpath := ""
	if len(classpathJars) > 0 {
		for i, jar := range classpathJars {
			if i > 0 {
				classpath += string(os.PathListSeparator)
			}
			classpath += jar
		}
	}

	fmt.Printf("  loaded %d dependencies\n", len(resolved))
	fmt.Println()
	fmt.Println(headerStyle("→ starting JShell:"))
	fmt.Println()

	// Build JShell command
	jshellArgs := []string{}
	if classpath != "" {
		jshellArgs = append(jshellArgs, "--class-path", classpath)
	}
	if noStartup {
		jshellArgs = append(jshellArgs, "-q")
	}

	// Run JShell interactively
	jshellCmd := exec.Command("jshell", jshellArgs...)
	jshellCmd.Dir = projectPath
	jshellCmd.Stdout = os.Stdout
	jshellCmd.Stderr = os.Stderr
	jshellCmd.Stdin = os.Stdin

	if err := jshellCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("JShell failed: %w", err)
	}

	return nil
}
