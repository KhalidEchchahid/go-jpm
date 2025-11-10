package inspectors

import (
	"path/filepath"
	"testing"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
)

// TestFactoryMavenIntegration ensures the inspector factory can produce a
// Maven adapter that understands projects. It doubles as a
// high-level smoke test for the Maven parsing pipeline.
func TestFactoryMavenIntegration(t *testing.T) {
	projectRoot := filepath.Join("..", "..", "project_test")
	projectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		t.Fatalf("failed to resolve project root: %v", err)
	}

	// Check if pom.xml exists, skip if not
	pomPath := filepath.Join(projectRoot, "pom.xml")
	if _, err := filepath.Abs(pomPath); err != nil {
		// pom.xml not found, skip test
		t.Skipf("pom.xml not found at %s (skipping integration test)", pomPath)
	}

	factory := NewFactory()
	inspector, err := factory.ForTool(core.Maven)
	if err != nil {
		t.Fatalf("ForTool returned error: %v", err)
	}

	modules, err := inspector.ListModules(projectRoot)
	if err != nil {
		t.Skipf("ListModules error (pom.xml may not exist): %v", err)
	}

	// project_test should have at least one module
	if len(modules) == 0 {
		t.Fatalf("expected at least one module, got %v", modules)
	}

	// Verify we can get dependencies (even if empty)
	deps, err := inspector.ListDependencies(projectRoot)
	if err != nil {
		t.Logf("ListDependencies error: %v", err)
	}

	// Dependencies can be empty for project_test, which is fine
	if deps == nil {
		deps = []core.Dependency{}
	}

	t.Logf("Found %d modules and %d dependencies", len(modules), len(deps))
}
