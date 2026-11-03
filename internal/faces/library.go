package faces

import (
	"os"
	"path/filepath"
	"runtime"
)

// LibraryPath finds the ONNX Runtime shared library. It looks, in order, at:
//
//  1. the UMBELA_ORT_LIB environment variable,
//  2. next to the executable (or in a lib folder next to it),
//  3. /usr/lib/umbela (where the Linux packages install it) and
//     /usr/local/lib/umbela (where the plain downloads are meant to go).
//
// If nothing is found it returns "" and the system's library search is used.
func LibraryPath() string {
	if p := os.Getenv("UMBELA_ORT_LIB"); p != "" {
		return p
	}

	var pattern string
	switch runtime.GOOS {
	case "windows":
		pattern = "onnxruntime.dll"
	case "darwin":
		pattern = "libonnxruntime*.dylib"
	default:
		pattern = "libonnxruntime.so*"
	}

	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		dirs = append(dirs, dir, filepath.Join(dir, "lib"))
	}
	switch runtime.GOOS {
	case "linux":
		dirs = append(dirs, "/usr/lib/umbela", "/usr/local/lib/umbela")
	case "darwin":
		dirs = append(dirs, "/usr/local/lib/umbela")
	}

	for _, dir := range dirs {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err == nil && len(matches) > 0 {
			return matches[0]
		}
	}
	return ""
}
