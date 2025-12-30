// Package config provides configuration file loading and merging for contagent.
//
// It supports two-tier configuration (global ~/.config/contagent/config.yaml
// and project .contagent.yaml) with hybrid merge strategy: list fields append,
// scalar fields override. Environment variable expansion is supported in
// config values using $VAR and ${VAR} syntax.
//
// Configuration resolution order:
//  1. Hardcoded defaults
//  2. Global config (if exists)
//  3. Project config (if exists)
//  4. CLI flags (final override)
//
// The Load() function is the main entry point for discovering, loading,
// and merging all configuration sources.
package config
