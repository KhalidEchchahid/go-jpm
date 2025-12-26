package resolver

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"

	"golang.org/x/net/html/charset"
)

// POMModel represents parsed POM metadata.
type POMModel struct {
	GroupID      string
	ArtifactID   string
	Version      string
	Packaging    string
	Parent       *GAV
	Dependencies []Dependency
	Properties   map[string]string
	Resolved     bool
	Error        string
}

// GAV represents a Maven coordinate (Group:Artifact:Version).
type GAV struct {
	Group    string
	Artifact string
	Version  string
}

// String returns the GAV in "group:artifact:version" format.
func (g *GAV) String() string {
	return fmt.Sprintf("%s:%s:%s", g.Group, g.Artifact, g.Version)
}

// Dependency represents a POM dependency entry.
type Dependency struct {
	GroupID    string
	ArtifactID string
	Version    string
	Scope      string // compile, test, provided, optional, runtime
	Optional   bool
	Classifier string
	Type       string
	Exclusions []ExclusionRule
}

// pomXML is the internal XML structure for unmarshalling.
type pomXML struct {
	XMLName      xml.Name   `xml:"project"`
	GroupID      string     `xml:"groupId"`
	ArtifactID   string     `xml:"artifactId"`
	Version      string     `xml:"version"`
	Packaging    string     `xml:"packaging"`
	Parent       *pomParent `xml:"parent"`
	Dependencies []pomDep   `xml:"dependencies>dependency"`
	Properties   *pomProperties `xml:"properties"`
}

// pomProperties captures arbitrary <properties> children.
// We keep it permissive so we can pull values like <hamcrestVersion>1.3</hamcrestVersion>.
type pomProperties struct {
	Entries []NameValue `xml:",any"`
}

// NameValue is a helper for decoding arbitrary <key>value</key> pairs.
// encoding/xml doesn't ship a generic type for this, so we provide one.
type NameValue struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type pomParent struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
}

type pomDep struct {
	GroupID    string       `xml:"groupId"`
	ArtifactID string       `xml:"artifactId"`
	Version    string       `xml:"version"`
	Scope      string       `xml:"scope"`
	Optional   string       `xml:"optional"`
	Classifier string       `xml:"classifier"`
	Type       string       `xml:"type"`
	Exclusions []pomExclude `xml:"exclusions>exclusion"`
}

type pomExclude struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
}

// ParsePOM parses POM XML content into a POMModel.
func ParsePOM(content []byte) *POMModel {
	model := &POMModel{
		Resolved:   false,
		Properties: make(map[string]string),
	}

	var pom pomXML
	dec := xml.NewDecoder(bytes.NewReader(content))
	dec.CharsetReader = func(enc string, input io.Reader) (io.Reader, error) {
		return charset.NewReaderLabel(enc, input)
	}
	if err := dec.Decode(&pom); err != nil {
		model.Error = fmt.Sprintf("failed to parse POM XML: %v", err)
		return model
	}

	// Extract basic info
	model.GroupID = pom.GroupID
	model.ArtifactID = pom.ArtifactID
	model.Version = pom.Version
	model.Packaging = pom.Packaging
	if model.Packaging == "" {
		model.Packaging = "jar"
	}

	// Extract parent (if present)
	if pom.Parent != nil {
		model.Parent = &GAV{
			Group:    pom.Parent.GroupID,
			Artifact: pom.Parent.ArtifactID,
			Version:  pom.Parent.Version,
		}
	}

	// Extract properties
	model.Properties = make(map[string]string)
	// common implicit properties
	if model.GroupID != "" {
		model.Properties["project.groupId"] = model.GroupID
		model.Properties["pom.groupId"] = model.GroupID
	}
	if model.ArtifactID != "" {
		model.Properties["project.artifactId"] = model.ArtifactID
		model.Properties["pom.artifactId"] = model.ArtifactID
	}
	if model.Version != "" {
		model.Properties["project.version"] = model.Version
		model.Properties["pom.version"] = model.Version
		model.Properties["version"] = model.Version // commonly referenced shorthand
	}
	// raw properties from <properties>
	if pom.Properties != nil {
		for _, entry := range pom.Properties.Entries {
			key := strings.TrimSpace(entry.XMLName.Local)
			val := strings.TrimSpace(entry.Value)
			if key == "" || val == "" {
				continue
			}
			model.Properties[key] = val
		}
	}

	// Extract dependencies
	for _, dep := range pom.Dependencies {
		model.Dependencies = append(model.Dependencies, Dependency{
			GroupID:    dep.GroupID,
			ArtifactID: dep.ArtifactID,
			Version:    substituteProperties(dep.Version, model.Properties),
			Scope:      dep.Scope,
			Optional:   dep.Optional == "true",
			Classifier: dep.Classifier,
			Type:       dep.Type,
			Exclusions: parsePOMExclusions(dep.Exclusions),
		})
	}

	model.Resolved = true
	return model
}

// parsePOMExclusions converts POM exclusion elements to ExclusionRule slice.
func parsePOMExclusions(exclusions []pomExclude) []ExclusionRule {
	rules := make([]ExclusionRule, 0, len(exclusions))
	for _, ex := range exclusions {
		rule := ExclusionRule{
			GroupID:    ex.GroupID,
			ArtifactID: ex.ArtifactID,
		}
		if rule.ArtifactID == "" {
			rule.ArtifactID = "*"
		}
		rules = append(rules, rule)
	}
	return rules
}

// substituteProperties replaces ${property} with values from the map.
func substituteProperties(text string, props map[string]string) string {
	if text == "" {
		return text
	}

	// Replace ${property.name} patterns
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		key := match[2 : len(match)-1] // Extract "property.name" from "${property.name}"
		if val, ok := props[key]; ok {
			return val
		}
		// Fallback: keep original if not found
		return match
	})
}

// String returns debug representation of POMModel.
func (p *POMModel) String() string {
	return fmt.Sprintf("POM{%s:%s:%s, deps=%d, resolved=%v, err=%q}",
		p.GroupID, p.ArtifactID, p.Version, len(p.Dependencies), p.Resolved, p.Error)
}
