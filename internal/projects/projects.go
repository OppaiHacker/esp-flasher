// Package projects reads project.json manifests from the projects/ directory.
package projects

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Project is the project manifest from projects/<name>/project.json.
type Project struct {
	Dir         string   `json:"-"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"` // "micropython" lub "bin"
	Chips       []string `json:"chips"`
	Main        string   `json:"main"`
	Files       []string `json:"files"`
	Bin         string   `json:"bin"`
	FlashOffset string   `json:"flash_offset"`
}

// Scan reads all projects from the directory.
func Scan(dir string) ([]Project, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", dir, err)
	}
	var out []Project
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifest := filepath.Join(dir, e.Name(), "project.json")
		data, err := os.ReadFile(manifest)
		if err != nil {
			continue // folder without manifest - skip
		}
		var p Project
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		p.Dir = filepath.Join(dir, e.Name())
		if p.Name == "" {
			p.Name = e.Name()
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	return out, nil
}

// SupportsChip checks if the project declares support for the given chip.
// An empty chips list means no restrictions.
func (p Project) SupportsChip(chip string) bool {
	if len(p.Chips) == 0 || chip == "" {
		return true
	}
	for _, c := range p.Chips {
		if c == chip {
			return true
		}
	}
	return false
}

// FilePaths returns absolute paths to project files to be flashed.
func (p Project) FilePaths() []string {
	var out []string
	for _, f := range p.Files {
		out = append(out, filepath.Join(p.Dir, f))
	}
	return out
}
