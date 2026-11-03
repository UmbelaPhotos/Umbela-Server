package tor

import (
	"os"
	"path/filepath"
	"runtime"
)

// bundledTor looks for the Tor that is shipped next to the umbela executable
// (the Windows download carries tor/tor.exe and data/geoip*). It returns the
// path to the executable, and the extra arguments to give it, or "" when
// there is none, in which case the `tor` found in the PATH is used.
func bundledTor() (exe string, args []string) {
	self, err := os.Executable()
	if err != nil {
		return "", nil
	}
	dir := filepath.Dir(self)

	name := "tor"
	if runtime.GOOS == "windows" {
		name = "tor.exe"
	}
	exe = filepath.Join(dir, "tor", name)
	if info, err := os.Stat(exe); err != nil || info.IsDir() {
		return "", nil
	}

	// Tor can run without the GeoIP files, but it needs them for country data.
	for flag, file := range map[string]string{"--GeoIPFile": "geoip", "--GeoIPv6File": "geoip6"} {
		path := filepath.Join(dir, "data", file)
		if _, err := os.Stat(path); err == nil {
			args = append(args, flag, path)
		}
	}
	return exe, args
}
