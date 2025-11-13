package core

// Dependency represents a declared Maven/Gradle dependency entry. It mirrors
// the common coordinates found in pom.xml and Gradle build descriptors so that
// adapters can return a unified shape to the CLI.
type Dependency struct {
	GroupID    string `yaml:"group_id"`    // Maven groupId or Gradle group coordinate.
	ArtifactID string `yaml:"artifact_id"` // Maven artifactId or Gradle name coordinate.
	Version    string // Resolved version (after property interpolation when possible).
	Type       string // Packaging / type hint (e.g. "jar", "pom").
	Scope      string // Dependency scope such as "compile" or "test".
	Classifier string // Optional classifier for shaded sources, javadoc, etc.
	Optional   bool   // Whether the dependency is marked as optional.
}

// DependencyTree captures a hierarchical view of a project's dependencies as
// produced by build tools such as Maven. The Root node represents the project
// itself, while each child node corresponds to dependencies (direct or
// transitive).
type DependencyTree struct {
	Root     *DependencyNode
	Warnings []string
}

// DependencyNode represents a single vertex within a dependency tree.
type DependencyNode struct {
	Coordinate string
	Children   []*DependencyNode
}
