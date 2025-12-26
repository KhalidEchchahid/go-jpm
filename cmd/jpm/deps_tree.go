package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

		manifest, _, mErr := core.LoadManifest(projectRoot)
		var tree *core.DependencyTree
		var warnings []string

		if mErr == nil && manifest != nil {
			if manifest.Project.GroupID == "" || manifest.Project.ArtifactID == "" {
				return fmt.Errorf("manifest is missing project.group_id or project.artifact_id")
			}
			rootCoord := fmt.Sprintf("%s:%s:%s", manifest.Project.GroupID, manifest.Project.ArtifactID, manifest.Project.Version)
			node := &core.DependencyNode{Coordinate: rootCoord}
			deps := make([]core.Dependency, len(manifest.Dependencies))
			copy(deps, manifest.Dependencies)
			for _, dep := range deps {
				child := &core.DependencyNode{Coordinate: dependencyCoordinateForTree(dep)}
				node.Children = append(node.Children, child)
			}
			tree = &core.DependencyTree{Root: node, Warnings: []string{"displaying direct dependencies from manifest (transitives require build sync)"}}
			warnings = tree.Warnings
		} else {
			if mErr != nil && !errors.Is(mErr, os.ErrNotExist) {
				return mErr
			}
			factory := inspectors.NewFactory()
			inspector, err := factory.ForTool(buildTool)
			if err != nil {
				return err
			}
			tree, err = inspector.DependencyTree(projectRoot)
			if err != nil {
				return err
			}
		}

		fmt.Println(headerStyle("→ dependency tree:"))
		if tree == nil || tree.Root == nil {
			fmt.Println(subduedStyle("  (no dependency information)"))
			return nil
		}
		for _, warning := range warnings {
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

func dependencyCoordinateForTree(dep core.Dependency) string {
	parts := []string{strings.TrimSpace(dep.GroupID), strings.TrimSpace(dep.ArtifactID)}
	packaging := strings.TrimSpace(dep.Type)
	if packaging == "" {
		packaging = "jar"
	}
	parts = append(parts, packaging)
	version := strings.TrimSpace(dep.Version)
	if version == "" {
		version = "unspecified"
	}
	parts = append(parts, version)
	coord := strings.Join(parts, ":")
	if dep.Scope != "" {
		coord = fmt.Sprintf("%s:%s", coord, dep.Scope)
	}
	return coord
}
