package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FindGlobalConfig returns the path to the global config file.
// It checks in order:
//  1. $CONTAGENT_GLOBAL_CONFIG_FILE (if set, must exist)
//  2. $XDG_CONFIG_HOME/contagent/config.yaml (if XDG_CONFIG_HOME set)
//  3. ~/.config/contagent/config.yaml (default)
//
// Returns empty string if no global config found (not an error).
// Returns error if $CONTAGENT_GLOBAL_CONFIG_FILE is set but file doesn't exist.
func FindGlobalConfig(environment []string) (string, error) {
	envMap := makeEnvMap(environment)

	// 1. Check $CONTAGENT_GLOBAL_CONFIG_FILE
	if path := envMap["CONTAGENT_GLOBAL_CONFIG_FILE"]; path != "" {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("CONTAGENT_GLOBAL_CONFIG_FILE is set but file does not exist: %s", path)
			}
			return "", fmt.Errorf("cannot stat CONTAGENT_GLOBAL_CONFIG_FILE: %w", err)
		}
		return path, nil
	}

	// 2. Check $XDG_CONFIG_HOME/contagent/config.yaml
	if xdgConfig := envMap["XDG_CONFIG_HOME"]; xdgConfig != "" {
		path := filepath.Join(xdgConfig, "contagent", "config.yaml")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// 3. Check ~/.config/contagent/config.yaml
	home := envMap["HOME"]
	if home == "" {
		// No HOME set, can't find default config
		return "", nil
	}
	path := filepath.Join(home, ".config", "contagent", "config.yaml")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	return "", nil
}

// FindProjectConfig walks up from startDir looking for .contagent.yaml.
// Returns the path to the first .contagent.yaml found, or empty string if none found.
// Returns error only if filesystem operations fail (e.g., permission denied).
func FindProjectConfig(startDir string) (string, error) {
	// Clean the path to normalize it
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("cannot get absolute path: %w", err)
	}

	// Walk up directory tree
	for {
		configPath := filepath.Join(dir, ".contagent.yaml")
		info, err := os.Stat(configPath)
		if err == nil && !info.IsDir() {
			return configPath, nil
		}
		if err != nil && !os.IsNotExist(err) {
			// Real error (permission denied, etc.)
			return "", fmt.Errorf("cannot stat %s: %w", configPath, err)
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			break
		}
		dir = parent
	}

	return "", nil
}

// ParseFile reads and parses a YAML config file at the given path.
// Returns error if file cannot be read or parsed.
func ParseFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("cannot read config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("cannot parse config file %s: %w", path, err)
	}

	return cfg, nil
}

// makeEnvMap converts environment slice (KEY=VALUE) to a map for easier lookup.
func makeEnvMap(environment []string) map[string]string {
	envMap := make(map[string]string)
	for _, env := range environment {
		if key, value, ok := strings.Cut(env, "="); ok {
			envMap[key] = value
		}
	}
	return envMap
}
