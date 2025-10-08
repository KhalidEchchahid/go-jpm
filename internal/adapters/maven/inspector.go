package maven

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MavenProjectInspector implements ProjectInspector for Maven projects
type MavenProjectInspector struct{}

// NewMavenProjectInspector creates a new Maven project inspector
func NewMavenProjectInspector() *MavenProjectInspector {
	return &MavenProjectInspector{}
}

// ListModules returns a list of module names from the pom.xml
func (i *MavenProjectInspector) ListModules(projectRoot string) ([]string, error) {
	pomPath := filepath.Join(projectRoot, "pom.xml")

	// Check if pom.xml exists
	if _, err := os.Stat(pomPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("pom.xml not found at: %s", pomPath)
	}

	// Read the pom.xml file
	content, err := os.ReadFile(pomPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pom.xml: %w", err)
	}

	// Remove XML comments
	xmlWithoutComments := regexp.MustCompile(`(?s)<!--.*?-->`).ReplaceAllString(string(content), "")

	// Find modules block
	modulesBlockRegex := regexp.MustCompile(`(?is)<modules>(.*?)</modules>`)
	block := modulesBlockRegex.FindStringSubmatch(xmlWithoutComments)
	if len(block) < 2 {
		return []string{}, nil // No modules found
	}

	// Find individual module tags
	moduleRegex := regexp.MustCompile(`(?is)<module>(.*?)</module>`)
	matches := moduleRegex.FindAllStringSubmatch(block[1], -1)

	var modules []string
	for _, match := range matches {
		if len(match) >= 2 {
			module := strings.TrimSpace(match[1])
			if module != "" {
				modules = append(modules, module)
			}
		}
	}

	return modules, nil
}