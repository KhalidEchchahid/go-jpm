package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// clean.go implements "jpm clean" which removes build artifacts and cached files.

var cleanCmd = &cobra.Command{
	Use:   "clean [path]",
	Short: "Remove build artifacts and cache",
	Long: `Remove build artifacts and cached files from the project.

This command removes:
  - .jpm/out/     (compiled classes and JAR files)
  - .jpm/cache/   (downloaded dependencies cache)
  - target/       (Maven target directory if present)

Use --all to also remove the entire .jpm directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runClean,
}

func init() {
	rootCmd.AddCommand(cleanCmd)

	cleanCmd.Flags().Bool("all", false, "Remove entire .jpm directory (including maven setup)")
	cleanCmd.Flags().Bool("cache", false, "Only remove dependency cache")
	cleanCmd.Flags().Bool("dry-run", false, "Preview what would be deleted")
}

func runClean(cmd *cobra.Command, args []string) error {
	projectPath := "."
	if len(args) > 0 {
		projectPath = args[0]
	}

	projectPath = strings.TrimSpace(projectPath)
	projectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("failed to resolve project path: %w", err)
	}

	// Check if jpm.yaml exists
	manifestPath := filepath.Join(projectPath, "jpm.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return fmt.Errorf("no jpm.yaml found in %s (not a JPM project)", projectPath)
	}

	cleanAll, _ := cmd.Flags().GetBool("all")
	cacheOnly, _ := cmd.Flags().GetBool("cache")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	fmt.Println(headerStyle("→ cleaning:"))

	var dirsToRemove []string

	if cleanAll {
		// Remove entire .jpm directory
		dirsToRemove = append(dirsToRemove, filepath.Join(projectPath, ".jpm"))
	} else if cacheOnly {
		// Only remove cache
		dirsToRemove = append(dirsToRemove, filepath.Join(projectPath, ".jpm", "cache"))
	} else {
		// Default: remove build outputs and cache
		dirsToRemove = append(dirsToRemove,
			filepath.Join(projectPath, ".jpm", "out"),
			filepath.Join(projectPath, ".jpm", "cache"),
			filepath.Join(projectPath, "target"), // Maven target if present
		)
	}

	var totalSize int64
	var removedCount int

	for _, dir := range dirsToRemove {
		info, err := os.Stat(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			fmt.Printf("  warning: cannot stat %s: %v\n", dir, err)
			continue
		}

		// Calculate size
		size, _ := getDirSize(dir)
		totalSize += size

		relPath, _ := filepath.Rel(projectPath, dir)
		if relPath == "" {
			relPath = dir
		}

		if dryRun {
			fmt.Printf("  would remove %s (%s)\n", relPath, formatBytes(size))
		} else {
			if info.IsDir() {
				if err := os.RemoveAll(dir); err != nil {
					fmt.Printf("  error removing %s: %v\n", relPath, err)
					continue
				}
			} else {
				if err := os.Remove(dir); err != nil {
					fmt.Printf("  error removing %s: %v\n", relPath, err)
					continue
				}
			}
			fmt.Printf("  removed %s (%s)\n", relPath, formatBytes(size))
			removedCount++
		}
	}

	if dryRun {
		fmt.Printf("\n  dry run: would free %s\n", formatBytes(totalSize))
	} else if removedCount == 0 {
		fmt.Println("  nothing to clean")
	} else {
		fmt.Printf("\n  freed %s\n", formatBytes(totalSize))
	}

	return nil
}

// getDirSize calculates the total size of a directory
func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Ignore errors
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
