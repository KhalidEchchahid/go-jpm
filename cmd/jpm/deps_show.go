package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/KhalidEchchahid/go-jpm/internal/inspectors"
	"github.com/spf13/cobra"
)

// deps_show.go implements "jpm deps show" which mirrors the way Java builders
// list declared dependencies. It delegates the heavy lifting to build-tool
// inspectors and focuses on presenting the data in a readable format.

var depsShowCmd = &cobra.Command{
	Use:   "show [path]",
	Short: "Display declared dependencies",
	Long:  "Display the dependencies declared in a project's build definition",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		buildToolStr, _ := cmd.Flags().GetString("build-tool")
		buildTool, err := core.ParseBuildTool(buildToolStr)
		if err != nil {
			return err
		}

		projectRoot := "."
		if len(args) > 0 {
			projectRoot = args[0]
		}
		projectRoot, err = filepath.Abs(projectRoot)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		factory := inspectors.NewFactory()
		inspector, err := factory.ForTool(buildTool)
		if err != nil {
			return err
		}

		deps, err := inspector.ListDependencies(projectRoot)
		if err != nil {
			return err
		}

		fmt.Println("→ dependencies:")
		if len(deps) == 0 {
			fmt.Println("  (none)")
			return nil
		}

		for i, dep := range deps {
			fmt.Printf("  %d) %s\n", i+1, formatDependency(dep))
		}

		return nil
	},
}

func init() {
	depsCmd.AddCommand(depsShowCmd)
	depsShowCmd.Flags().StringP("build-tool", "b", "maven", "Build tool: maven|gradle")
}

// formatDependency converts a Dependency struct into a human-friendly line that
// matches the CLI output contract shared across tests and documentation.
func formatDependency(dep core.Dependency) string {
	identifier := fmt.Sprintf("%s:%s", dep.GroupID, dep.ArtifactID)
	if dep.Version != "" {
		identifier = fmt.Sprintf("%s:%s", identifier, dep.Version)
	}

	var qualifiers []string
	if dep.Scope != "" {
		qualifiers = append(qualifiers, fmt.Sprintf("scope=%s", dep.Scope))
	}
	if dep.Type != "" {
		qualifiers = append(qualifiers, fmt.Sprintf("type=%s", dep.Type))
	}
	if dep.Classifier != "" {
		qualifiers = append(qualifiers, fmt.Sprintf("classifier=%s", dep.Classifier))
	}
	if dep.Optional {
		qualifiers = append(qualifiers, "optional")
	}

	if len(qualifiers) == 0 {
		return identifier
	}

	return fmt.Sprintf("%s [%s]", identifier, strings.Join(qualifiers, ", "))
}
