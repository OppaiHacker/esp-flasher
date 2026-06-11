package rpi

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// countingReader counts read bytes and reports progress via
// progress(done, total). It reports at most every progressStep bytes
// and at the end of the stream to avoid flooding the UI with calls.
type countingReader struct {
	r        io.Reader
	done     int64
	total    int64
	last     int64
	progress func(done, total int64)
}

const progressStep = 4 << 20 // 4 MiB

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.done += int64(n)
	if c.progress != nil && (c.done-c.last >= progressStep || err != nil) {
		c.last = c.done
		c.progress(c.done, c.total)
	}
	return n, err
}

// hasImageExt checks if the filename has an extension supported
// by ListImages.
func hasImageExt(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{".img", ".iso", ".img.xz", ".img.gz", ".zip"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// ListImages returns images (.img/.iso/.img.xz/.img.gz/.zip) found
// in the isoDir directory, sorted by name.
func ListImages(isoDir string) ([]Image, error) {
	entries, err := os.ReadDir(isoDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read images directory %s: %w", isoDir, err)
	}
	var images []Image
	for _, e := range entries {
		if e.IsDir() || !hasImageExt(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue // file might have disappeared in the meantime — skip
		}
		images = append(images, Image{
			Path:       filepath.Join(isoDir, e.Name()),
			Name:       e.Name(),
			SizeH:      HumanSize(info.Size()),
			Compressed: isCompressedName(e.Name()),
		})
	}
	sort.Slice(images, func(i, j int) bool { return images[i].Name < images[j].Name })
	return images, nil
}

// downloadFilename determines the name of the saved file based
// on the Content-Disposition header or the final URL (after
// redirects).
func downloadFilename(resp *http.Response, rawURL string) string {
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if name := filepath.Base(params["filename"]); name != "" && name != "." && name != "/" {
				return name
			}
		}
	}
	u := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		u = resp.Request.URL.Path
	}
	name := path.Base(u)
	if name == "" || name == "." || name == "/" {
		return "downloaded-image.img"
	}
	return name
}

// DownloadImage downloads an image from the given url to isoDir,
// calling progress(done, total) during download (total can
// be -1 if the server doesn't provide the size). Returns the path
// to the saved file.
func DownloadImage(url, isoDir string, progress func(done, total int64)) (string, error) {
	if err := os.MkdirAll(isoDir, 0o755); err != nil {
		return "", fmt.Errorf("cannot create directory %s: %w", isoDir, err)
	}
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("downloading %s failed: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s failed: server replied %s", url, resp.Status)
	}

	dst := filepath.Join(isoDir, downloadFilename(resp, url))
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return "", fmt.Errorf("cannot create file %s: %w", tmp, err)
	}

	cr := &countingReader{r: resp.Body, total: resp.ContentLength, progress: progress}
	if _, err := io.Copy(out, cr); err != nil {
		out.Close()
		os.Remove(tmp)
		return "", fmt.Errorf("downloading %s was interrupted: %w", url, err)
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("saving file %s failed: %w", tmp, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("cannot rename temporary file: %w", err)
	}
	return dst, nil
}

// Decompress decompresses an .xz image (via `xz -dc`), .gz
// (compress/gzip) or .zip (archive/zip) next to the source file,
// reporting progress. Returns the path to the decompressed .img file.
// For plain .img (or .iso) it does nothing.
func Decompress(img string, progress func(done, total int64)) (string, error) {
	lower := strings.ToLower(img)
	switch {
	case strings.HasSuffix(lower, ".img"), strings.HasSuffix(lower, ".iso"):
		return img, nil // already uncompressed
	case strings.HasSuffix(lower, ".xz"):
		return decompressXZ(img, progress)
	case strings.HasSuffix(lower, ".gz"):
		return decompressGZ(img, progress)
	case strings.HasSuffix(lower, ".zip"):
		return decompressZip(img, progress)
	default:
		return "", fmt.Errorf("unsupported image format: %s", filepath.Base(img))
	}
}

// stripExt removes the last extension and ensures that the result
// ends with .img (or .iso).
func stripExt(img string) string {
	out := strings.TrimSuffix(img, filepath.Ext(img))
	lower := strings.ToLower(out)
	if !strings.HasSuffix(lower, ".img") && !strings.HasSuffix(lower, ".iso") {
		out += ".img"
	}
	return out
}

// decompressXZ decompresses an .xz file with `xz -dc`, providing data
// on stdin (so we know the progress on the compressed side).
func decompressXZ(img string, progress func(done, total int64)) (string, error) {
	if _, err := exec.LookPath("xz"); err != nil {
		return "", fmt.Errorf("missing 'xz' program — install xz-utils package: %w", err)
	}
	in, err := os.Open(img)
	if err != nil {
		return "", fmt.Errorf("cannot open file %s: %w", img, err)
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return "", fmt.Errorf("cannot read file size %s: %w", img, err)
	}

	outPath := stripExt(img)
	out, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("cannot create file %s: %w", outPath, err)
	}

	cmd := exec.Command("xz", "-dc")
	cmd.Stdin = &countingReader{r: in, total: info.Size(), progress: progress}
	cmd.Stdout = out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		out.Close()
		os.Remove(outPath)
		return "", fmt.Errorf("xz decompression failed: %w (%s)",
			err, strings.TrimSpace(stderr.String()))
	}
	if err := out.Close(); err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("saving file %s failed: %w", outPath, err)
	}
	return outPath, nil
}

// decompressGZ decompresses a .gz file with the compress/gzip library;
// progress is counted on the compressed side.
func decompressGZ(img string, progress func(done, total int64)) (string, error) {
	in, err := os.Open(img)
	if err != nil {
		return "", fmt.Errorf("cannot open file %s: %w", img, err)
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return "", fmt.Errorf("cannot read file size %s: %w", img, err)
	}

	gz, err := gzip.NewReader(&countingReader{r: in, total: info.Size(), progress: progress})
	if err != nil {
		return "", fmt.Errorf("file %s is not a valid gzip archive: %w", filepath.Base(img), err)
	}
	defer gz.Close()

	outPath := stripExt(img)
	out, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("cannot create file %s: %w", outPath, err)
	}
	if _, err := io.Copy(out, gz); err != nil {
		out.Close()
		os.Remove(outPath)
		return "", fmt.Errorf("gzip decompression failed: %w", err)
	}
	if err := out.Close(); err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("saving file %s failed: %w", outPath, err)
	}
	return outPath, nil
}

// zipImageEntry finds the first .img or .iso entry in a ZIP archive.
func zipImageEntry(zr *zip.ReadCloser) *zip.File {
	for _, f := range zr.File {
		lower := strings.ToLower(f.Name)
		if strings.HasSuffix(lower, ".img") || strings.HasSuffix(lower, ".iso") {
			return f
		}
	}
	return nil
}

// decompressZip extracts an image from a .zip archive; progress is counted
// on the uncompressed side (entry size is known from the ZIP header).
func decompressZip(img string, progress func(done, total int64)) (string, error) {
	zr, err := zip.OpenReader(img)
	if err != nil {
		return "", fmt.Errorf("cannot open archive %s: %w", filepath.Base(img), err)
	}
	defer zr.Close()

	entry := zipImageEntry(zr)
	if entry == nil {
		return "", fmt.Errorf("archive %s does not contain an .img or .iso file", filepath.Base(img))
	}
	rc, err := entry.Open()
	if err != nil {
		return "", fmt.Errorf("cannot read entry %s from archive: %w", entry.Name, err)
	}
	defer rc.Close()

	outPath := filepath.Join(filepath.Dir(img), filepath.Base(entry.Name))
	out, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("cannot create file %s: %w", outPath, err)
	}
	cr := &countingReader{r: rc, total: int64(entry.UncompressedSize64), progress: progress}
	if _, err := io.Copy(out, cr); err != nil {
		out.Close()
		os.Remove(outPath)
		return "", fmt.Errorf("zip decompression failed: %w", err)
	}
	if err := out.Close(); err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("saving file %s failed: %w", outPath, err)
	}
	return outPath, nil
}
