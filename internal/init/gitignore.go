package init

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GitignoreGenerator creates or updates a .gitignore file with JPM patterns.
type GitignoreGenerator struct {
	projectRoot string
}

// NewGitignoreGenerator creates a new generator for the given project root.
func NewGitignoreGenerator(projectRoot string) *GitignoreGenerator {
	return &GitignoreGenerator{projectRoot: projectRoot}
}

// jpmPatterns are the patterns to include in .gitignore.
var jpmPatterns = []string{
	"# JPM build artifacts",
	".jpm/out/",
	".jpm/logs/",
	".jpm/cache/",
	"",
	"# Maven/Gradle remnants (if any)",
	"target/",
	"",
	"# IDE/Editor metadata",
	".idea/",
	".classpath",
	".settings/",
	".project",
	"*.iml",
	".vscode/",
}

// Generate creates or updates the .gitignore file idempotently.
// Returns (created bool, error).
func (g *GitignoreGenerator) Generate() (bool, error) {
	gitignorePath := filepath.Join(g.projectRoot, ".gitignore")

	// Check if .gitignore exists
	_, err := os.Stat(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("stat .gitignore: %w", err)
	}

	if os.IsNotExist(err) {
		// Create new .gitignore
		content := strings.Join(jpmPatterns, "\n") + "\n"
		if err := os.WriteFile(gitignorePath, []byte(content), 0o644); err != nil {
			return false, fmt.Errorf("write .gitignore: %w", err)
		}
		return true, nil
	}

	// .gitignore exists; append missing patterns idempotently
	return g.appendMissingPatterns(gitignorePath)
}

// appendMissingPatterns reads existing .gitignore, appends missing JPM patterns.
func (g *GitignoreGenerator) appendMissingPatterns(path string) (bool, error) {
	// Read existing content
	content, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read .gitignore: %w", err)
	}

	existing := make(map[string]bool)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			existing[trimmed] = true
		}
	}

	// Find missing patterns
	var toAdd []string
	for _, pattern := range jpmPatterns {
		trimmed := strings.TrimSpace(pattern)
		if trimmed != "" && !existing[trimmed] {
			toAdd = append(toAdd, pattern)
		}
	}

	if len(toAdd) == 0 {
		// Nothing to add
		return false, nil
	}

	// Append missing patterns
	output := string(content)
	if !strings.HasSuffix(output, "\n") {
		output += "\n"
	}
	output += "\n" + strings.Join(toAdd, "\n") + "\n"

	if err := os.WriteFile(path, []byte(output), 0o644); err != nil {
		return false, fmt.Errorf("write .gitignore: %w", err)
	}

	return false, nil // File existed; not newly created
}
