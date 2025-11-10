package lockfile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/KhalidEchchahid/go-jpm/internal/engine/resolver"
	"gopkg.in/yaml.v3"
)

// Repository represents a Maven repository entry in the lockfile.
type Repository struct {
	ID  string `yaml:"id"`
	URL string `yaml:"url"`
}

// DependencyEntry represents a single locked dependency.
type DependencyEntry struct {
	GAV      string   `yaml:"gav"`
	Scope    string   `yaml:"scope"`
	Optional bool     `yaml:"optional"`
	SHA256   string   `yaml:"sha256,omitempty"`
	Size     int64    `yaml:"size,omitempty"`
	URL      string   `yaml:"url,omitempty"`
	Parents  []string `yaml:"parents,omitempty"`
}

// Lockfile represents the jpm.lock.yaml structure.
type Lockfile struct {
	LockfileFormat int               `yaml:"lockfile_format"`
	CreatedAt      string            `yaml:"created_at"`
	ToolVersion    string            `yaml:"tool_version"`
	Resolver       string            `yaml:"resolver"`
	Repositories   []Repository      `yaml:"repositories"`
	Dependencies   []DependencyEntry `yaml:"dependencies"`
}

// Writer handles lockfile generation and persistence.
type Writer struct {
	toolVersion string
}

// NewWriter creates a new lockfile writer.
func NewWriter(toolVersion string) *Writer {
	return &Writer{
		toolVersion: toolVersion,
	}
}

// Write generates and writes a lockfile from a resolved dependency graph.
func (w *Writer) Write(projectRoot string, graph *resolver.Graph, resolver string) error {
	lockfile := w.buildLockfile(graph, resolver)
	path := filepath.Join(projectRoot, "jpm.lock.yaml")

	data, err := yaml.Marshal(lockfile)
	if err != nil {
		return fmt.Errorf("failed to marshal lockfile: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write lockfile at %s: %w", path, err)
	}

	return nil
}

// buildLockfile constructs the lockfile structure from a dependency graph.
func (w *Writer) buildLockfile(graph *resolver.Graph, resolverType string) *Lockfile {
	entries := w.extractDependencies(graph)

	// Sort entries by GAV for determinism
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].GAV < entries[j].GAV
	})

	return &Lockfile{
		LockfileFormat: 1,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		ToolVersion:    w.toolVersion,
		Resolver:       resolverType,
		Repositories: []Repository{
			{
				ID:  "maven-central",
				URL: "https://repo.maven.apache.org/maven2",
			},
		},
		Dependencies: entries,
	}
}

// extractDependencies converts graph nodes to lockfile entries.
func (w *Writer) extractDependencies(graph *resolver.Graph) []DependencyEntry {
	var entries []DependencyEntry

	for _, node := range graph.Nodes {
		if !node.Resolved {
			continue // Skip unresolved nodes
		}

		// Skip test and provided scopes (not needed in lockfile)
		if node.Scope == resolver.ScopeTest || node.Scope == resolver.ScopeProvided {
			continue
		}

		entry := DependencyEntry{
			GAV:      node.GAV,
			Scope:    string(node.Scope),
			Optional: node.Optional,
			SHA256:   node.CheckSum,
		}

		// Add parent information for debugging
		for _, parent := range node.Parents {
			if parent != nil {
				entry.Parents = append(entry.Parents, parent.GAV)
			}
		}

		entries = append(entries, entry)
	}

	return entries
}
