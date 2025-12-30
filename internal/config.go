package internal

import (
	"fmt"
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
func ParseConfig(args []string, environment []string) (Config, error) {
	// Use the new config package to load and parse configuration
	cfg, programArgs, err := config.Load(args, environment)
	if err != nil {
		return Config{}, err
	}

	// Build environment variables with defaults and config env
	env := buildEnvironment(environment, cfg.Env)

	// Build volumes with defaults
	volumes := append([]string{
		"/var/run/docker.sock:/var/run/docker.sock",
		"/run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock",
	}, cfg.Volumes...)

	return Config{
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

// buildEnvironment constructs the environment variable list with defaults
func buildEnvironment(environment []string, configEnv map[string]string) []string {
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
	value = lookup["ANTHROPIC_API_KEY"]
	env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", value))

	// Set SSH_AUTH_SOCK for SSH agent access in container
	env = append(env, "SSH_AUTH_SOCK=/run/host-services/ssh-auth.sock")

	// Add environment variables from config file and CLI flags
	for key, value := range configEnv {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env
}
