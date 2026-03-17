package internal

import (
	"fmt"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/ryanmoran/contagent/internal/config"
)

const (
	// DefaultStopTimeout is the timeout in seconds for gracefully stopping a container
	// before forcefully killing it. 10 seconds provides enough time for most applications
	// to handle SIGTERM and clean up resources.
	DefaultStopTimeout = 10

	// DefaultTTYRetries is the number of retry attempts for initial TTY resize operations.
	// The container may not be fully ready when we first try to resize, so we retry
	// multiple times with increasing delays.
	DefaultTTYRetries = 10

	// DefaultRetryDelay is the base delay between TTY resize retry attempts.
	// Each retry multiplies this by (retry+1) to implement exponential backoff:
	// 10ms, 20ms, 30ms, etc.
	DefaultRetryDelay = 10 * time.Millisecond
)

type Config struct {
	Runtime     string
	ImageName   ImageName
	WorkingDir  string
	StopTimeout int
	TTYRetries  int
	RetryDelay  time.Duration
	GitUser     GitUserConfig

	Args           Command
	Env            Environment
	Volumes        []string
	DockerfilePath string
	Network        string
}

type GitUserConfig struct {
	Name  string
	Email string
}

// ParseConfig parses command-line arguments and environment variables to construct
// the configuration for running a container. It uses the new config package to load
// and merge configuration from multiple sources (defaults, config files, CLI flags).
// Returns an error if config loading fails (e.g., invalid config file, bad flags).
func ParseConfig(args []string, environment []string, startDir string) (Config, error) {
	// Use the new config package to load and parse configuration
	cfg, programArgs, err := config.Load(args, environment, startDir)
	if err != nil {
		return Config{}, err
	}

	// Resolve runtime (auto-detect if not explicitly set)
	rt := resolveRuntime(cfg.Runtime)

	// Build environment variables with defaults (runtime-aware)
	env := buildEnvironment(environment, cfg.Env, rt)

	// Build volumes with defaults (runtime-aware)
	volumes := buildVolumes(cfg.Volumes, rt)

	// Resolve relative host paths in volumes to absolute paths
	volumes = resolveVolumePaths(volumes, startDir)

	return Config{
		Runtime:        rt,
		ImageName:      ImageName(cfg.Image),
		WorkingDir:     cfg.WorkingDir,
		DockerfilePath: cfg.Dockerfile,
		StopTimeout:    cfg.StopTimeout,
		TTYRetries:     cfg.TTYRetries,
		RetryDelay:     cfg.RetryDelay,
		GitUser: GitUserConfig{
			Name:  cfg.Git.User.Name,
			Email: cfg.Git.User.Email,
		},
		Args:    Command(programArgs),
		Env:     Environment(env),
		Volumes: volumes,
		Network: cfg.Network,
	}, nil
}

// resolveRuntime determines the container runtime to use.
// If explicitly configured, uses that value.
// Otherwise, auto-detects: on macOS with `container` CLI available, uses "apple";
// otherwise defaults to "docker".
func resolveRuntime(configured string) string {
	if configured != "" {
		return configured
	}

	if goruntime.GOOS == "darwin" {
		if _, err := exec.LookPath("container"); err == nil {
			return "apple"
		}
	}

	return "docker"
}

// buildVolumes constructs the volume mount list with runtime-aware defaults.
// Docker gets docker.sock and ssh-auth.sock mounts; Apple gets no default mounts.
func buildVolumes(configVolumes []string, rt string) []string {
	switch rt {
	case "apple":
		return configVolumes
	default: // "docker"
		return append([]string{
			"/var/run/docker.sock:/var/run/docker.sock",
			"/run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock",
		}, configVolumes...)
	}
}

// resolveVolumePaths resolves relative host paths in volume mount specs to absolute paths.
// Volume specs have the format [host-path:]container-path[:options].
// Host paths starting with "." are treated as relative and resolved against baseDir.
// Named volumes (no leading "/" or ".") and container-only specs are left unchanged.
func resolveVolumePaths(volumes []string, baseDir string) []string {
	resolved := make([]string, len(volumes))
	for i, volume := range volumes {
		hostPath, rest, hasColon := strings.Cut(volume, ":")
		if !hasColon || !strings.HasPrefix(hostPath, ".") {
			resolved[i] = volume
			continue
		}
		absPath, err := filepath.Abs(filepath.Join(baseDir, hostPath))
		if err != nil {
			resolved[i] = volume
			continue
		}
		resolved[i] = absPath + ":" + rest
	}
	return resolved
}

// buildEnvironment constructs the environment variable list with runtime-aware defaults
func buildEnvironment(environment []string, configEnv map[string]string, rt string) []string {
	lookup := make(map[string]string)
	for _, variable := range environment {
		key, value, ok := strings.Cut(variable, "=")
		if ok {
			lookup[key] = value
		}
	}

	var env []string

	// Add TERM with default
	value, ok := lookup["TERM"]
	if !ok {
		value = "xterm-256color"
	}
	env = append(env, fmt.Sprintf("TERM=%s", value))

	// Add COLORTERM with default
	value, ok = lookup["COLORTERM"]
	if !ok {
		value = "truecolor"
	}
	env = append(env, fmt.Sprintf("COLORTERM=%s", value))

	// Add ANTHROPIC_API_KEY if present
	if value := lookup["ANTHROPIC_API_KEY"]; value != "" {
		env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", value))
	}

	// Set SSH_AUTH_SOCK for Docker runtime only (Apple uses --ssh flag natively)
	if rt == "docker" {
		env = append(env, "SSH_AUTH_SOCK=/run/host-services/ssh-auth.sock")
	}

	// Add environment variables from config file and CLI flags
	for key, value := range configEnv {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env
}
