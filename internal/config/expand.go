package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ExpandEnv expands environment variables in config values.
// It processes:
//   - env map values: expands $VAR and ${VAR} using provided environment
//   - volumes paths: expands variables in volume mount strings
//   - file paths: expands ~/ prefix to user's home directory in WorkingDir, Dockerfile, and Volumes
//
// Uses os.ExpandEnv behavior: undefined variables expand to empty string.
// Returns a new Config with expanded values.
func ExpandEnv(cfg Config, environment []string) Config {
	// Create a mapping function from the environment slice
	envMap := makeEnvMap(environment)
	mapper := func(varName string) string {
		return envMap[varName]
	}

	// Create a new config to avoid modifying the original
	result := cfg

	// Expand environment variables in Env map
	if cfg.Env != nil {
		result.Env = make(map[string]string, len(cfg.Env))
		for key, value := range cfg.Env {
			result.Env[key] = os.Expand(value, mapper)
		}
	}

	// Expand environment variables in Volumes slice
	if cfg.Volumes != nil {
		result.Volumes = make([]string, len(cfg.Volumes))
		for i, volume := range cfg.Volumes {
			expanded := os.Expand(volume, mapper)
			result.Volumes[i] = expandHome(expanded)
		}
	}

	// Expand home directory in file path fields
	result.WorkingDir = expandHome(cfg.WorkingDir)
	result.Dockerfile = expandHome(cfg.Dockerfile)

	return result
}

// expandHome expands the tilde (~) prefix in a path to the user's home directory.
// Only paths that start with ~/ are expanded. Other instances of ~ are left as literals.
// If the path starts with ~/ but the home directory cannot be determined, the path is returned unchanged.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}

	return filepath.Join(home, path[2:])
}
