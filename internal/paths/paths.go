// Package paths determines project directories (tools/, projects/, firmware/, iso/).
package paths

import (
	"os"
	"path/filepath"
)

// BaseDir returns the root tool directory. First it checks the current working directory
// (`go run` mode / running from repo), then the binary directory.
func BaseDir() string {
	if cwd, err := os.Getwd(); err == nil {
		if dirExists(filepath.Join(cwd, "projects")) || dirExists(filepath.Join(cwd, "tools")) {
			return cwd
		}
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if dirExists(filepath.Join(dir, "projects")) || dirExists(filepath.Join(dir, "tools")) {
			return dir
		}
	}
	cwd, _ := os.Getwd()
	return cwd
}

func ToolsDir() string    { return ensure(filepath.Join(BaseDir(), "tools")) }
func ProjectsDir() string { return ensure(filepath.Join(BaseDir(), "projects")) }
func FirmwareDir() string { return ensure(filepath.Join(BaseDir(), "firmware")) }
func IsoDir() string      { return ensure(filepath.Join(BaseDir(), "iso")) }
func LogsDir() string     { return ensure(filepath.Join(BaseDir(), "logs")) }

func ensure(dir string) string {
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
