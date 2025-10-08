package main

import (
	"fmt"
	"path/filepath"

	"github.com/hicham-amazigh/jpm/internal/core"
	"github.com/spf13/cobra"
)

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

		// Get project root
		projectRoot := "."
		if len(args) > 0 {
			projectRoot = args[0]
		}
		projectRoot, err = filepath.Abs(projectRoot)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		// Create inspector
		factory := core.NewInspectorFactory()
		inspector, err := factory.ForTool(buildTool)
		if err != nil {
			return err
		}

		// List modules
		modules, err := inspector.ListModules(projectRoot)
		if err != nil {
			return err
		}

		// Print results
		fmt.Println("→ found:")
		if len(modules) == 0 {
			fmt.Println("  (none)")
		} else {
			for i, module := range modules {
				fmt.Printf("  %d) %s\n", i+1, module)
			}
		}

		return nil
	},
}

func init() {
	moduleCmd.AddCommand(findCmd)

	findCmd.Flags().StringP("build-tool", "b", "maven", "Build tool: maven|gradle")
}