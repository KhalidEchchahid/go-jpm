package main

import (
	"fmt"
	"path/filepath"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/KhalidEchchahid/go-jpm/internal/inspectors"
	"github.com/spf13/cobra"
)

// find.go exposes "jpm module find", a read-only helper that enumerates module
// names for a given project path. The command leans on the inspector factory so
// that support for new build tools automatically flows through here.

// findCmd represents the find command
var findCmd = &cobra.Command{
	Use:   "find [path]",
	Short: "List modules in a project",
	Long:  `List all modules defined in a multi-module project`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get build tool
		buildToolStr, _ := cmd.Flags().GetString("build-tool")
		buildTool, err := core.ParseBuildTool(buildToolStr)
		if err != nil {
			return err
		}

		// Resolve the project root to an absolute path to keep inspector
		// implementations free from working-directory assumptions.
		projectRoot := "."
		if len(args) > 0 {
			projectRoot = args[0]
		}
		projectRoot, err = filepath.Abs(projectRoot)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		// Create inspector
		factory := inspectors.NewFactory()
		inspector, err := factory.ForTool(buildTool)
		if err != nil {
			return err
		}

		// List modules using the selected inspector. Each adapter encapsulates how
		// a given build tool models multi-module projects.
		modules, err := inspector.ListModules(projectRoot)
		if err != nil {
			return err
		}

		// Print results in a human-friendly list. The formatter intentionally keeps
		// output stable while adding light ANSI color styling for TTY users.
		fmt.Println(headerStyle("→ found:"))
		if len(modules) == 0 {
			fmt.Println(subduedStyle("  (none)"))
		} else {
			for i, module := range modules {
				fmt.Printf("  %s %s\n", formatIndex(i+1), primaryTextStyle(module))
			}
		}

		return nil
	},
}

func init() {
	moduleCmd.AddCommand(findCmd)

	findCmd.Flags().StringP("build-tool", "b", "maven", "Build tool: maven|gradle")
}
