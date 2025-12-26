package native

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"
)

// POM represents a minimal Maven POM structure for dependency extraction.
type POM struct {
	XMLName              xml.Name        `xml:"project"`
	GroupID              string          `xml:"groupId"`
	ArtifactID           string          `xml:"artifactId"`
	Version              string          `xml:"version"`
	Packaging            string          `xml:"packaging"`
	Parent               *POMParent      `xml:"parent"`
	Properties           POMProperties   `xml:"properties"`
	DependencyManagement *DependencyMgmt `xml:"dependencyManagement"`
	Dependencies         []POMDependency `xml:"dependencies>dependency"`
}

// POMParent represents a parent POM reference.
type POMParent struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
}

// POMProperties holds arbitrary key-value properties for interpolation.
type POMProperties struct {
	Entries map[string]string
}

// UnmarshalXML parses <properties> into a map.
func (p *POMProperties) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	p.Entries = make(map[string]string)
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch el := tok.(type) {
		case xml.StartElement:
			var value string
			if err := d.DecodeElement(&value, &el); err != nil {
				return err
			}
			p.Entries[el.Name.Local] = value
		case xml.EndElement:
			if el.Name == start.Name {
				return nil
			}
		}
	}
}

// DependencyMgmt wraps managed dependencies.
type DependencyMgmt struct {
	Dependencies []POMDependency `xml:"dependencies>dependency"`
}

// POMDependency represents a single dependency entry.
type POMDependency struct {
	GroupID    string         `xml:"groupId"`
	ArtifactID string         `xml:"artifactId"`
	Version    string         `xml:"version"`
	Scope      string         `xml:"scope"`
	Type       string         `xml:"type"`
	Classifier string         `xml:"classifier"`
	Optional   string         `xml:"optional"`
	Exclusions []POMExclusion `xml:"exclusions>exclusion"`
}

// POMExclusion represents an exclusion entry.
type POMExclusion struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
}

// IsOptional returns true if the dependency is marked optional.
func (d POMDependency) IsOptional() bool {
	return strings.EqualFold(strings.TrimSpace(d.Optional), "true")
}

// ParsePOM reads and parses a POM file from disk.
func ParsePOM(path string) (*POM, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read POM %s: %w", path, err)
	}
	return ParsePOMBytes(data)
}

// ParsePOMBytes parses POM XML content.
func ParsePOMBytes(data []byte) (*POM, error) {
	var pom POM
	if err := xml.Unmarshal(data, &pom); err != nil {
		return nil, fmt.Errorf("parse POM: %w", err)
	}
	return &pom, nil
}

// EffectiveGroupID returns the group ID, falling back to parent if empty.
func (p *POM) EffectiveGroupID() string {
	if p.GroupID != "" {
		return p.GroupID
	}
	if p.Parent != nil {
		return p.Parent.GroupID
	}
	return ""
}

// EffectiveVersion returns the version, falling back to parent if empty.
func (p *POM) EffectiveVersion() string {
	if p.Version != "" {
		return p.Version
	}
	if p.Parent != nil {
		return p.Parent.Version
	}
	return ""
}

// InterpolateVersion resolves ${project.version}, ${...} placeholders.
func (p *POM) InterpolateVersion(raw string) string {
	if raw == "" {
		return raw
	}
	// Handle ${project.version}
	raw = strings.ReplaceAll(raw, "${project.version}", p.EffectiveVersion())
	raw = strings.ReplaceAll(raw, "${pom.version}", p.EffectiveVersion())
	raw = strings.ReplaceAll(raw, "${version}", p.EffectiveVersion())

	// Handle properties
	for key, val := range p.Properties.Entries {
		placeholder := fmt.Sprintf("${%s}", key)
		raw = strings.ReplaceAll(raw, placeholder, val)
	}

	return raw
}

// ResolveDependencies returns the list of dependencies with versions interpolated
// and managed versions applied.
func (p *POM) ResolveDependencies() []POMDependency {
	managed := make(map[string]string) // "groupId:artifactId" -> version
	if p.DependencyManagement != nil {
		for _, dep := range p.DependencyManagement.Dependencies {
			key := dep.GroupID + ":" + dep.ArtifactID
			managed[key] = p.InterpolateVersion(dep.Version)
		}
	}

	result := make([]POMDependency, 0, len(p.Dependencies))
	for _, dep := range p.Dependencies {
		resolved := POMDependency{
			GroupID:    dep.GroupID,
			ArtifactID: dep.ArtifactID,
			Version:    p.InterpolateVersion(dep.Version),
			Scope:      dep.Scope,
			Type:       dep.Type,
			Classifier: dep.Classifier,
			Optional:   dep.Optional,
			Exclusions: dep.Exclusions,
		}

		// Apply managed version if dependency version is empty
		if resolved.Version == "" {
			key := resolved.GroupID + ":" + resolved.ArtifactID
			if v, ok := managed[key]; ok {
				resolved.Version = v
			}
		}

		// Default scope
		if resolved.Scope == "" {
			resolved.Scope = "compile"
		}

		result = append(result, resolved)
	}

	return result
}
