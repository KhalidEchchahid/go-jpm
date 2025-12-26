package resolver

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Fetcher interface for retrieving POMs.
type Fetcher interface {
	FetchPOM(ctx context.Context, gav string) ([]byte, error)
}

// Resolver builds a dependency graph from manifest entries.
type Resolver struct {
	fetcher     Fetcher
	pomCache    map[string]*POMModel // GAV -> parsed POM
	nodeCache   map[string]*Node     // GAV -> resolved node
	visited     map[string]bool      // GAV -> visited
	mu          sync.RWMutex         // Protect caches
	verbose     bool
	toolVersion string
	startTime   time.Time
	maxDepth    int
}

// NewResolver creates a new dependency resolver.
func NewResolver(fetcher Fetcher, verbose bool, toolVersion string) *Resolver {
	return &Resolver{
		fetcher:     fetcher,
		pomCache:    make(map[string]*POMModel),
		nodeCache:   make(map[string]*Node),
		visited:     make(map[string]bool),
		verbose:     verbose,
		toolVersion: toolVersion,
		maxDepth:    50, // Prevent infinite recursion
	}
}

// Resolve builds a dependency graph from root nodes.
// rootDeps: initial dependencies to resolve (from manifest)
func (r *Resolver) Resolve(ctx context.Context, rootDeps []Dependency) (*Graph, error) {
	r.startTime = time.Now()
	// Reset per-resolution state. Resolver instances are reused by the CLI, and
	// these maps are meant to be per-run.
	r.mu.Lock()
	r.visited = make(map[string]bool)
	r.nodeCache = make(map[string]*Node)
	r.mu.Unlock()
	graph := NewGraph(r.toolVersion)

	// Create root node (synthetic)
	rootNode := &Node{
		GAV:      "root",
		Distance: 0,
		Resolved: true,
	}
	graph.Root = rootNode

	// BFS queue
	queue := make([]*queueEntry, 0)

	// Enqueue root dependencies
	for _, dep := range rootDeps {
		if dep.GroupID == "" || dep.ArtifactID == "" || dep.Version == "" {
			if r.verbose {
				fmt.Printf("! Skipping invalid root dependency: %+v\n", dep)
			}
			continue
		}
		entry := &queueEntry{
			GAV:      fmt.Sprintf("%s:%s:%s", dep.GroupID, dep.ArtifactID, dep.Version),
			Distance: 1,
			Scope:    DependencyScope(dep.Scope),
			Optional: dep.Optional,
			Parents:  []*Node{rootNode},
		}
		queue = append(queue, entry)
	}

	// BFS traversal
	for len(queue) > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Pop from queue
		entry := queue[0]
		queue = queue[1:]

		// Skip if already visited
		r.mu.Lock()
		if r.visited[entry.GAV] {
			r.mu.Unlock()
			continue
		}
		r.visited[entry.GAV] = true
		r.mu.Unlock()

		// Fetch and parse POM
		node, pomDeps, err := r.fetchAndParseNode(ctx, entry)
		if err != nil {
			if r.verbose {
				fmt.Printf("! Failed to fetch %s: %v\n", entry.GAV, err)
			}
			graph.UnresolvedIDs = append(graph.UnresolvedIDs, entry.GAV)
			continue
		}

		// Add to graph
		graph.AddNode(node)
		for _, parent := range entry.Parents {
			parent.AddChild(node)
		}

		// Enqueue transitive dependencies
		for _, dep := range pomDeps {
			// If version is missing, it's typically controlled by <dependencyManagement>
			// (which we don't implement yet). Skip instead of trying to fetch "...::".
			if dep.GroupID == "" || dep.ArtifactID == "" || dep.Version == "" {
				continue
			}
			transGAV := fmt.Sprintf("%s:%s:%s", dep.GroupID, dep.ArtifactID, dep.Version)

			// Skip if already visited
			r.mu.RLock()
			already := r.visited[transGAV]
			r.mu.RUnlock()
			if already {
				continue
			}

			// Check exclusions
			excluded := false
			for _, parent := range entry.Parents {
				if parent.ExcludesTransitive(node) {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}

			// Skip optional unless root declared it
			if dep.Optional {
				continue
			}

			// Skip test scope for transitive
			if dep.Scope == "test" {
				continue
			}

			// Enqueue
			newEntry := &queueEntry{
				GAV:      transGAV,
				Distance: entry.Distance + 1,
				Scope:    DependencyScope(dep.Scope),
				Optional: dep.Optional,
				Parents:  []*Node{node},
			}
			queue = append(queue, newEntry)
		}
	}

	// Finalize metadata
	graph.Metadata.Duration = time.Since(r.startTime)

	if r.verbose {
		fmt.Printf("✓ Resolution complete: %d artifacts\n", len(graph.Nodes))
	}

	return graph, nil
}

// queueEntry represents an item in the BFS queue.
type queueEntry struct {
	GAV      string
	Distance int
	Scope    DependencyScope
	Optional bool
	Parents  []*Node
}

// fetchAndParseNode fetches and parses a POM, returning a Node and its dependencies.
func (r *Resolver) fetchAndParseNode(ctx context.Context, entry *queueEntry) (*Node, []Dependency, error) {
	// Check cache
	r.mu.RLock()
	if cached, ok := r.pomCache[entry.GAV]; ok {
		r.mu.RUnlock()
		if cached.Resolved {
			node := r.gavToNode(entry.GAV, entry.Scope, entry.Optional, cached)
			return node, cached.Dependencies, nil
		}
	}
	r.mu.RUnlock()

	// Fetch POM
	pomContent, err := r.fetcher.FetchPOM(ctx, entry.GAV)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch failed: %w", err)
	}

	// Parse POM
	pom := ParsePOM(pomContent)
	if !pom.Resolved {
		return nil, nil, fmt.Errorf("parse failed: %s", pom.Error)
	}

	// Cache
	r.mu.Lock()
	r.pomCache[entry.GAV] = pom
	r.mu.Unlock()

	// Create node
	node := r.gavToNode(entry.GAV, entry.Scope, entry.Optional, pom)
	return node, pom.Dependencies, nil
}

// gavToNode converts a GAV string to a Node.
func (r *Resolver) gavToNode(gav string, scope DependencyScope, optional bool, pom *POMModel) *Node {
	parts := parseGAV(gav)
	if len(parts) != 3 {
		return nil
	}

	node := NewNode(parts[0], parts[1], parts[2], scope)
	node.Optional = optional
	node.Resolved = true
	return node
}

// parseGAV parses "group:artifact:version" into [group, artifact, version].
func parseGAV(gav string) []string {
	parts := make([]string, 0, 3)
	current := ""
	for _, ch := range gav {
		if ch == ':' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
