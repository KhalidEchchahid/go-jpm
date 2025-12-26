package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/KhalidEchchahid/go-jpm/internal/engine/native"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// package.go implements "jpm package" which creates distributable JARs.

var packageCmd = &cobra.Command{
	Use:   "package [path]",
	Short: "Package project for distribution",
	Long: `Package the project into a distributable JAR file.

Packaging options:
  - Standard JAR: Contains only project classes
  - Uber JAR (fat JAR): Includes all dependencies in a single JAR
  - With sources: Include source files in the JAR`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPackage,
}

func init() {
	rootCmd.AddCommand(packageCmd)

	packageCmd.Flags().Bool("uber", false, "Create uber JAR with all dependencies")
	packageCmd.Flags().Bool("sources", false, "Include source files")
	packageCmd.Flags().StringP("output", "o", "", "Output JAR path (default: .jpm/out/<artifact>-<version>.jar)")
}

func runPackage(cmd *cobra.Command, args []string) error {
	projectPath := "."
	if len(args) > 0 {
		projectPath = args[0]
	}

	projectPath = strings.TrimSpace(projectPath)
	projectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("failed to resolve project path: %w", err)
	}

	uber, _ := cmd.Flags().GetBool("uber")
	includeSources, _ := cmd.Flags().GetBool("sources")
	outputPath, _ := cmd.Flags().GetString("output")

	// Load manifest
	manifestPath := filepath.Join(projectPath, "jpm.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("no jpm.yaml found in %s (run 'jpm init')", projectPath)
	}

	var manifest core.Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse jpm.yaml: %w", err)
	}

	// First build the project
	fmt.Println(headerStyle("→ building project:"))

	// Use native engine to build
	eng := native.New()
	result, err := eng.Build(projectPath, &manifest)
	if err != nil {
		return fmt.Errorf("build failed: %w\n%s", err, result.Logs)
	}

	fmt.Println("  ✓ build complete")

	// Determine output path
	jarName := fmt.Sprintf("%s-%s.jar", manifest.Project.ArtifactID, manifest.Project.Version)
	if uber {
		jarName = fmt.Sprintf("%s-%s-uber.jar", manifest.Project.ArtifactID, manifest.Project.Version)
	}

	if outputPath == "" {
		outputPath = filepath.Join(projectPath, ".jpm", "out", jarName)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if uber {
		return createUberJar(projectPath, outputPath, manifest, includeSources)
	}

	return createStandardJar(projectPath, outputPath, manifest, includeSources)
}

func createStandardJar(projectPath, outputPath string, manifest core.Manifest, includeSources bool) error {
	fmt.Println(headerStyle("→ packaging standard JAR:"))

	classesDir := filepath.Join(projectPath, ".jpm", "tmp", "classes")
	srcDir := filepath.Join(projectPath, "src")

	// Create JAR file
	jarFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create JAR file: %w", err)
	}
	defer jarFile.Close()

	zipWriter := zip.NewWriter(jarFile)
	defer zipWriter.Close()

	// Add manifest
	manifestContent := fmt.Sprintf("Manifest-Version: 1.0\nMain-Class: %s\nCreated-By: JPM\n",
		manifest.App.MainClass)

	mfWriter, err := zipWriter.Create("META-INF/MANIFEST.MF")
	if err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}
	mfWriter.Write([]byte(manifestContent))

	// Add class files
	classCount := 0
	err = filepath.Walk(classesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(classesDir, path)
		relPath = strings.ReplaceAll(relPath, string(os.PathSeparator), "/")

		writer, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		writer.Write(data)
		classCount++
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to add class files: %w", err)
	}

	fmt.Printf("  added %d class files\n", classCount)

	// Add source files if requested
	if includeSources {
		sourceCount := 0
		err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".java") {
				return nil
			}

			relPath, _ := filepath.Rel(srcDir, path)
			relPath = strings.ReplaceAll(relPath, string(os.PathSeparator), "/")

			writer, err := zipWriter.Create(relPath)
			if err != nil {
				return err
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			writer.Write(data)
			sourceCount++
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to add source files: %w", err)
		}
		fmt.Printf("  added %d source files\n", sourceCount)
	}

	// Get file size
	jarInfo, _ := os.Stat(outputPath)
	fmt.Printf("\n  "+headerStyle("✓ packaged:")+" %s (%s)\n", filepath.Base(outputPath), formatBytes(jarInfo.Size()))

	return nil
}

func createUberJar(projectPath, outputPath string, manifest core.Manifest, includeSources bool) error {
	fmt.Println(headerStyle("→ packaging uber JAR:"))

	classesDir := filepath.Join(projectPath, ".jpm", "tmp", "classes")
	cacheDir := filepath.Join(projectPath, ".jpm", "cache")
	srcDir := filepath.Join(projectPath, "src")

	// Create JAR file
	jarFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create JAR file: %w", err)
	}
	defer jarFile.Close()

	zipWriter := zip.NewWriter(jarFile)
	defer zipWriter.Close()

	// Track added entries to avoid duplicates
	addedEntries := make(map[string]bool)

	// Add manifest
	manifestContent := fmt.Sprintf("Manifest-Version: 1.0\nMain-Class: %s\nCreated-By: JPM (Uber JAR)\n",
		manifest.App.MainClass)

	mfWriter, err := zipWriter.Create("META-INF/MANIFEST.MF")
	if err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}
	mfWriter.Write([]byte(manifestContent))
	addedEntries["META-INF/MANIFEST.MF"] = true

	// Add project class files
	classCount := 0
	err = filepath.Walk(classesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(classesDir, path)
		relPath = strings.ReplaceAll(relPath, string(os.PathSeparator), "/")

		if addedEntries[relPath] {
			return nil
		}

		writer, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		writer.Write(data)
		addedEntries[relPath] = true
		classCount++
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to add class files: %w", err)
	}
	fmt.Printf("  added %d project classes\n", classCount)

	// Resolve and add dependency JARs
	cache := native.NewCache(cacheDir)
	downloader := native.NewDownloader("", cache)
	resolver := native.NewResolver(downloader, cache)

	coreDeps := make([]core.Dependency, 0)
	for _, d := range manifest.Dependencies {
		if d.Scope != "test" && d.Scope != "provided" {
			coreDeps = append(coreDeps, core.Dependency{
				GroupID:    d.GroupID,
				ArtifactID: d.ArtifactID,
				Version:    d.Version,
				Scope:      d.Scope,
			})
		}
	}

	resolved, err := resolver.Resolve(coreDeps)
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	depCount := 0
	depClassCount := 0
	for _, dep := range resolved {
		if dep.LocalPath == "" || dep.Scope == "test" || dep.Scope == "provided" {
			continue
		}

		// Extract classes from dependency JAR
		count, err := extractJarToZip(dep.LocalPath, zipWriter, addedEntries)
		if err != nil {
			fmt.Printf("  warning: failed to process %s: %v\n", dep.Artifact.Coordinate(), err)
			continue
		}
		depClassCount += count
		depCount++
	}
	fmt.Printf("  added %d classes from %d dependencies\n", depClassCount, depCount)

	// Add source files if requested
	if includeSources {
		sourceCount := 0
		err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".java") {
				return nil
			}

			relPath, _ := filepath.Rel(srcDir, path)
			relPath = strings.ReplaceAll(relPath, string(os.PathSeparator), "/")

			if addedEntries[relPath] {
				return nil
			}

			writer, err := zipWriter.Create(relPath)
			if err != nil {
				return err
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			writer.Write(data)
			addedEntries[relPath] = true
			sourceCount++
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to add source files: %w", err)
		}
		fmt.Printf("  added %d source files\n", sourceCount)
	}

	// Get file size
	zipWriter.Close()
	jarFile.Close()
	jarInfo, _ := os.Stat(outputPath)
	fmt.Printf("\n  "+headerStyle("✓ uber JAR created:")+" %s (%s)\n", filepath.Base(outputPath), formatBytes(jarInfo.Size()))

	return nil
}

// extractJarToZip extracts a JAR file's contents into the zip writer
func extractJarToZip(jarPath string, zipWriter *zip.Writer, addedEntries map[string]bool) (int, error) {
	reader, err := zip.OpenReader(jarPath)
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	count := 0
	for _, file := range reader.File {
		// Skip directories and META-INF (except services)
		if file.FileInfo().IsDir() {
			continue
		}
		if strings.HasPrefix(file.Name, "META-INF/") && !strings.HasPrefix(file.Name, "META-INF/services/") {
			continue
		}

		// Skip if already added
		if addedEntries[file.Name] {
			continue
		}

		// Copy file to new JAR
		rc, err := file.Open()
		if err != nil {
			continue
		}

		writer, err := zipWriter.Create(file.Name)
		if err != nil {
			rc.Close()
			continue
		}

		io.Copy(writer, rc)
		rc.Close()

		addedEntries[file.Name] = true
		count++
	}

	return count, nil
}
