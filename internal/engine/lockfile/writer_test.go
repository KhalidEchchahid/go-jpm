package lockfile

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KhalidEchchahid/go-jpm/internal/engine/resolver"
	"gopkg.in/yaml.v3"
)

func TestWriter_Write(t *testing.T) {
	// Create a test graph
	graph := resolver.NewGraph("0.0.1")
	node := resolver.NewNode("org.example", "my-lib", "1.0.0", resolver.ScopeCompile)
	node.Resolved = true
	node.CheckSum = "abcd1234"
	graph.AddNode(node)

	// Create temp directory
	tmpDir := t.TempDir()

	// Write lockfile
	writer := NewWriter("0.0.1")
	err := writer.Write(tmpDir, graph, "native")
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify file exists
	lockPath := filepath.Join(tmpDir, "jpm.lock.yaml")
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lockfile not created: %v", err)
	}

	// Parse and verify content
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("failed to read lockfile: %v", err)
	}

	var lf Lockfile
	err = yaml.Unmarshal(data, &lf)
	if err != nil {
		t.Fatalf("failed to parse lockfile: %v", err)
	}

	// Verify structure
	if lf.LockfileFormat != 1 {
		t.Errorf("expected format 1, got %d", lf.LockfileFormat)
	}
	if lf.Resolver != "native" {
		t.Errorf("expected resolver 'native', got %s", lf.Resolver)
	}
	if lf.ToolVersion != "0.0.1" {
		t.Errorf("expected tool version '0.0.1', got %s", lf.ToolVersion)
	}
	if len(lf.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(lf.Dependencies))
	}
	if lf.Dependencies[0].GAV != "org.example:my-lib:1.0.0" {
		t.Errorf("unexpected GAV: %s", lf.Dependencies[0].GAV)
	}
}

func TestWriter_Determinism(t *testing.T) {
	// Create a graph with multiple dependencies
	graph := resolver.NewGraph("0.0.1")

	// Add in reverse order to test sorting
	deps := []struct {
		group    string
		artifact string
		version  string
	}{
		{"zebra", "last", "1.0.0"},
		{"apple", "first", "1.0.0"},
		{"middle", "middle", "1.0.0"},
	}

	for _, dep := range deps {
		node := resolver.NewNode(dep.group, dep.artifact, dep.version, resolver.ScopeCompile)
		node.Resolved = true
		graph.AddNode(node)
	}

	tmpDir := t.TempDir()
	writer := NewWriter("0.0.1")
	err := writer.Write(tmpDir, graph, "native")
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read and verify sorted order
	lockPath := filepath.Join(tmpDir, "jpm.lock.yaml")
	data, _ := os.ReadFile(lockPath)

	var lf Lockfile
	yaml.Unmarshal(data, &lf)

	// Verify sorted by GAV
	for i := 0; i < len(lf.Dependencies)-1; i++ {
		if lf.Dependencies[i].GAV > lf.Dependencies[i+1].GAV {
			t.Errorf("dependencies not sorted: %s > %s",
				lf.Dependencies[i].GAV, lf.Dependencies[i+1].GAV)
		}
	}
}

func TestWriter_ExcludesTestScope(t *testing.T) {
	graph := resolver.NewGraph("0.0.1")

	// Add compile and test scoped deps
	compile := resolver.NewNode("org.example", "compile-lib", "1.0.0", resolver.ScopeCompile)
	compile.Resolved = true
	graph.AddNode(compile)

	test := resolver.NewNode("junit", "junit", "4.13.2", resolver.ScopeTest)
	test.Resolved = true
	graph.AddNode(test)

	tmpDir := t.TempDir()
	writer := NewWriter("0.0.1")
	writer.Write(tmpDir, graph, "native")

	lockPath := filepath.Join(tmpDir, "jpm.lock.yaml")
	data, _ := os.ReadFile(lockPath)

	var lf Lockfile
	yaml.Unmarshal(data, &lf)

	// Only compile scope should be in lockfile
	if len(lf.Dependencies) != 1 {
		t.Errorf("expected 1 dependency (compile), got %d", len(lf.Dependencies))
	}
	if lf.Dependencies[0].GAV != "org.example:compile-lib:1.0.0" {
		t.Errorf("unexpected dependency: %s", lf.Dependencies[0].GAV)
	}
}

func TestWriter_CreatedAtFormat(t *testing.T) {
	graph := resolver.NewGraph("0.0.1")
	tmpDir := t.TempDir()

	before := time.Now().UTC()
	writer := NewWriter("0.0.1")
	writer.Write(tmpDir, graph, "native")
	after := time.Now().UTC()

	lockPath := filepath.Join(tmpDir, "jpm.lock.yaml")
	data, _ := os.ReadFile(lockPath)

	var lf Lockfile
	yaml.Unmarshal(data, &lf)

	// Parse created_at
	createdAt, err := time.Parse(time.RFC3339, lf.CreatedAt)
	if err != nil {
		t.Fatalf("invalid time format: %v", err)
	}

	// Verify it's within expected range (with 2-second buffer for processing time)
	if createdAt.Before(before.Add(-1*time.Second)) || createdAt.After(after.Add(2*time.Second)) {
		t.Errorf("timestamp out of range: %v (expected between %v and %v)",
			createdAt, before.Add(-1*time.Second), after.Add(2*time.Second))
	}
}

func TestBuildLockfile_RepositoriesIncluded(t *testing.T) {
	graph := resolver.NewGraph("0.0.1")
	writer := NewWriter("0.0.1")
	lf := writer.buildLockfile(graph, "native")

	if len(lf.Repositories) == 0 {
		t.Fatal("no repositories in lockfile")
	}

	central := lf.Repositories[0]
	if central.ID != "maven-central" {
		t.Errorf("expected maven-central, got %s", central.ID)
	}
	if central.URL != "https://repo.maven.apache.org/maven2" {
		t.Errorf("unexpected repository URL: %s", central.URL)
	}
}
