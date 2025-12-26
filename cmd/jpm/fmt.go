package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// fmt.go implements "jpm fmt" to format Java source files.

var fmtCmd = &cobra.Command{
	Use:   "fmt [files...]",
	Short: "Format Java source files",
	Long: `Format Java source files using Google Java Format.

If no files are specified, formats all .java files in src/.

The formatter downloads google-java-format automatically if not present.`,
	Example: `  jpm fmt                    # Format all files
  jpm fmt src/Main.java      # Format specific file
  jpm fmt --check            # Check without modifying (CI mode)
  jpm fmt --dry-run          # Show what would be changed`,
	RunE: runFmt,
}

func init() {
	rootCmd.AddCommand(fmtCmd)

	fmtCmd.Flags().StringP("path", "p", ".", "Project directory")
	fmtCmd.Flags().Bool("check", false, "Check formatting without modifying files (exit 1 if unformatted)")
	fmtCmd.Flags().Bool("dry-run", false, "Show what would be changed without modifying files")
}

const googleJavaFormatVersion = "1.25.2"

func runFmt(cmd *cobra.Command, args []string) error {
	projectPath, _ := cmd.Flags().GetString("path")
	checkOnly, _ := cmd.Flags().GetBool("check")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	projectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return err
	}

	// Ensure google-java-format is available
	formatterPath, err := ensureGoogleJavaFormat(projectPath)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	// Find Java files to format
	var javaFiles []string
	if len(args) > 0 {
		// Use specified files
		for _, f := range args {
			absPath := f
			if !filepath.IsAbs(f) {
				absPath = filepath.Join(projectPath, f)
			}
			javaFiles = append(javaFiles, absPath)
		}
	} else {
		// Find all .java files in src/
		srcDir := filepath.Join(projectPath, "src")
		err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && strings.HasSuffix(path, ".java") {
				javaFiles = append(javaFiles, path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to find Java files: %w", err)
		}
	}

	if len(javaFiles) == 0 {
		fmt.Println("  no Java files found to format")
		return nil
	}

	fmt.Printf(headerStyle("→ formatting %d Java files:\n"), len(javaFiles))

	formattedCount := 0
	unchangedCount := 0
	unformattedFiles := []string{}

	for _, file := range javaFiles {
		relPath, _ := filepath.Rel(projectPath, file)

		// Read original content
		original, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("  error reading %s: %v\n", relPath, err)
			continue
		}

		// Run formatter
		formatted, err := formatFile(formatterPath, file)
		if err != nil {
			fmt.Printf("  error formatting %s: %v\n", relPath, err)
			continue
		}

		// Compare
		if bytes.Equal(original, formatted) {
			unchangedCount++
			if !checkOnly && !dryRun {
				fmt.Printf("  unchanged: %s\n", relPath)
			}
		} else {
			if checkOnly {
				unformattedFiles = append(unformattedFiles, relPath)
			} else if dryRun {
				fmt.Printf("  would format: %s\n", relPath)
				formattedCount++
			} else {
				// Write formatted content
				if err := os.WriteFile(file, formatted, 0644); err != nil {
					fmt.Printf("  error writing %s: %v\n", relPath, err)
					continue
				}
				fmt.Printf("  formatted: %s\n", relPath)
				formattedCount++
			}
		}
	}

	fmt.Println()
	if checkOnly {
		if len(unformattedFiles) > 0 {
			fmt.Printf("  %d files need formatting:\n", len(unformattedFiles))
			for _, f := range unformattedFiles {
				fmt.Printf("    • %s\n", f)
			}
			return fmt.Errorf("found %d unformatted files", len(unformattedFiles))
		}
		fmt.Println("  ✓ all files are properly formatted")
	} else if dryRun {
		fmt.Printf("  would format %d files, %d already formatted\n", formattedCount, unchangedCount)
	} else {
		fmt.Printf("  %d files formatted, %d unchanged\n", formattedCount, unchangedCount)
	}

	return nil
}

func ensureGoogleJavaFormat(projectPath string) (string, error) {
	// Check in .jpm/tools/
	toolsDir := filepath.Join(projectPath, ".jpm", "tools")
	jarName := fmt.Sprintf("google-java-format-%s-all-deps.jar", googleJavaFormatVersion)
	jarPath := filepath.Join(toolsDir, jarName)

	if _, err := os.Stat(jarPath); err == nil {
		return jarPath, nil
	}

	// Download
	fmt.Println("  downloading google-java-format...")

	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		return "", err
	}

	url := fmt.Sprintf(
		"https://github.com/google/google-java-format/releases/download/v%s/%s",
		googleJavaFormatVersion, jarName,
	)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	out, err := os.Create(jarPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	fmt.Printf("  downloaded %s\n", jarName)
	return jarPath, nil
}

func formatFile(formatterPath, filePath string) ([]byte, error) {
	// Need --add-exports for newer Java versions
	cmd := exec.Command("java",
		"--add-exports", "jdk.compiler/com.sun.tools.javac.api=ALL-UNNAMED",
		"--add-exports", "jdk.compiler/com.sun.tools.javac.file=ALL-UNNAMED",
		"--add-exports", "jdk.compiler/com.sun.tools.javac.parser=ALL-UNNAMED",
		"--add-exports", "jdk.compiler/com.sun.tools.javac.tree=ALL-UNNAMED",
		"--add-exports", "jdk.compiler/com.sun.tools.javac.util=ALL-UNNAMED",
		"-jar", formatterPath, filePath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Check if it's a Java version issue
		if strings.Contains(stderr.String(), "NoSuchMethodError") {
			return nil, fmt.Errorf("google-java-format requires Java 17-23 (detected incompatible version)")
		}
		return nil, fmt.Errorf("%s", stderr.String())
	}
	return stdout.Bytes(), nil
}
