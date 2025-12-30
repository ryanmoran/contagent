package config

import (
	"os"
)

// ExpandEnv expands environment variables in config values.
// It processes:
//   - env map values: expands $VAR and ${VAR} using provided environment
//   - volumes paths: expands variables in volume mount strings
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
			result.Volumes[i] = os.Expand(volume, mapper)
		}
	}

	return result
}
