package maven

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
)

func TestAddDependency_AddsNewEntry(t *testing.T) {
	tmpDir := t.TempDir()
	writePomFile(t, tmpDir, `<project>
	<modelVersion>4.0.0</modelVersion>
</project>`)

	dep := core.Dependency{
		GroupID:    "org.example",
		ArtifactID: "demo",
		Version:    "1.2.3",
	}

	result, err := AddDependency(tmpDir, dep, AddDependencyOptions{})
	if err != nil {
		t.Fatalf("AddDependency returned error: %v", err)
	}

	if !result.Added || result.Updated {
		t.Fatalf("expected dependency to be added, got added=%v updated=%v", result.Added, result.Updated)
	}

	content := readPom(t, tmpDir)
	if !strings.Contains(content, "<dependencies>") {
		t.Fatalf("dependencies block was not created: %s", content)
	}
	if !strings.Contains(content, "<groupId>org.example</groupId>") ||
		!strings.Contains(content, "<artifactId>demo</artifactId>") ||
		!strings.Contains(content, "<version>1.2.3</version>") {
		t.Fatalf("dependency entry not found in pom: %s", content)
	}
}

func TestAddDependency_UpdatesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	writePomFile(t, tmpDir, `<project>
	<modelVersion>4.0.0</modelVersion>
	<dependencies>
		<dependency>
			<groupId>org.example</groupId>
			<artifactId>demo</artifactId>
			<version>1.0.0</version>
		</dependency>
	</dependencies>
</project>`)

	dep := core.Dependency{
		GroupID:    "org.example",
		ArtifactID: "demo",
		Version:    "2.0.0",
	}

	result, err := AddDependency(tmpDir, dep, AddDependencyOptions{})
	if err != nil {
		t.Fatalf("AddDependency returned error: %v", err)
	}

	if result.Added || !result.Updated {
		t.Fatalf("expected dependency to be updated, got added=%v updated=%v", result.Added, result.Updated)
	}

	content := readPom(t, tmpDir)
	if !strings.Contains(content, "<version>2.0.0</version>") {
		t.Fatalf("dependency version was not updated: %s", content)
	}
}

func TestAddDependency_DryRunDoesNotModifyFile(t *testing.T) {
	tmpDir := t.TempDir()
	original := `<project>
	<modelVersion>4.0.0</modelVersion>
</project>`
	writePomFile(t, tmpDir, original)

	dep := core.Dependency{
		GroupID:    "org.example",
		ArtifactID: "demo",
		Version:    "1.0.0",
	}

	result, err := AddDependency(tmpDir, dep, AddDependencyOptions{DryRun: true})
	if err != nil {
		t.Fatalf("AddDependency returned error: %v", err)
	}

	content := readPom(t, tmpDir)
	if content != original {
		t.Fatalf("expected pom.xml to remain unchanged during dry-run")
	}

	if result.After == "" || !strings.Contains(result.After, "<version>1.0.0</version>") {
		t.Fatalf("dry-run result should include planned dependency entry")
	}
}

func writePomFile(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, "pom.xml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write pom.xml fixture: %v", err)
	}
}

func readPom(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "pom.xml")
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read pom.xml: %v", err)
	}
	return string(bytes)
}
