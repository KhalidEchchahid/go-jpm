package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/KhalidEchchahid/go-jpm/internal/adapters/maven"
	"github.com/KhalidEchchahid/go-jpm/internal/catalog"
	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/spf13/cobra"
)

// deps_add.go implements "jpm deps add" which inserts or updates dependency
// entries in a project's build definition. The initial implementation focuses on
// Maven projects while Gradle support is tracked for a subsequent milestone.

var depsAddCmd = &cobra.Command{
	Use:               "add [group:artifact[@version]]",
	Short:             "Add a dependency to the project",
	Long:              "Add or update a dependency declaration in the target project's build definition.",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: depsAddCompletions,
	RunE:              runDepsAdd,
}

func init() {
	depsCmd.AddCommand(depsAddCmd)

	depsAddCmd.Flags().StringP("build-tool", "b", "maven", "Build tool: maven|gradle")
	depsAddCmd.Flags().String("path", ".", "Project directory containing pom.xml")
	depsAddCmd.Flags().String("version", "", "Dependency version to add")
	depsAddCmd.Flags().StringP("scope", "s", "", "Maven scope to assign (e.g., compile, test)")
	depsAddCmd.Flags().String("type", "", "Packaging type (e.g., jar, pom)")
	depsAddCmd.Flags().String("classifier", "", "Optional classifier to include")
	depsAddCmd.Flags().Bool("optional", false, "Mark the dependency as optional")
	depsAddCmd.Flags().Bool("dry-run", false, "Preview changes without modifying files")
	depsAddCmd.Flags().Bool("list-versions", false, "Show available versions for the coordinate and exit")
}

func runDepsAdd(cmd *cobra.Command, args []string) error {
	buildToolStr, _ := cmd.Flags().GetString("build-tool")
	buildTool, err := core.ParseBuildTool(buildToolStr)
	if err != nil {
		return err
	}
	if buildTool != core.Maven {
		return fmt.Errorf("build tool %s is not supported yet; only maven is currently available", buildToolStr)
	}

	coordinateInput := ""
	if len(args) > 0 {
		coordinateInput = args[0]
	}
	if coordinateInput == "" {
		return errors.New("coordinate argument is required (format: group:artifact[@version])")
	}

	groupID, artifactID, versionFromArg, err := parseCoordinate(coordinateInput)
	if err != nil {
		return err
	}

	versionFlag, _ := cmd.Flags().GetString("version")
	version := strings.TrimSpace(versionFlag)
	if version == "" {
		version = versionFromArg
	}

	scope, _ := cmd.Flags().GetString("scope")
	typ, _ := cmd.Flags().GetString("type")
	classifier, _ := cmd.Flags().GetString("classifier")
	optional, _ := cmd.Flags().GetBool("optional")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	listVersions, _ := cmd.Flags().GetBool("list-versions")

	projectPath, _ := cmd.Flags().GetString("path")
	projectPath = strings.TrimSpace(projectPath)
	if projectPath == "" {
		projectPath = "."
	}
	projectPath, err = filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("failed to resolve project path: %w", err)
	}

	dep := core.Dependency{
		GroupID:    groupID,
		ArtifactID: artifactID,
		Version:    version,
		Scope:      scope,
		Type:       typ,
		Classifier: classifier,
		Optional:   optional,
	}

	catalogService, err := catalog.NewService()
	if err != nil {
		return fmt.Errorf("failed to initialize dependency catalog: %w", err)
	}

	var meta *catalog.ArtifactMetadata
	if strings.TrimSpace(dep.Version) == "" {
		resolved, resolveErr := catalogService.ResolveArtifact(dep.GroupID, dep.ArtifactID)
		if resolveErr != nil {
			return fmt.Errorf("failed to resolve artifact version: %w", resolveErr)
		}
		meta = resolved
		dep.Version = resolved.LatestVersion
	}

	if listVersions {
		if meta == nil {
			resolved, resolveErr := catalogService.ResolveArtifact(dep.GroupID, dep.ArtifactID)
			if resolveErr != nil {
				return fmt.Errorf("failed to resolve artifact version list: %w", resolveErr)
			}
			meta = resolved
		}

		fmt.Println(headerStyle("→ available versions:"))
		if len(meta.Versions) == 0 {
			fmt.Println(subduedStyle("  (no versions found)"))
			return nil
		}
		for _, version := range meta.Versions {
			fmt.Printf("  %s\n", primaryTextStyle(version))
		}
		return nil
	}

	result, err := maven.AddDependency(projectPath, dep, maven.AddDependencyOptions{DryRun: dryRun})
	if err != nil {
		return err
	}

	printAddDependencySummary(result)
	return nil
}

func parseCoordinate(input string) (string, string, string, error) {
	parts := strings.Split(input, "@")
	if len(parts) > 2 {
		return "", "", "", fmt.Errorf("invalid coordinate %q: too many '@' segments", input)
	}

	coordinate := strings.TrimSpace(parts[0])
	version := ""
	if len(parts) == 2 {
		version = strings.TrimSpace(parts[1])
	}

	subParts := strings.Split(coordinate, ":")
	if len(subParts) != 2 {
		return "", "", "", fmt.Errorf("invalid coordinate %q: expected group:artifact", input)
	}

	groupID := strings.TrimSpace(subParts[0])
	artifactID := strings.TrimSpace(subParts[1])
	if groupID == "" || artifactID == "" {
		return "", "", "", fmt.Errorf("invalid coordinate %q: group and artifact must be non-empty", input)
	}

	return groupID, artifactID, version, nil
}

func depsAddCompletions(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	service, err := catalog.NewService()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	trimmed := strings.TrimSpace(toComplete)
	if atIndex := strings.Index(trimmed, "@"); atIndex != -1 {
		coordinateInput := trimmed
		groupID, artifactID, versionPrefix, err := parseCoordinate(coordinateInput)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		meta, err := service.ResolveArtifact(groupID, artifactID)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		coordinate := fmt.Sprintf("%s:%s", groupID, artifactID)
		return buildVersionCompletions(meta, coordinate, versionPrefix), cobra.ShellCompDirectiveNoFileComp
	}

	suggestions, err := service.SuggestArtifacts(trimmed)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return buildArtifactCompletions(suggestions), cobra.ShellCompDirectiveNoFileComp
}

const maxVersionCompletions = 20

func buildArtifactCompletions(suggestions []catalog.ArtifactSuggestion) []string {
	completions := make([]string, 0, len(suggestions))
	for _, suggestion := range suggestions {
		value := fmt.Sprintf("%s:%s", suggestion.GroupID, suggestion.ArtifactID)
		annotation := buildArtifactAnnotation(suggestion)
		completions = append(completions, formatCompletion(value, annotation))
	}
	return completions
}

func buildArtifactAnnotation(suggestion catalog.ArtifactSuggestion) string {
	parts := make([]string, 0, 2)
	if suggestion.LatestVersion != "" {
		parts = append(parts, fmt.Sprintf("latest: %s", suggestion.LatestVersion))
	}
	if suggestion.Description != "" {
		parts = append(parts, suggestion.Description)
	}
	return strings.Join(parts, " — ")
}

func buildVersionCompletions(meta *catalog.ArtifactMetadata, coordinate, versionPrefix string) []string {
	if meta == nil {
		return nil
	}

	limited := make([]string, 0, maxVersionCompletions)
	for _, version := range meta.Versions {
		if versionPrefix != "" && !strings.HasPrefix(version, versionPrefix) {
			continue
		}

		annotation := ""
		if version == meta.LatestVersion {
			annotation = "latest release"
		}

		limited = append(limited, formatCompletion(fmt.Sprintf("%s@%s", coordinate, version), annotation))
		if len(limited) >= maxVersionCompletions {
			break
		}
	}

	return limited
}

func formatCompletion(value, annotation string) string {
	if strings.TrimSpace(annotation) == "" {
		return value
	}
	return fmt.Sprintf("%s\t%s", value, annotation)
}

func printAddDependencySummary(result *maven.AddDependencyResult) {
	if result == nil {
		return
	}

	fmt.Println(headerStyle("→ dependency update:"))
	if result.Added {
		fmt.Printf("  %s %s\n", primaryTextStyle("added"), result.Coordinate)
	} else if result.Updated {
		fmt.Printf("  %s %s\n", primaryTextStyle("updated"), result.Coordinate)
	} else {
		fmt.Println(subduedStyle("  (no changes required)"))
	}

	if result.DryRun {
		fmt.Println(subduedStyle("  dry run: no files were modified"))
	} else if result.Added || result.Updated {
		fmt.Printf("  %s %s\n", primaryTextStyle("file"), result.Path)
	}
}
