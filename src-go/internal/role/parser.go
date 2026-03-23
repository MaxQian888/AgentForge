package role

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ParseFile reads and parses a YAML role manifest from a file.
func ParseFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read role file %s: %w", path, err)
	}
	return Parse(data)
}

// Parse parses a YAML role manifest from bytes.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse role manifest: %w", err)
	}
	return &m, nil
}
