package resolver

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// MockFetcher mocks the Fetcher interface for testing.
type MockFetcher struct {
	poms map[string][]byte
}

func NewMockFetcher() *MockFetcher {
	return &MockFetcher{poms: make(map[string][]byte)}
}

func (m *MockFetcher) AddPOM(gav, xml string) {
	m.poms[gav] = []byte(xml)
}

func (m *MockFetcher) FetchPOM(ctx context.Context, gav string) ([]byte, error) {
	if pom, ok := m.poms[gav]; ok {
		return pom, nil
	}
	return nil, fmt.Errorf("not found: %s", gav)
}

func TestModel_NewNode(t *testing.T) {
	node := NewNode("org.slf4j", "slf4j-api", "2.0.16", ScopeCompile)
	if node.GAV != "org.slf4j:slf4j-api:2.0.16" {
		t.Errorf("GAV mismatch: %s", node.GAV)
	}
	if node.Scope != ScopeCompile {
		t.Errorf("Scope mismatch: %s", node.Scope)
	}
	if node.Distance != 0 {
		t.Errorf("Distance should be 0")
	}
}

func TestModel_ExcludesTransitive(t *testing.T) {
	node := NewNode("org.example", "lib", "1.0", ScopeCompile)
	node.Exclusions = []ExclusionRule{
		{GroupID: "org.slf4j", ArtifactID: "*"},
	}

	depSlf4j := NewNode("org.slf4j", "slf4j-api", "2.0", ScopeCompile)
	depOther := NewNode("com.google", "guava", "33", ScopeCompile)

	if !node.ExcludesTransitive(depSlf4j) {
		t.Errorf("Should exclude slf4j")
	}
	if node.ExcludesTransitive(depOther) {
		t.Errorf("Should not exclude guava")
	}
}

func TestPOMParser_Simple(t *testing.T) {
	xml := `<?xml version="1.0"?>
<project>
  <groupId>org.slf4j</groupId>
  <artifactId>slf4j-api</artifactId>
  <version>2.0.16</version>
  <packaging>jar</packaging>
</project>`

	pom := ParsePOM([]byte(xml))
	if !pom.Resolved {
		t.Errorf("POM should resolve")
	}
	if pom.GroupID != "org.slf4j" {
		t.Errorf("GroupID mismatch: %s", pom.GroupID)
	}
	if pom.ArtifactID != "slf4j-api" {
		t.Errorf("ArtifactID mismatch: %s", pom.ArtifactID)
	}
	if pom.Version != "2.0.16" {
		t.Errorf("Version mismatch: %s", pom.Version)
	}
}

func TestPOMParser_WithDependencies(t *testing.T) {
	xml := `<?xml version="1.0"?>
<project>
  <groupId>org.example</groupId>
  <artifactId>myapp</artifactId>
  <version>1.0</version>
  <dependencies>
    <dependency>
      <groupId>org.slf4j</groupId>
      <artifactId>slf4j-api</artifactId>
      <version>2.0.16</version>
      <scope>compile</scope>
    </dependency>
    <dependency>
      <groupId>junit</groupId>
      <artifactId>junit</artifactId>
      <version>4.13</version>
      <scope>test</scope>
    </dependency>
  </dependencies>
</project>`

	pom := ParsePOM([]byte(xml))
	if len(pom.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(pom.Dependencies))
	}

	if pom.Dependencies[0].GroupID != "org.slf4j" {
		t.Errorf("First dep groupID mismatch")
	}
	if pom.Dependencies[0].Scope != "compile" {
		t.Errorf("First dep scope mismatch")
	}

	if pom.Dependencies[1].Scope != "test" {
		t.Errorf("Second dep scope mismatch")
	}
}

func TestPOMParser_WithExclusions(t *testing.T) {
	xml := `<?xml version="1.0"?>
<project>
  <groupId>org.example</groupId>
  <artifactId>app</artifactId>
  <version>1.0</version>
  <dependencies>
    <dependency>
      <groupId>com.google.guava</groupId>
      <artifactId>guava</artifactId>
      <version>33.0</version>
      <exclusions>
        <exclusion>
          <groupId>org.checkerframework</groupId>
          <artifactId>*</artifactId>
        </exclusion>
      </exclusions>
    </dependency>
  </dependencies>
</project>`

	pom := ParsePOM([]byte(xml))
	if len(pom.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency")
	}

	if len(pom.Dependencies[0].Exclusions) != 1 {
		t.Errorf("Expected 1 exclusion")
	}

	exc := pom.Dependencies[0].Exclusions[0]
	if exc.GroupID != "org.checkerframework" {
		t.Errorf("Exclusion groupID mismatch")
	}
	if exc.ArtifactID != "*" {
		t.Errorf("Exclusion artifactID should be *")
	}
}

func TestPOMParser_PropertyInterpolation(t *testing.T) {
	xml := `<?xml version="1.0"?>
<project>
  <groupId>junit</groupId>
  <artifactId>junit</artifactId>
  <version>4.13.2</version>
  <properties>
    <hamcrestVersion>1.3</hamcrestVersion>
  </properties>
  <dependencies>
    <dependency>
      <groupId>org.hamcrest</groupId>
      <artifactId>hamcrest-core</artifactId>
      <version>${hamcrestVersion}</version>
      <scope>test</scope>
    </dependency>
  </dependencies>
</project>`

	pom := ParsePOM([]byte(xml))
	if !pom.Resolved {
		t.Fatalf("POM should resolve: %s", pom.Error)
	}
	if got := pom.Properties["hamcrestVersion"]; got != "1.3" {
		t.Fatalf("expected properties.hamcrestVersion=1.3, got %q", got)
	}
	if len(pom.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(pom.Dependencies))
	}
	if got := pom.Dependencies[0].Version; got != "1.3" {
		t.Fatalf("expected interpolated dependency version 1.3, got %q", got)
	}
}

func TestResolver_SimpleGraph(t *testing.T) {
	fetcher := NewMockFetcher()

	// Simple POM with no dependencies
	fetcher.AddPOM("org.slf4j:slf4j-api:2.0.16", `<?xml version="1.0"?>
<project>
  <groupId>org.slf4j</groupId>
  <artifactId>slf4j-api</artifactId>
  <version>2.0.16</version>
</project>`)

	resolver := NewResolver(fetcher, false, "0.0.354")
	deps := []Dependency{
		{
			GroupID:    "org.slf4j",
			ArtifactID: "slf4j-api",
			Version:    "2.0.16",
			Scope:      "compile",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	graph, err := resolver.Resolve(ctx, deps)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if graph == nil {
		t.Fatalf("Graph is nil")
	}

	if len(graph.Nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(graph.Nodes))
	}

	node := graph.GetNode("org.slf4j:slf4j-api:2.0.16")
	if node == nil {
		t.Errorf("Node not found in graph")
	}
}

func TestResolver_TransitiveDeps(t *testing.T) {
	fetcher := NewMockFetcher()

	// App depends on guava, guava depends on failureaccess
	fetcher.AddPOM("com.google.guava:guava:33.0.0", `<?xml version="1.0"?>
<project>
  <groupId>com.google.guava</groupId>
  <artifactId>guava</artifactId>
  <version>33.0.0</version>
  <dependencies>
    <dependency>
      <groupId>com.google.failureaccess</groupId>
      <artifactId>failure-access</artifactId>
      <version>1.0.1</version>
      <scope>compile</scope>
    </dependency>
  </dependencies>
</project>`)

	fetcher.AddPOM("com.google.failureaccess:failure-access:1.0.1", `<?xml version="1.0"?>
<project>
  <groupId>com.google.failureaccess</groupId>
  <artifactId>failure-access</artifactId>
  <version>1.0.1</version>
</project>`)

	resolver := NewResolver(fetcher, false, "0.0.354")
	deps := []Dependency{
		{
			GroupID:    "com.google.guava",
			ArtifactID: "guava",
			Version:    "33.0.0",
			Scope:      "compile",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	graph, err := resolver.Resolve(ctx, deps)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// Should have both guava and failure-access
	if len(graph.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(graph.Nodes))
	}

	guava := graph.GetNode("com.google.guava:guava:33.0.0")
	if guava == nil {
		t.Errorf("Guava not found")
	}

	failureAccess := graph.GetNode("com.google.failureaccess:failure-access:1.0.1")
	if failureAccess == nil {
		t.Errorf("Failure-access not found")
	}
}

func TestResolveGAV_Parsing(t *testing.T) {
	tests := []struct {
		gav       string
		wantParts []string
	}{
		{"org.slf4j:slf4j-api:2.0.16", []string{"org.slf4j", "slf4j-api", "2.0.16"}},
		{"com.google.guava:guava:33.0.0", []string{"com.google.guava", "guava", "33.0.0"}},
	}

	for _, tt := range tests {
		t.Run(tt.gav, func(t *testing.T) {
			parts := parseGAV(tt.gav)
			if len(parts) != len(tt.wantParts) {
				t.Errorf("parseGAV() got %d parts, want %d", len(parts), len(tt.wantParts))
			}
			for i, part := range parts {
				if part != tt.wantParts[i] {
					t.Errorf("parseGAV()[%d] = %s, want %s", i, part, tt.wantParts[i])
				}
			}
		})
	}
}
