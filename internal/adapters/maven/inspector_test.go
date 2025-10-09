package maven

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
)

// TestMavenInspectorParsesModulesAndDependencies validates the XML streaming
// parser against an in-memory pom snippet to catch regressions in module or
// dependency extraction logic.
func TestMavenInspectorParsesModulesAndDependencies(t *testing.T) {
	dir := t.TempDir()
	pom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modules>
    <module>api</module>
    <module>cli</module>
  </modules>
  <dependencies>
    <dependency>
      <groupId>org.slf4j</groupId>
      <artifactId>slf4j-api</artifactId>
      <version>2.0.7</version>
      <scope>compile</scope>
    </dependency>
    <dependency>
      <groupId>com.fasterxml.jackson.core</groupId>
      <artifactId>jackson-databind</artifactId>
      <version>${jackson.version}</version>
      <optional>true</optional>
    </dependency>
  </dependencies>
</project>`

	writePom(t, dir, pom)

	inspector := NewMavenProjectInspector()

	modules, err := inspector.ListModules(dir)
	if err != nil {
		t.Fatalf("ListModules returned error: %v", err)
	}
	if !reflect.DeepEqual([]string{"api", "cli"}, modules) {
		t.Fatalf("modules mismatch: got %v", modules)
	}

	deps, err := inspector.ListDependencies(dir)
	if err != nil {
		t.Fatalf("ListDependencies returned error: %v", err)
	}

	expected := []core.Dependency{
		{GroupID: "org.slf4j", ArtifactID: "slf4j-api", Version: "2.0.7", Scope: "compile"},
		{GroupID: "com.fasterxml.jackson.core", ArtifactID: "jackson-databind", Version: "${jackson.version}", Optional: true},
	}

	if !reflect.DeepEqual(expected, deps) {
		t.Fatalf("dependencies mismatch:\nexpected: %#v\nactual: %#v", expected, deps)
	}
}

func writePom(t *testing.T, dir, contents string) {
	t.Helper()
	path := filepath.Join(dir, "pom.xml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write pom: %v", err)
	}
}
