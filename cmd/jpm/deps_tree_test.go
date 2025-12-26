package main

import (
	"testing"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
)

func TestDependencyCoordinateForTree_Defaults(t *testing.T) {
	dep := core.Dependency{GroupID: "org.example", ArtifactID: "demo", Version: "1.0.0"}
	coord := dependencyCoordinateForTree(dep)
	if coord != "org.example:demo:jar:1.0.0" {
		t.Fatalf("unexpected coordinate: %s", coord)
	}
}

func TestDependencyCoordinateForTree_CustomFields(t *testing.T) {
	dep := core.Dependency{
		GroupID:    "org.example",
		ArtifactID: "demo",
		Version:    "1.0.0",
		Type:       "pom",
		Scope:      "test",
	}
	coord := dependencyCoordinateForTree(dep)
	expected := "org.example:demo:pom:1.0.0:test"
	if coord != expected {
		t.Fatalf("expected %s, got %s", expected, coord)
	}
}
