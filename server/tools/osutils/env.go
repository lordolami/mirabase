package osutils

import "os"

// GetEnv returns the value of the "MIRABASE_<name>" environment variable,
// falling back to the inherited "PB_<name>" one so that applications adopted
// from the upstream project keep working with their existing configuration.
//
// Example: GetEnv("THUMBS_MAX_WORKERS") checks MIRABASE_THUMBS_MAX_WORKERS
// first and PB_THUMBS_MAX_WORKERS second.
func GetEnv(name string) string {
	if v := os.Getenv("MIRABASE_" + name); v != "" {
		return v
	}

	return os.Getenv("PB_" + name)
}
