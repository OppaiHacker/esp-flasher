package projects

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureManifestMicropython(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.py"), []byte("print(1)"), 0o644)
	os.WriteFile(filepath.Join(dir, "lib.py"), []byte("x=1"), 0o644)

	if err := ensureManifest(dir, "demo", func(string) {}); err != nil {
		t.Fatalf("ensureManifest: %v", err)
	}
	projs, err := Scan(filepath.Dir(dir))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(projs) != 1 || projs[0].Type != "micropython" || projs[0].Main != "main.py" || len(projs[0].Files) != 2 {
		t.Fatalf("unexpected manifest: %+v", projs)
	}
}

func TestEnsureManifestBin(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "firmware.bin"), []byte{0xE9}, 0o644)

	if err := ensureManifest(dir, "fw", func(string) {}); err != nil {
		t.Fatalf("ensureManifest: %v", err)
	}
	projs, err := Scan(filepath.Dir(dir))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(projs) != 1 || projs[0].Type != "bin" || projs[0].Bin != "firmware.bin" || projs[0].FlashOffset != "0x10000" {
		t.Fatalf("unexpected manifest: %+v", projs)
	}
}

func TestEnsureManifestEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := ensureManifest(dir, "empty", func(string) {}); err == nil {
		t.Fatal("want error for directory without flashable files")
	}
}

func TestFlattenSingleDir(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "repo-main")
	os.MkdirAll(inner, 0o755)
	os.WriteFile(filepath.Join(inner, "main.py"), []byte("print(1)"), 0o644)

	if err := flattenSingleDir(dir); err != nil {
		t.Fatalf("flattenSingleDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "main.py")); err != nil {
		t.Fatalf("main.py not moved up: %v", err)
	}
	if _, err := os.Stat(inner); !os.IsNotExist(err) {
		t.Fatalf("inner dir still present: %v", err)
	}
}
