package native

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Cache manages the local artifact storage under .jpm/cache or a global location.
type Cache struct {
	root string
}

// NewCache creates a cache rooted at the given directory.
func NewCache(root string) *Cache {
	return &Cache{root: root}
}

// DefaultProjectCache returns a cache under <projectRoot>/.jpm/cache.
func DefaultProjectCache(projectRoot string) *Cache {
	return NewCache(filepath.Join(projectRoot, ".jpm", "cache"))
}

// ArtifactPath returns the expected local path for an artifact.
// Classifier may be empty; packaging defaults to "jar" if empty.
func (c *Cache) ArtifactPath(groupID, artifactID, version, classifier, packaging string) string {
	if packaging == "" {
		packaging = "jar"
	}
	groupPath := strings.ReplaceAll(groupID, ".", string(os.PathSeparator))
	fileName := artifactID + "-" + version
	if classifier != "" {
		fileName += "-" + classifier
	}
	fileName += "." + packaging
	return filepath.Join(c.root, groupPath, artifactID, version, fileName)
}

// POMPath returns the expected local path for a POM file.
func (c *Cache) POMPath(groupID, artifactID, version string) string {
	groupPath := strings.ReplaceAll(groupID, ".", string(os.PathSeparator))
	fileName := fmt.Sprintf("%s-%s.pom", artifactID, version)
	return filepath.Join(c.root, groupPath, artifactID, version, fileName)
}

// Has checks if the artifact exists and has non-zero size.
func (c *Cache) Has(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > 0
}

// EnsureDir creates the directory for a given artifact path.
func (c *Cache) EnsureDir(artifactPath string) error {
	return os.MkdirAll(filepath.Dir(artifactPath), 0o755)
}

// Root returns the cache root directory.
func (c *Cache) Root() string {
	return c.root
}
