package main

import (
	"fmt"
	"path/filepath"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/KhalidEchchahid/go-jpm/internal/inspectors"
	"github.com/spf13/cobra"
)

// deps_tree.go implements "jpm deps tree" which surfaces Maven's full
// dependency graph in a readable, colorized tree format.

var depsTreeCmd = &cobra.Command{
	Use:   "tree [path]",
	Short: "Display full dependency tree",
	Long:  "Display the full dependency tree for a project, including transitives when available. Works best when Maven is on PATH; otherwise falls back to local POM parsing (transitive coverage may be incomplete).",
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

		tree, err := inspector.DependencyTree(projectRoot)
		if err != nil {
			return err
		}

		fmt.Println(headerStyle("→ dependency tree:"))
		if tree == nil || tree.Root == nil {
			fmt.Println(subduedStyle("  (no dependency information)"))
			return nil
		}
		for _, warning := range tree.Warnings {
			fmt.Printf("  %s %s\n", warningIconStyle("!"), warningTextStyle(warning))
		}

		fmt.Printf("  %s\n", primaryTextStyle(tree.Root.Coordinate))
		if len(tree.Root.Children) == 0 {
			fmt.Println(subduedStyle("    (no dependencies)"))
			return nil
		}

		renderDependencyChildren(tree.Root.Children, "  ")
		return nil
	},
}

func init() {
	depsCmd.AddCommand(depsTreeCmd)
	depsTreeCmd.Flags().StringP("build-tool", "b", "maven", "Build tool: maven|gradle")
}

func renderDependencyChildren(children []*core.DependencyNode, prefix string) {
	for idx, child := range children {
		isLast := idx == len(children)-1
		connector := "├─"
		nextPrefix := prefix
		if isLast {
			connector = "└─"
			nextPrefix = prefix + "   "
		} else {
			nextPrefix = prefix + "│  "
		}

		fmt.Printf("%s%s %s\n", prefix, subduedStyle(connector), primaryTextStyle(child.Coordinate))
		if len(child.Children) > 0 {
			renderDependencyChildren(child.Children, nextPrefix)
		}
	}
}
