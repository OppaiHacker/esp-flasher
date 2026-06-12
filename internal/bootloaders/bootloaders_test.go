package bootloaders

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallPlainBin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte{0xE9, 0x01})
	}))
	defer srv.Close()

	dir := t.TempDir()
	bins, err := Install(srv.URL+"/bootloader.bin", dir, nil, func(string) {})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(bins) != 1 || filepath.Base(bins[0]) != "bootloader.bin" {
		t.Fatalf("unexpected bins: %v", bins)
	}
}

func TestInstallZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("build/bootloader.bin")
	w.Write([]byte{0xE9})
	zw.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(buf.Bytes())
	}))
	defer srv.Close()

	dir := t.TempDir()
	bins, err := Install(srv.URL+"/release.zip", dir, nil, func(string) {})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(bins) != 1 || filepath.Base(bins[0]) != "bootloader.bin" {
		t.Fatalf("unexpected bins: %v", bins)
	}
	// archive removed after extraction
	if _, err := os.Stat(filepath.Join(dir, "release.zip")); !os.IsNotExist(err) {
		t.Fatalf("archive not removed: %v", err)
	}
	// Scan finds the extracted bootloader
	found, err := Scan(dir)
	if err != nil || len(found) != 1 {
		t.Fatalf("Scan: %v %v", found, err)
	}
}

func TestOffset(t *testing.T) {
	cases := map[string]string{
		"esp32": "0x1000", "esp32s2": "0x1000",
		"esp32s3": "0x0", "esp32c3": "0x0", "esp32c6": "0x0", "esp8266": "0x0",
	}
	for chip, want := range cases {
		if got := Offset(chip); got != want {
			t.Errorf("Offset(%q) = %q, want %q", chip, got, want)
		}
	}
}
