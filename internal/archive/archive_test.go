package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsArchive(t *testing.T) {
	yes := []string{"a.zip", "b.RAR", "c.7z", "d.tar.gz", "e.tgz", "f.tar.xz", "g.gz", "h.xz", "i.bz2", "j.tar"}
	for _, n := range yes {
		if !IsArchive(n) {
			t.Errorf("IsArchive(%q) = false, want true", n)
		}
	}
	no := []string{"a.bin", "b.img", "c.py", "d"}
	for _, n := range no {
		if IsArchive(n) {
			t.Errorf("IsArchive(%q) = true, want false", n)
		}
	}
}

func TestStripExt(t *testing.T) {
	cases := map[string]string{
		"proj.tar.gz": "proj",
		"proj.tgz":    "proj",
		"boot.bin.gz": "boot.bin",
		"a.zip":       "a",
		"plain.bin":   "plain.bin",
		"archive.7z":  "archive",
		"backup.tar":  "backup",
	}
	for in, want := range cases {
		if got := StripExt(in); got != want {
			t.Errorf("StripExt(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractZip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "test.zip")
	f, _ := os.Create(src)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("sub/boot.bin")
	w.Write([]byte("BOOT"))
	zw.Close()
	f.Close()

	dst := filepath.Join(dir, "out")
	if err := Extract(src, dst, nil); err != nil {
		t.Fatalf("Extract zip: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "sub", "boot.bin"))
	if err != nil || string(data) != "BOOT" {
		t.Fatalf("extracted content wrong: %q err=%v", data, err)
	}
}

func TestExtractZipSlip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "evil.zip")
	f, _ := os.Create(src)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("../evil.txt")
	w.Write([]byte("X"))
	zw.Close()
	f.Close()

	if err := Extract(src, filepath.Join(dir, "out"), nil); err == nil {
		t.Fatal("Extract accepted a zip-slip path, want error")
	}
}

func TestExtractTarGz(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "test.tar.gz")
	f, _ := os.Create(src)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	body := []byte("print('hi')")
	tw.WriteHeader(&tar.Header{Name: "main.py", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
	tw.Write(body)
	tw.Close()
	gz.Close()
	f.Close()

	dst := filepath.Join(dir, "out")
	if err := Extract(src, dst, nil); err != nil {
		t.Fatalf("Extract tar.gz: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "main.py"))
	if err != nil || string(data) != string(body) {
		t.Fatalf("extracted content wrong: %q err=%v", data, err)
	}
}

func TestExtractSingleGz(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "boot.bin.gz")
	f, _ := os.Create(src)
	gz := gzip.NewWriter(f)
	gz.Write([]byte("BIN"))
	gz.Close()
	f.Close()

	dst := filepath.Join(dir, "out")
	if err := Extract(src, dst, nil); err != nil {
		t.Fatalf("Extract gz: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "boot.bin"))
	if err != nil || string(data) != "BIN" {
		t.Fatalf("extracted content wrong: %q err=%v", data, err)
	}
}

func TestUnsafeEntry(t *testing.T) {
	bad := []string{"../evil", "a/../../etc/passwd", "/etc/passwd", "C:\\Windows\\x", "dir\\..\\..\\x"}
	for _, n := range bad {
		if !unsafeEntry(n) {
			t.Errorf("unsafeEntry(%q) = false, want true", n)
		}
	}
	good := []string{"boot.bin", "sub/dir/fw.bin", "a..b/x", "...dots/y", ""}
	for _, n := range good {
		if unsafeEntry(n) {
			t.Errorf("unsafeEntry(%q) = true, want false", n)
		}
	}
}

func TestValidateURL(t *testing.T) {
	ok := []string{"http://example.com/a.bin", "https://x.org/p/q.zip"}
	for _, u := range ok {
		if err := ValidateURL(u); err != nil {
			t.Errorf("ValidateURL(%q) = %v, want nil", u, err)
		}
	}
	bad := []string{"file:///etc/passwd", "ftp://x/y", "gopher://x", "https://", "/local/path", "javascript:alert(1)"}
	for _, u := range bad {
		if err := ValidateURL(u); err == nil {
			t.Errorf("ValidateURL(%q) = nil, want error", u)
		}
	}
}

func TestExtractTarXz(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("xz not installed")
	}
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "test.tar")
	f, _ := os.Create(tarPath)
	tw := tar.NewWriter(f)
	body := []byte("DATA")
	tw.WriteHeader(&tar.Header{Name: "fw.bin", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
	tw.Write(body)
	tw.Close()
	f.Close()
	if out, err := exec.Command("xz", tarPath).CombinedOutput(); err != nil {
		t.Fatalf("xz compress: %v %s", err, out)
	}

	dst := filepath.Join(dir, "out")
	if err := Extract(tarPath+".xz", dst, nil); err != nil {
		t.Fatalf("Extract tar.xz: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "fw.bin"))
	if err != nil || string(data) != "DATA" {
		t.Fatalf("extracted content wrong: %q err=%v", data, err)
	}
}
