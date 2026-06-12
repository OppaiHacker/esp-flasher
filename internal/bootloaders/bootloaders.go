// Package bootloaders manages the bootloaders/ directory: listing
// local .bin files, downloading bootloaders from a URL (plain file or
// archive) and searching GitHub for bootloader releases.
package bootloaders

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"espflasher/internal/archive"
)

// Bootloader is a local .bin file found under the bootloaders/ directory.
type Bootloader struct {
	Path  string // absolute path
	Rel   string // path relative to bootloaders/ (shown in the UI)
	SizeH string
}

// Scan walks dir recursively and returns all .bin files sorted by path.
func Scan(dir string) ([]Bootloader, error) {
	var out []Bootloader
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".bin") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil // file disappeared — skip
		}
		rel, _ := filepath.Rel(dir, p)
		out = append(out, Bootloader{Path: p, Rel: rel, SizeH: humanSize(info.Size())})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", dir, err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out, nil
}

// Offset returns the bootloader flash offset for the chip family.
// ESP32 and ESP32-S2 boot from 0x1000; newer chips and ESP8266 from 0x0.
func Offset(chip string) string {
	switch chip {
	case "esp32", "esp32s2":
		return "0x1000"
	}
	return "0x0"
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// Install downloads rawURL into dir. A plain file is kept as-is; an
// archive is extracted into a subdirectory and removed. Returns the
// paths of all .bin files that the download produced.
func Install(rawURL, dir string, progress func(done, total int64), log func(string)) ([]string, error) {
	path, err := archive.DownloadFile(rawURL, dir, progress)
	if err != nil {
		return nil, err
	}
	log("Saved: " + path)
	if !archive.IsArchive(filepath.Base(path)) {
		return []string{path}, nil
	}
	sub := filepath.Join(dir, archive.StripExt(filepath.Base(path)))
	log("Extracting to " + sub + " ...")
	if err := archive.Extract(path, sub, log); err != nil {
		return nil, err
	}
	os.Remove(path)
	bins := findBins(sub)
	if len(bins) == 0 {
		return nil, fmt.Errorf("archive %s does not contain any .bin file", filepath.Base(path))
	}
	return bins, nil
}

// findBins returns all .bin files under dir, sorted.
func findBins(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".bin") {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// ---------- GitHub search ----------

const apiBase = "https://api.github.com"

var httpClient = &http.Client{Timeout: 20 * time.Second}

func apiGet(path string, v any) error {
	req, err := http.NewRequest(http.MethodGet, apiBase+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "espflasher")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// Repo is a GitHub search result.
type Repo struct {
	FullName    string
	Description string
	Stars       int
}

// SearchRepos searches GitHub repositories for the given query
// (the word "bootloader" is appended when missing).
func SearchRepos(query string) ([]Repo, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		q = "esp32 bootloader"
	} else if !strings.Contains(strings.ToLower(q), "bootloader") {
		q += " bootloader"
	}
	var res struct {
		Items []struct {
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			Stars       int    `json:"stargazers_count"`
		} `json:"items"`
	}
	if err := apiGet("/search/repositories?per_page=15&q="+url.QueryEscape(q), &res); err != nil {
		return nil, err
	}
	var out []Repo
	for _, it := range res.Items {
		out = append(out, Repo{FullName: it.FullName, Description: it.Description, Stars: it.Stars})
	}
	return out, nil
}

// Asset is a downloadable release file (.bin or archive).
type Asset struct {
	Name string
	Tag  string
	URL  string
	Size int64
}

// SizeH returns the human-readable asset size ("?" when unknown).
func (a Asset) SizeH() string {
	if a.Size <= 0 {
		return "?"
	}
	return humanSize(a.Size)
}

// ListAssets returns release assets of repo (owner/name) that are .bin
// files or archives. Releases without matching assets contribute their
// source zipball instead.
func ListAssets(repo string) ([]Asset, error) {
	var releases []struct {
		Tag     string `json:"tag_name"`
		Zipball string `json:"zipball_url"`
		Assets  []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := apiGet("/repos/"+repo+"/releases?per_page=10", &releases); err != nil {
		return nil, err
	}
	var out []Asset
	for _, r := range releases {
		matched := false
		for _, a := range r.Assets {
			lower := strings.ToLower(a.Name)
			if strings.HasSuffix(lower, ".bin") || archive.IsArchive(lower) {
				out = append(out, Asset{Name: a.Name, Tag: r.Tag, URL: a.URL, Size: a.Size})
				matched = true
			}
		}
		if !matched && r.Zipball != "" {
			out = append(out, Asset{Name: "source-" + r.Tag + ".zip", Tag: r.Tag, URL: r.Zipball, Size: -1})
		}
	}
	return out, nil
}
