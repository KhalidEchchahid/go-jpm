package main

import (
	"fmt"
	"path/filepath"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/KhalidEchchahid/go-jpm/internal/inspectors"
	"github.com/spf13/cobra"
)

// module_ls.go exposes "jpm module ls", a read-only helper that enumerates module
// names for a given project path. The command leans on the inspector factory so
// that support for new build tools automatically flows through here.

var moduleLsCmd = &cobra.Command{
	Use:   "ls [path]",
	Short: "List modules in a project",
	Long:  `List all modules defined in a multi-module project`,
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

		modules, err := inspector.ListModules(projectRoot)
		if err != nil {
			return err
		}

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
	moduleCmd.AddCommand(moduleLsCmd)

	moduleLsCmd.Flags().StringP("build-tool", "b", "maven", "Build tool: maven|gradle")
}
