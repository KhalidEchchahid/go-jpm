package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const ManifestFileName = "jpm.yaml"

// LoadManifest reads jpm.yaml from the given project root.
func LoadManifest(projectRoot string) (*Manifest, string, error) {
	path := filepath.Join(projectRoot, ManifestFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, path, os.ErrNotExist
		}
		return nil, path, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, path, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, path, nil
}

// SaveManifest writes the provided manifest back to jpm.yaml.
func SaveManifest(projectRoot string, m *Manifest) (string, error) {
	if m == nil {
		return "", fmt.Errorf("nil manifest")
	}
	b, err := yaml.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("marshal manifest: %w", err)
	}
	path := filepath.Join(projectRoot, ManifestFileName)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", fmt.Errorf("write manifest: %w", err)
	}
	return path, nil
}
