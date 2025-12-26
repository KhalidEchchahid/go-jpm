package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	projectPath, _ := cmd.Flags().GetString("path")
	projectPath = strings.TrimSpace(projectPath)
	if projectPath == "" {
		projectPath = "."
	}
	projectPath, err = filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("failed to resolve project path: %w", err)
	}

	groupID := ""
	artifactID := ""
	versionFromArg := ""
	if coordinateInput != "" {
		g, a, v, perr := parseCoordinate(coordinateInput)
		if perr != nil {
			return perr
		}
		groupID, artifactID, versionFromArg = g, a, v
	}

	versionFlag, _ := cmd.Flags().GetString("version")
	version := strings.TrimSpace(versionFlag)
	if version == "" {
		version = versionFromArg
	}

	scope, _ := cmd.Flags().GetString("scope")
	typeFlag, _ := cmd.Flags().GetString("type")
	classifier, _ := cmd.Flags().GetString("classifier")
	optional, _ := cmd.Flags().GetBool("optional")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	listVersions, _ := cmd.Flags().GetBool("list-versions")

	scopeChanged := cmd.Flags().Changed("scope")
	typeChanged := cmd.Flags().Changed("type")
	classifierChanged := cmd.Flags().Changed("classifier")
	optionalChanged := cmd.Flags().Changed("optional")

	var reader *bufio.Reader
	ensureReader := func() *bufio.Reader {
		if reader == nil {
			reader = bufio.NewReader(os.Stdin)
		}
		return reader
	}

	if strings.TrimSpace(groupID) == "" {
		groupID = strings.TrimSpace(prompt(ensureReader(), "Group ID", ""))
	}
	if strings.TrimSpace(artifactID) == "" {
		artifactID = strings.TrimSpace(prompt(ensureReader(), "Artifact ID", ""))
	}
	if groupID == "" || artifactID == "" {
		return errors.New("groupId and artifactId are required")
	}

	if listVersions {
		service, svcErr := catalog.NewService()
		if svcErr != nil {
			return fmt.Errorf("failed to initialize dependency catalog: %w", svcErr)
		}
		meta, resolveErr := service.ResolveArtifact(groupID, artifactID)
		if resolveErr != nil {
			return fmt.Errorf("failed to resolve artifact version list: %w", resolveErr)
		}
		fmt.Println(headerStyle("→ available versions:"))
		if len(meta.Versions) == 0 {
			fmt.Println(subduedStyle("  (no versions found)"))
			return nil
		}
		for _, v := range meta.Versions {
			fmt.Printf("  %s\n", primaryTextStyle(v))
		}
		return nil
	}

	if strings.TrimSpace(version) == "" {
		version = strings.TrimSpace(prompt(ensureReader(), "Version", "latest"))
	}
	if version == "" {
		return errors.New("version is required")
	}

	manifest, _, err := core.LoadManifest(projectPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no %s found in %s (run 'jpm init' first)", core.ManifestFileName, projectPath)
		}
		return err
	}

	if strings.EqualFold(version, "latest") {
		service, svcErr := catalog.NewService()
		if svcErr != nil {
			return fmt.Errorf("failed to initialize dependency catalog: %w", svcErr)
		}
		meta, resolveErr := service.ResolveArtifact(groupID, artifactID)
		if resolveErr != nil {
			return fmt.Errorf("failed to resolve artifact version: %w", resolveErr)
		}
		version = meta.LatestVersion
	}

	scope = strings.TrimSpace(scope)
	typeFlag = strings.TrimSpace(typeFlag)
	classifier = strings.TrimSpace(classifier)

	idx := -1
	for i := range manifest.Dependencies {
		dep := manifest.Dependencies[i]
		if dep.GroupID == groupID && dep.ArtifactID == artifactID {
			idx = i
			break
		}
	}

	var entry core.Dependency
	if idx >= 0 {
		entry = manifest.Dependencies[idx]
	} else {
		entry.GroupID = groupID
		entry.ArtifactID = artifactID
	}
	entry.Version = version
	if idx < 0 || scopeChanged {
		entry.Scope = scope
	}
	if idx < 0 || typeChanged {
		entry.Type = typeFlag
	}
	if idx < 0 || classifierChanged {
		entry.Classifier = classifier
	}
	if idx < 0 || optionalChanged {
		entry.Optional = optional
	}

	if idx >= 0 && dependenciesEqual(manifest.Dependencies[idx], entry) {
		fmt.Println(headerStyle("→ dependency update:"))
		fmt.Println(subduedStyle("  (no changes required)"))
		if dryRun {
			fmt.Println(subduedStyle("  dry run: jpm.yaml was not modified"))
		}
		return nil
	}

	if dryRun {
		fmt.Println(headerStyle("→ dependency update:"))
		action := "added"
		if idx >= 0 {
			action = "updated"
		}
		fmt.Printf("  %s %s\n", primaryTextStyle(action), formatDependency(entry))
		fmt.Println(subduedStyle("  dry run: jpm.yaml was not modified"))
		return nil
	}

	if idx >= 0 {
		manifest.Dependencies[idx] = entry
	} else {
		manifest.Dependencies = append(manifest.Dependencies, entry)
	}

	savedPath, err := core.SaveManifest(projectPath, manifest)
	if err != nil {
		return err
	}

	fmt.Println(headerStyle("→ dependency update:"))
	if idx >= 0 {
		fmt.Printf("  %s %s\n", primaryTextStyle("updated"), formatDependency(entry))
	} else {
		fmt.Printf("  %s %s\n", primaryTextStyle("added"), formatDependency(entry))
	}
	fmt.Printf("  %s %s\n", primaryTextStyle("file"), savedPath)
	return nil
}

func prompt(r *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	text, _ := r.ReadString('\n')
	text = strings.TrimSpace(text)
	if text == "" {
		return def
	}
	return text
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

func dependenciesEqual(a, b core.Dependency) bool {
	return a.GroupID == b.GroupID &&
		a.ArtifactID == b.ArtifactID &&
		a.Version == b.Version &&
		a.Scope == b.Scope &&
		a.Type == b.Type &&
		a.Classifier == b.Classifier &&
		a.Optional == b.Optional
}
