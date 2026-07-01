package mount

import (
	"os"
	"path/filepath"
	"strings"
)

// HasVolumeMount reports whether a compose volume list binds source to target.
func HasVolumeMount(volumes []string, source, target string) bool {
	source = normalizeSource(source)
	for _, volume := range volumes {
		parts := strings.Split(volume, ":")
		if len(parts) >= 2 && normalizeSource(parts[0]) == source && parts[1] == target {
			return true
		}
	}
	return false
}

func normalizeSource(source string) string {
	source = strings.TrimSpace(os.ExpandEnv(source))
	if source == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Clean(home)
		}
	}
	if strings.HasPrefix(source, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			source = filepath.Join(home, strings.TrimPrefix(source, "~/"))
		}
	}
	return filepath.Clean(source)
}
