package main

import (
	"testing"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
)

// TestFormatDependency verifies the human-readable rendering used by
// "jpm deps ls" so that CLI output stays stable as new qualifiers are added.
func TestFormatDependency(t *testing.T) {
	cases := []struct {
		name string
		dep  core.Dependency
		exp  string
	}{
		{
			name: "basic with version",
			dep:  core.Dependency{GroupID: "g", ArtifactID: "a", Version: "1.2.3"},
			exp:  "g:a:1.2.3",
		},
		{
			name: "without version",
			dep:  core.Dependency{GroupID: "g", ArtifactID: "a"},
			exp:  "g:a",
		},
		{
			name: "with qualifiers",
			dep:  core.Dependency{GroupID: "g", ArtifactID: "a", Version: "1.0", Scope: "test", Type: "pom", Classifier: "sources", Optional: true},
			exp:  "g:a:1.0 [scope=test, type=pom, classifier=sources, optional]",
		},
	}

	for _, tc := range cases {
		if got := formatDependency(tc.dep); got != tc.exp {
			t.Fatalf("%s: expected %q, got %q", tc.name, tc.exp, got)
		}
	}
}
