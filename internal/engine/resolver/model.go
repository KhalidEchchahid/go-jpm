package resolver

import (
	"fmt"
	"time"
)

// Graph represents the complete resolved dependency tree.
type Graph struct {
	Root          *Node            // Synthetic root or first dependency
	Nodes         map[string]*Node // All unique GAVs (key: "group:artifact:version")
	Metadata      *ResolutionMeta
	UnresolvedIDs []string // Missing/failed artifacts (partial resolution)
}

// Node represents a single artifact in the graph.
type Node struct {
	GAV        string // "groupId:artifactId:version"
	GroupID    string
	ArtifactID string
	Version    string
	Scope      DependencyScope
	Optional   bool
	Classifier string

	// Graph structure
	Parents  []*Node // Nodes that depend on this
	Children []*Node // Direct dependencies
	Distance int     // Hops from root (0 = root)

	// Metadata
	Exclusions []ExclusionRule // e.g., "log4j:*"
	Resolved   bool            // Successfully fetched & parsed
	CheckSum   string          // SHA-256 from lockfile (if available)

	// Conflict resolution
	Conflict *ConflictInfo // Other versions competed
}

// DependencyScope defines the lifetime of a dependency.
type DependencyScope string

const (
	ScopeCompile  DependencyScope = "compile"
	ScopeRuntime  DependencyScope = "runtime"
	ScopeTest     DependencyScope = "test"
	ScopeProvided DependencyScope = "provided"
	ScopeOptional DependencyScope = "optional"
)

// ExclusionRule represents a pattern to exclude transitive deps.
type ExclusionRule struct {
	GroupID    string // e.g., "org.slf4j"
	ArtifactID string // e.g., "*" (wildcard) or "slf4j-core"
}

// ConflictInfo tracks the loser in a version conflict.
type ConflictInfo struct {
	CandidateVersion  string
	CandidateDistance int
	Reason            string // "nearest-wins"
}

// ResolutionMeta stores metadata about the resolution process.
type ResolutionMeta struct {
	Timestamp        time.Time
	Duration         time.Duration
	Strategy         string // "nearest-wins"
	ToolVersion      string
	RepoCount        int
	ArtifactsFetched int
}

// NewGraph creates a new dependency graph.
func NewGraph(toolVersion string) *Graph {
	return &Graph{
		Root:  nil,
		Nodes: make(map[string]*Node),
		Metadata: &ResolutionMeta{
			Timestamp:   time.Now().UTC(),
			Strategy:    "nearest-wins",
			ToolVersion: toolVersion,
		},
		UnresolvedIDs: []string{},
	}
}

// AddNode adds a node to the graph or updates if exists.
func (g *Graph) AddNode(node *Node) {
	if existing, ok := g.Nodes[node.GAV]; ok {
		// Node exists; merge info
		existing.Resolved = existing.Resolved || node.Resolved
		existing.CheckSum = node.CheckSum
		return
	}
	g.Nodes[node.GAV] = node
}

// GetNode retrieves a node by GAV.
func (g *Graph) GetNode(gav string) *Node {
	return g.Nodes[gav]
}

// String returns a debug representation of the graph.
func (g *Graph) String() string {
	return fmt.Sprintf("Graph{nodes=%d, resolved=%d, unresolved=%d}",
		len(g.Nodes),
		g.resolvedCount(),
		len(g.UnresolvedIDs))
}

func (g *Graph) resolvedCount() int {
	count := 0
	for _, node := range g.Nodes {
		if node.Resolved {
			count++
		}
	}
	return count
}

// ToClasspath builds a classpath string from all resolved dependencies.
// Returns a slice of absolute paths to JAR files in the format expected by javac -cp.
func (g *Graph) ToClasspath() ([]string, error) {
	var classpath []string
	for _, node := range g.Nodes {
		if node.Resolved && node.Scope != ScopeTest && node.Scope != ScopeProvided {
			// For now, construct path based on standard Maven cache layout
			// Format: ~/.m2/repository/{group}/{artifact}/{version}/{artifact}-{version}.jar
			// This will be replaced when we implement actual fetcher integration
			jarPath := fmt.Sprintf("%s:%s:%s", node.GroupID, node.ArtifactID, node.Version)
			classpath = append(classpath, jarPath)
		}
	}
	return classpath, nil
}

// NewNode creates a new dependency node.
func NewNode(groupID, artifactID, version string, scope DependencyScope) *Node {
	return &Node{
		GAV:        fmt.Sprintf("%s:%s:%s", groupID, artifactID, version),
		GroupID:    groupID,
		ArtifactID: artifactID,
		Version:    version,
		Scope:      scope,
		Optional:   false,
		Classifier: "",
		Parents:    []*Node{},
		Children:   []*Node{},
		Distance:   0,
		Exclusions: []ExclusionRule{},
		Resolved:   false,
	}
}

// AddChild adds a child dependency to this node.
func (n *Node) AddChild(child *Node) {
	for _, existing := range n.Children {
		if existing.GAV == child.GAV {
			return // Already present
		}
	}
	n.Children = append(n.Children, child)
	child.Parents = append(child.Parents, n)
}

// ExcludesTransitive checks if this node excludes a transitive dependency.
func (n *Node) ExcludesTransitive(dep *Node) bool {
	for _, rule := range n.Exclusions {
		if matchesExclusionRule(rule, dep) {
			return true
		}
	}
	return false
}

// matchesExclusionRule checks if a dependency matches an exclusion pattern.
func matchesExclusionRule(rule ExclusionRule, dep *Node) bool {
	// Group ID must match exactly or rule is "*"
	if rule.GroupID != "*" && rule.GroupID != dep.GroupID {
		return false
	}
	// Artifact ID must match exactly or rule is "*"
	if rule.ArtifactID != "*" && rule.ArtifactID != dep.ArtifactID {
		return false
	}
	return true
}

// String returns debug representation of a node.
func (n *Node) String() string {
	return fmt.Sprintf("%s (scope=%s, optional=%v, distance=%d, resolved=%v)",
		n.GAV, n.Scope, n.Optional, n.Distance, n.Resolved)
}
