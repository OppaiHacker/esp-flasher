// Package archive downloads files and extracts archives (zip, tar,
// tar.gz, tar.xz, tar.bz2, gz, xz, bz2, rar, 7z). The formats that the
// Go standard library does not handle (xz, rar, 7z) use external tools
// (xz, unrar/bsdtar, 7z) when available.
package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// archiveExts — recognized archive suffixes, multi-part first so that
// "foo.tar.gz" is not classified as plain ".gz".
var archiveExts = []string{
	".tar.gz", ".tar.xz", ".tar.bz2",
	".tgz", ".txz", ".tbz2",
	".tar", ".zip", ".rar", ".7z",
	".gz", ".xz", ".bz2",
}

// IsArchive reports whether the filename has a supported archive extension.
func IsArchive(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range archiveExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// StripExt removes the archive extension from a filename
// ("proj.tar.gz" → "proj"). Non-archive names are returned unchanged.
func StripExt(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range archiveExts {
		if strings.HasSuffix(lower, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	return name
}

// Extract unpacks the src archive into dstDir (created if missing).
// log receives progress lines (may be nil).
func Extract(src, dstDir string, log func(string)) error {
	if log == nil {
		log = func(string) {}
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("cannot create directory %s: %w", dstDir, err)
	}
	lower := strings.ToLower(src)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(src, dstDir, log)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTar(src, dstDir, "gz", log)
	case strings.HasSuffix(lower, ".tar.xz"), strings.HasSuffix(lower, ".txz"):
		return extractTar(src, dstDir, "xz", log)
	case strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz2"):
		return extractTar(src, dstDir, "bz2", log)
	case strings.HasSuffix(lower, ".tar"):
		return extractTar(src, dstDir, "", log)
	case strings.HasSuffix(lower, ".rar"):
		return extractRar(src, dstDir, log)
	case strings.HasSuffix(lower, ".7z"):
		return extract7z(src, dstDir, log)
	case strings.HasSuffix(lower, ".gz"):
		return extractSingle(src, dstDir, "gz")
	case strings.HasSuffix(lower, ".xz"):
		return extractSingle(src, dstDir, "xz")
	case strings.HasSuffix(lower, ".bz2"):
		return extractSingle(src, dstDir, "bz2")
	}
	return fmt.Errorf("unsupported archive format: %s", filepath.Base(src))
}

// safeJoin joins dir+name and rejects paths escaping dir (zip-slip).
func safeJoin(dir, name string) (string, error) {
	target := filepath.Join(dir, filepath.FromSlash(name))
	if !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry escapes target directory: %s", name)
	}
	return target, nil
}

func extractZip(src, dstDir string, log func(string)) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("cannot open archive %s: %w", filepath.Base(src), err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		target, err := safeJoin(dstDir, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("cannot read entry %s: %w", f.Name, err)
		}
		err = writeFile(target, rc)
		rc.Close()
		if err != nil {
			return err
		}
		log("  " + f.Name)
	}
	return nil
}

// extractTar unpacks a tar stream; decomp selects the decompression
// layer: "" (none), "gz", "bz2" or "xz" (external xz tool).
func extractTar(src, dstDir, decomp string, log func(string)) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open file %s: %w", src, err)
	}
	defer in.Close()

	var r io.Reader = in
	var cmd *exec.Cmd
	switch decomp {
	case "gz":
		gz, err := gzip.NewReader(in)
		if err != nil {
			return fmt.Errorf("file %s is not a valid gzip archive: %w", filepath.Base(src), err)
		}
		defer gz.Close()
		r = gz
	case "bz2":
		r = bzip2.NewReader(in)
	case "xz":
		if _, err := exec.LookPath("xz"); err != nil {
			return fmt.Errorf("missing 'xz' program — install xz-utils package: %w", err)
		}
		cmd = exec.Command("xz", "-dc")
		cmd.Stdin = in
		pr, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return err
		}
		r = pr
	}

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar %s failed: %w", filepath.Base(src), err)
		}
		target, err := safeJoin(dstDir, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := writeFile(target, tr); err != nil {
				return err
			}
			log("  " + hdr.Name)
		}
	}
	if cmd != nil {
		if err := cmd.Wait(); err != nil {
			return fmt.Errorf("xz decompression failed: %w", err)
		}
	}
	return nil
}

// extractSingle decompresses a single compressed file (.gz/.xz/.bz2,
// not a tar) into dstDir under the name without the extension.
func extractSingle(src, dstDir, decomp string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open file %s: %w", src, err)
	}
	defer in.Close()

	var r io.Reader
	switch decomp {
	case "gz":
		gz, err := gzip.NewReader(in)
		if err != nil {
			return fmt.Errorf("file %s is not a valid gzip archive: %w", filepath.Base(src), err)
		}
		defer gz.Close()
		r = gz
	case "bz2":
		r = bzip2.NewReader(in)
	case "xz":
		if _, err := exec.LookPath("xz"); err != nil {
			return fmt.Errorf("missing 'xz' program — install xz-utils package: %w", err)
		}
		cmd := exec.Command("xz", "-dc")
		cmd.Stdin = in
		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("xz decompression failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
		}
		r = &out
	default:
		return fmt.Errorf("unknown compression: %s", decomp)
	}

	target := filepath.Join(dstDir, StripExt(filepath.Base(src)))
	return writeFile(target, r)
}

// extractRar unpacks .rar via external tool: unrar, bsdtar or 7z.
func extractRar(src, dstDir string, log func(string)) error {
	if p, err := exec.LookPath("unrar"); err == nil {
		return runTool(log, p, "x", "-o+", "-y", src, dstDir+string(os.PathSeparator))
	}
	if p, err := exec.LookPath("bsdtar"); err == nil {
		return runTool(log, p, "-xvf", src, "-C", dstDir)
	}
	for _, name := range []string{"7z", "7za"} {
		if p, err := exec.LookPath(name); err == nil {
			return runTool(log, p, "x", "-y", "-o"+dstDir, src)
		}
	}
	return fmt.Errorf("no tool to unpack .rar — install 'unrar', 'bsdtar' or '7zip'")
}

// extract7z unpacks .7z via external tool: 7z, 7za or bsdtar.
func extract7z(src, dstDir string, log func(string)) error {
	for _, name := range []string{"7z", "7za"} {
		if p, err := exec.LookPath(name); err == nil {
			return runTool(log, p, "x", "-y", "-o"+dstDir, src)
		}
	}
	if p, err := exec.LookPath("bsdtar"); err == nil {
		return runTool(log, p, "-xvf", src, "-C", dstDir)
	}
	return fmt.Errorf("no tool to unpack .7z — install '7zip' or 'bsdtar'")
}

// runTool runs an external unpacker, passing its output to log.
func runTool(log func(string), name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	for _, line := range strings.Split(out.String(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			log("  " + line)
		}
	}
	if err != nil {
		return fmt.Errorf("%s failed: %w", filepath.Base(name), err)
	}
	return nil
}

func writeFile(target string, r io.Reader) error {
	out, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("cannot create file %s: %w", target, err)
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		os.Remove(target)
		return fmt.Errorf("writing %s failed: %w", target, err)
	}
	return out.Close()
}

// ---------- download ----------

// countingReader reports download progress at most every 1 MiB.
type countingReader struct {
	r        io.Reader
	done     int64
	total    int64
	last     int64
	progress func(done, total int64)
}

const progressStep = 1 << 20

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.done += int64(n)
	if c.progress != nil && (c.done-c.last >= progressStep || err != nil) {
		c.last = c.done
		c.progress(c.done, c.total)
	}
	return n, err
}

// downloadFilename picks the saved-file name from the
// Content-Disposition header or the final (post-redirect) URL.
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
		return "download"
	}
	return name
}

// DownloadFile downloads rawURL into dstDir, reporting
// progress(done, total) (total is -1 when the server does not send a
// size). Returns the path of the saved file.
func DownloadFile(rawURL, dstDir string, progress func(done, total int64)) (string, error) {
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return "", fmt.Errorf("cannot create directory %s: %w", dstDir, err)
	}
	client := &http.Client{Timeout: 30 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("invalid URL %s: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", "espflasher")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading %s failed: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s failed: server replied %s", rawURL, resp.Status)
	}

	dst := filepath.Join(dstDir, downloadFilename(resp, rawURL))
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return "", fmt.Errorf("cannot create file %s: %w", tmp, err)
	}
	cr := &countingReader{r: resp.Body, total: resp.ContentLength, progress: progress}
	if _, err := io.Copy(out, cr); err != nil {
		out.Close()
		os.Remove(tmp)
		return "", fmt.Errorf("downloading %s was interrupted: %w", rawURL, err)
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
