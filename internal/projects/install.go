package projects

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"espflasher/internal/archive"
)

// InstallFromURL downloads a project from rawURL into
// projectsDir/<name>/. The download may be a plain file (.py/.bin) or
// an archive (zip/rar/7z/gz/xz/tar...). When the result has no
// project.json, a manifest is generated from the files found.
// Returns the project directory.
func InstallFromURL(rawURL, projectsDir string, progress func(done, total int64), log func(string)) (string, error) {
	tmp, err := os.MkdirTemp("", "espproj")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	path, err := archive.DownloadFile(rawURL, tmp, progress)
	if err != nil {
		return "", err
	}
	base := filepath.Base(path)
	log("Downloaded: " + base)

	name := sanitizeName(archive.StripExt(base))
	dest := filepath.Join(projectsDir, name)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", fmt.Errorf("cannot create directory %s: %w", dest, err)
	}

	if archive.IsArchive(base) {
		log("Extracting archive...")
		if err := archive.Extract(path, dest, log); err != nil {
			return "", err
		}
		if err := flattenSingleDir(dest); err != nil {
			return "", err
		}
	} else {
		if err := copyFile(path, filepath.Join(dest, base)); err != nil {
			return "", err
		}
	}

	if err := ensureManifest(dest, name, log); err != nil {
		return "", err
	}
	return dest, nil
}

// sanitizeName makes a safe directory name out of a filename.
func sanitizeName(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', ' ':
			return '-'
		}
		return r
	}, name)
	if name == "" || name == "." {
		name = "project"
	}
	return name
}

// flattenSingleDir moves contents up when the extraction produced a
// single top-level directory (typical for GitHub zipballs).
func flattenSingleDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return nil
	}
	inner := filepath.Join(dir, entries[0].Name())
	children, err := os.ReadDir(inner)
	if err != nil {
		return err
	}
	for _, c := range children {
		if err := os.Rename(filepath.Join(inner, c.Name()), filepath.Join(dir, c.Name())); err != nil {
			return err
		}
	}
	return os.Remove(inner)
}

// ensureManifest writes project.json when missing: MicroPython project
// when .py files exist, bin project when a .bin exists.
func ensureManifest(dir, name string, log func(string)) error {
	manifest := filepath.Join(dir, "project.json")
	if _, err := os.Stat(manifest); err == nil {
		log("project.json found in download.")
		return nil
	}

	var pys, bins []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		switch {
		case strings.HasSuffix(lower, ".py"):
			pys = append(pys, e.Name())
		case strings.HasSuffix(lower, ".bin"):
			bins = append(bins, e.Name())
		}
	}
	sort.Strings(pys)
	sort.Strings(bins)

	p := Project{Name: name, Description: "Imported from URL"}
	switch {
	case len(pys) > 0:
		p.Type = "micropython"
		p.Files = pys
		for _, f := range pys {
			if strings.EqualFold(f, "main.py") {
				p.Main = f
				break
			}
		}
	case len(bins) > 0:
		p.Type = "bin"
		p.Bin = bins[0]
		p.FlashOffset = "0x10000"
	default:
		return fmt.Errorf("no flashable files (.py / .bin) in %s", dir)
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(manifest, data, 0o644); err != nil {
		return err
	}
	log("Generated project.json (" + p.Type + ").")
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}
