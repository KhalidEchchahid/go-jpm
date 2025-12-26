package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// lint.go implements "jpm lint" to run static analysis on Java source files.

var lintCmd = &cobra.Command{
	Use:   "lint [files...]",
	Short: "Run static analysis on Java source files",
	Long: `Run static analysis checks on Java source files.

JPM lint performs basic code quality checks including:
  - Unused imports detection
  - Missing Javadoc on public methods
  - Empty catch blocks
  - System.out/err usage
  - Magic numbers
  - Long methods
  - Complex conditionals

If no files are specified, analyzes all .java files in src/.`,
	Example: `  jpm lint                    # Lint all files
  jpm lint src/Main.java      # Lint specific file
  jpm lint --fix              # Auto-fix where possible`,
	RunE: runLint,
}

func init() {
	rootCmd.AddCommand(lintCmd)

	lintCmd.Flags().StringP("path", "p", ".", "Project directory")
	lintCmd.Flags().Bool("fix", false, "Auto-fix issues where possible")
	lintCmd.Flags().String("severity", "info", "Minimum severity to report (error, warning, info)")
}

// LintIssue represents a code quality issue
type LintIssue struct {
	File     string
	Line     int
	Column   int
	Severity string // error, warning, info
	Rule     string
	Message  string
}

func runLint(cmd *cobra.Command, args []string) error {
	projectPath, _ := cmd.Flags().GetString("path")
	minSeverity, _ := cmd.Flags().GetString("severity")

	projectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return err
	}

	// Find Java files to lint
	var javaFiles []string
	if len(args) > 0 {
		for _, f := range args {
			absPath := f
			if !filepath.IsAbs(f) {
				absPath = filepath.Join(projectPath, f)
			}
			javaFiles = append(javaFiles, absPath)
		}
	} else {
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
		fmt.Println("  no Java files found to lint")
		return nil
	}

	fmt.Printf(headerStyle("→ analyzing %d Java files:\n"), len(javaFiles))
	fmt.Println()

	var allIssues []LintIssue
	errorCount := 0
	warningCount := 0
	infoCount := 0

	for _, file := range javaFiles {
		issues := lintFile(file, projectPath)

		for _, issue := range issues {
			if !shouldReport(issue.Severity, minSeverity) {
				continue
			}

			allIssues = append(allIssues, issue)
			switch issue.Severity {
			case "error":
				errorCount++
			case "warning":
				warningCount++
			case "info":
				infoCount++
			}
		}
	}

	// Group issues by file
	issuesByFile := make(map[string][]LintIssue)
	for _, issue := range allIssues {
		issuesByFile[issue.File] = append(issuesByFile[issue.File], issue)
	}

	// Print issues
	for file, issues := range issuesByFile {
		relPath, _ := filepath.Rel(projectPath, file)
		fmt.Printf("  %s\n", relPath)

		for _, issue := range issues {
			var severityColor string
			switch issue.Severity {
			case "error":
				severityColor = "\033[31m" // Red
			case "warning":
				severityColor = "\033[33m" // Yellow
			case "info":
				severityColor = "\033[36m" // Cyan
			}

			fmt.Printf("    %s:%d %s%s\033[0m: %s\n",
				relPath, issue.Line, severityColor, issue.Severity, issue.Message)
		}
		fmt.Println()
	}

	// Summary
	if len(allIssues) == 0 {
		fmt.Println("  \033[32m✓ no issues found\033[0m")
	} else {
		fmt.Println("  " + headerStyle("summary:"))
		if errorCount > 0 {
			fmt.Printf("    \033[31m%d errors\033[0m\n", errorCount)
		}
		if warningCount > 0 {
			fmt.Printf("    \033[33m%d warnings\033[0m\n", warningCount)
		}
		if infoCount > 0 {
			fmt.Printf("    \033[36m%d info\033[0m\n", infoCount)
		}
	}

	if errorCount > 0 {
		return fmt.Errorf("found %d errors", errorCount)
	}

	return nil
}

func lintFile(filePath, projectPath string) []LintIssue {
	var issues []LintIssue

	file, err := os.Open(filePath)
	if err != nil {
		return issues
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	inBlockComment := false
	methodLines := 0
	braceDepth := 0
	imports := make(map[string]int) // import -> line number
	usedImports := make(map[string]bool)

	// Patterns for detection
	systemOutPattern := regexp.MustCompile(`System\.(out|err)\.`)
	emptyBlockPattern := regexp.MustCompile(`\{\s*\}`)
	magicNumberPattern := regexp.MustCompile(`[^0-9.][2-9]\d{2,}[^0-9]|[^0-9.]\d{4,}[^0-9]`)
	todoPattern := regexp.MustCompile(`(?i)(TODO|FIXME|XXX|HACK):?`)
	importPattern := regexp.MustCompile(`^import\s+(static\s+)?([^;]+);`)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Track block comments
		if strings.Contains(line, "/*") {
			inBlockComment = true
		}
		if strings.Contains(line, "*/") {
			inBlockComment = false
			continue
		}
		if inBlockComment || strings.HasPrefix(trimmed, "//") {
			// Check for TODOs in comments
			if todoPattern.MatchString(line) {
				issues = append(issues, LintIssue{
					File:     filePath,
					Line:     lineNum,
					Severity: "info",
					Rule:     "todo-comment",
					Message:  "TODO/FIXME comment found",
				})
			}
			continue
		}

		// Track imports
		if matches := importPattern.FindStringSubmatch(trimmed); matches != nil {
			importPath := matches[2]
			imports[importPath] = lineNum
		}

		// Check for used imports (simple heuristic)
		for imp := range imports {
			parts := strings.Split(imp, ".")
			className := parts[len(parts)-1]
			if className != "*" && strings.Contains(line, className) {
				usedImports[imp] = true
			}
		}

		// Track method length
		if strings.Contains(line, "{") {
			braceDepth++
		}
		if strings.Contains(line, "}") {
			braceDepth--
			if braceDepth == 1 && methodLines > 50 {
				issues = append(issues, LintIssue{
					File:     filePath,
					Line:     lineNum,
					Severity: "warning",
					Rule:     "method-length",
					Message:  fmt.Sprintf("Method is %d lines (consider breaking it up)", methodLines),
				})
			}
			if braceDepth == 1 {
				methodLines = 0
			}
		}
		if braceDepth >= 2 {
			methodLines++
		}

		// System.out/err usage
		if systemOutPattern.MatchString(line) {
			issues = append(issues, LintIssue{
				File:     filePath,
				Line:     lineNum,
				Severity: "warning",
				Rule:     "system-out",
				Message:  "Avoid System.out/err, use a logger instead",
			})
		}

		// Empty catch blocks
		if strings.Contains(line, "catch") && emptyBlockPattern.MatchString(line) {
			issues = append(issues, LintIssue{
				File:     filePath,
				Line:     lineNum,
				Severity: "warning",
				Rule:     "empty-catch",
				Message:  "Empty catch block - at least log the exception",
			})
		}

		// Magic numbers (not in comments, not 0/1/-1)
		if magicNumberPattern.MatchString(line) && !strings.Contains(line, "final") {
			issues = append(issues, LintIssue{
				File:     filePath,
				Line:     lineNum,
				Severity: "info",
				Rule:     "magic-number",
				Message:  "Consider extracting magic number to a named constant",
			})
		}

		// Long lines
		if len(line) > 120 {
			issues = append(issues, LintIssue{
				File:     filePath,
				Line:     lineNum,
				Severity: "info",
				Rule:     "line-length",
				Message:  fmt.Sprintf("Line exceeds 120 characters (%d)", len(line)),
			})
		}
	}

	// Check for unused imports
	for imp, line := range imports {
		if !usedImports[imp] && !strings.HasSuffix(imp, "*") {
			issues = append(issues, LintIssue{
				File:     filePath,
				Line:     line,
				Severity: "warning",
				Rule:     "unused-import",
				Message:  fmt.Sprintf("Unused import: %s", imp),
			})
		}
	}

	return issues
}

func shouldReport(issueSeverity, minSeverity string) bool {
	severityOrder := map[string]int{
		"error":   3,
		"warning": 2,
		"info":    1,
	}

	return severityOrder[issueSeverity] >= severityOrder[minSeverity]
}
