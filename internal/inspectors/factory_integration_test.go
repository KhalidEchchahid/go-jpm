package inspectors

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
)

// TestFactoryMavenIntegration ensures the inspector factory can produce a
// Maven adapter that understands the real java-legacy fixtures. It doubles as a
// high-level smoke test for the Maven parsing pipeline.
func TestFactoryMavenIntegration(t *testing.T) {
	projectRoot := filepath.Join("..", "..", "java-legacy", "JPM")
	projectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		t.Fatalf("failed to resolve project root: %v", err)
	}

	factory := NewFactory()
	inspector, err := factory.ForTool(core.Maven)
	if err != nil {
		t.Fatalf("ForTool returned error: %v", err)
	}

	modules, err := inspector.ListModules(projectRoot)
	if err != nil {
		t.Fatalf("ListModules error: %v", err)
	}

	expectedModules := []string{"jpm-core", "jpm-adapter-maven", "jpm-adapter-gradle", "jpm-init", "jpm-cli"}
	if !reflect.DeepEqual(expectedModules, modules) {
		t.Fatalf("modules mismatch: expected %v, got %v", expectedModules, modules)
	}

	cliModule := filepath.Join(projectRoot, "jpm-cli")
	deps, err := inspector.ListDependencies(cliModule)
	if err != nil {
		t.Fatalf("ListDependencies error: %v", err)
	}

	expectedDeps := []core.Dependency{
		{GroupID: "com.jpm", ArtifactID: "jpm-core", Version: "0.0.1-SNAPSHOT"},
		{GroupID: "com.jpm", ArtifactID: "jpm-adapter-maven", Version: "0.0.1-SNAPSHOT"},
		{GroupID: "com.jpm", ArtifactID: "jpm-adapter-gradle", Version: "0.0.1-SNAPSHOT"},
		{GroupID: "com.jpm", ArtifactID: "jpm-init", Version: "0.0.1-SNAPSHOT"},
		{GroupID: "info.picocli", ArtifactID: "picocli", Version: "4.7.6"},
	}

	if !reflect.DeepEqual(expectedDeps, deps) {
		t.Fatalf("dependencies mismatch:\nexpected: %#v\nactual: %#v", expectedDeps, deps)
	}
}
