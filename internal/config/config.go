package config

import (
	"flag"
	"strings"
	"time"
)

// Config represents the parsed and merged configuration for contagent.
// It includes all settings that can be specified via config files or CLI flags.
type Config struct {
	Runtime     string            `yaml:"runtime"`
	Image       string            `yaml:"image"`
	WorkingDir  string            `yaml:"working_dir"`
	Dockerfile  string            `yaml:"dockerfile"`
	Network     string            `yaml:"network"`
	StopTimeout int               `yaml:"stop_timeout"`
	TTYRetries  int               `yaml:"tty_retries"`
	RetryDelay  time.Duration     `yaml:"retry_delay"`
	Git         GitConfig         `yaml:"git"`
	Env         map[string]string `yaml:"env"`
	Volumes     []string          `yaml:"volumes"`
}

// GitConfig represents Git-specific configuration settings.
type GitConfig struct {
	User GitUserConfig `yaml:"user"`
}

// GitUserConfig represents Git user identity configuration.
type GitUserConfig struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

// stringSlice is a custom flag type that allows multiple values
type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// Load discovers, loads, and merges all configuration sources.
// It follows the resolution order: defaults → global config → project config → CLI flags.
// Environment variable expansion is applied after all merging is complete.
//
// Parameters:
//   - cliArgs: Command-line arguments to parse (flags override config files)
//   - environment: Environment variables for expansion (typically os.Environ())
//   - startDir: Starting directory for project config discovery (walks up looking for .contagent.yaml)
//
// Returns the final merged configuration, remaining program arguments, or an error if loading/parsing fails.
func Load(cliArgs []string, environment []string, startDir string) (Config, []string, error) {
	// 1. Load defaults
	cfg := Config{ //nolint:exhaustruct // Partial initialization with defaults, other fields loaded from config file
		Image:       "contagent:latest",
		WorkingDir:  "/app",
		Network:     "default",
		StopTimeout: 10,
		TTYRetries:  10,
		RetryDelay:  10 * time.Millisecond,
		Git: GitConfig{
			User: GitUserConfig{
				Name:  "Contagent",
				Email: "contagent@example.com",
			},
		},
		Env:     make(map[string]string),
		Volumes: []string{},
	}

	// 2. Find and load global config
	globalConfigPath, err := FindGlobalConfig(environment)
	if err != nil {
		return Config{}, nil, err
	}
	if globalConfigPath != "" {
		globalCfg, err := ParseFile(globalConfigPath)
		if err != nil {
			return Config{}, nil, err
		}
		cfg = Merge(cfg, globalCfg)
	}

	// 3. Find and load project config
	projectConfigPath, err := FindProjectConfig(startDir)
	if err != nil {
		return Config{}, nil, err
	}
	if projectConfigPath != "" {
		projectCfg, err := ParseFile(projectConfigPath)
		if err != nil {
			return Config{}, nil, err
		}
		cfg = Merge(cfg, projectCfg)
	}

	// 4. Parse CLI flags
	var (
		envFlags    stringSlice
		volumeFlags stringSlice
		retryDelay  string
	)

	cliCfg := Config{ //nolint:exhaustruct // Partial initialization, fields populated via CLI flags
		Git: GitConfig{
			User: GitUserConfig{}, //nolint:exhaustruct // Empty, populated via CLI flags
		},
		Env:     make(map[string]string),
		Volumes: []string{},
	}

	fs := flag.NewFlagSet("contagent", flag.ContinueOnError)
	fs.StringVar(&cliCfg.Runtime, "runtime", "", "Container runtime (docker or apple)")
	fs.StringVar(&cliCfg.Dockerfile, "dockerfile", "", "Dockerfile path")
	fs.StringVar(&cliCfg.Image, "image", "", "Container image name")
	fs.StringVar(&cliCfg.WorkingDir, "working-dir", "", "Working directory in container")
	fs.StringVar(&cliCfg.Network, "network", "", "Docker network to use")
	fs.IntVar(&cliCfg.StopTimeout, "stop-timeout", 0, "Stop timeout in seconds")
	fs.IntVar(&cliCfg.TTYRetries, "tty-retries", 0, "TTY retry attempts")
	fs.StringVar(&retryDelay, "retry-delay", "", "Retry delay duration")
	fs.StringVar(&cliCfg.Git.User.Name, "git-user-name", "", "Git user name")
	fs.StringVar(&cliCfg.Git.User.Email, "git-user-email", "", "Git user email")
	fs.Var(&envFlags, "env", "Environment variable (KEY=VALUE)")
	fs.Var(&volumeFlags, "volume", "Volume mount")

	if err := fs.Parse(cliArgs); err != nil {
		return Config{}, nil, err
	}

	// Extract remaining program arguments
	programArgs := fs.Args()

	// 5. Handle retry delay parsing
	if retryDelay != "" {
		duration, err := time.ParseDuration(retryDelay)
		if err != nil {
			return Config{}, nil, err
		}
		cliCfg.RetryDelay = duration
	}

	// Parse env flags
	for _, env := range envFlags {
		key, value, ok := strings.Cut(env, "=")
		if ok {
			cliCfg.Env[key] = value
		}
	}

	// Set volumes
	cliCfg.Volumes = volumeFlags

	// 6. Merge CLI flags with config
	cfg = Merge(cfg, cliCfg)

	// 7. Expand environment variables
	cfg = ExpandEnv(cfg, environment)

	// TODO: 8. Validate

	return cfg, programArgs, nil
}
