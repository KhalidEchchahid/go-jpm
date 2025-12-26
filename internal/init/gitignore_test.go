package init

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitignoreGenerator_CreateNew(t *testing.T) {
	tmpdir := t.TempDir()
	gen := NewGitignoreGenerator(tmpdir)

	created, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if !created {
		t.Fatalf("expected created=true")
	}

	// Verify file exists
	gitignorePath := filepath.Join(tmpdir, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}

	contentStr := string(content)
	for _, pattern := range jpmPatterns {
		if pattern != "" && !strings.Contains(contentStr, pattern) {
			t.Errorf("pattern not found: %s", pattern)
		}
	}
}

func TestGitignoreGenerator_Idempotent(t *testing.T) {
	tmpdir := t.TempDir()
	gen := NewGitignoreGenerator(tmpdir)

	// Generate first time
	created1, err := gen.Generate()
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	if !created1 {
		t.Fatalf("expected first create")
	}

	// Generate second time
	created2, err := gen.Generate()
	if err != nil {
		t.Fatalf("second generate: %v", err)
	}
	if created2 {
		t.Fatalf("expected no creation on second run")
	}

	// Verify no duplicate lines
	gitignorePath := filepath.Join(tmpdir, ".gitignore")
	content, _ := os.ReadFile(gitignorePath)
	lines := strings.Split(string(content), "\n")
	seen := make(map[string]int)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			seen[trimmed]++
		}
	}

	for line, count := range seen {
		if count > 1 {
			t.Errorf("duplicate line found %d times: %s", count, line)
		}
	}
}

func TestGitignoreGenerator_PreservesUserContent(t *testing.T) {
	tmpdir := t.TempDir()
	gitignorePath := filepath.Join(tmpdir, ".gitignore")

	// Create existing .gitignore with user content
	userContent := "# My custom patterns\n*.log\n"
	os.WriteFile(gitignorePath, []byte(userContent), 0o644)

	gen := NewGitignoreGenerator(tmpdir)
	var err error
	_, err = gen.Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	content, _ := os.ReadFile(gitignorePath)
	contentStr := string(content)
	if !strings.Contains(contentStr, "*.log") {
		t.Errorf("user content not preserved")
	}
	if !strings.Contains(contentStr, ".jpm/out/") {
		t.Errorf("JPM patterns not added")
	}
}

func TestReadmeGenerator_Create(t *testing.T) {
	tmpdir := t.TempDir()
	gen := NewReadmeGenerator(tmpdir, "my-app", "21", "Main")

	_, err := gen.Generate(false)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	readmePath := filepath.Join(tmpdir, "README.md")
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "my-app") {
		t.Errorf("project name not in README")
	}
	if !strings.Contains(contentStr, "jpm build") {
		t.Errorf("quickstart commands not in README")
	}
}

func TestReadmeGenerator_PlaceholderSubstitution(t *testing.T) {
	tmpdir := t.TempDir()
	gen := NewReadmeGenerator(tmpdir, "test-proj", "23", "TestMain")

	rendered := gen.renderTemplate()
	if !strings.Contains(rendered, "test-proj") {
		t.Errorf("project name not substituted")
	}
	// Java version is not in template; only project name and tool version
}
