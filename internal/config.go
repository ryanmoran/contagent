package internal

import (
	"flag"
	"fmt"
	"strings"
	"time"
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

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// ParseConfig parses command-line arguments and environment variables to construct
// the configuration for running a container. It extracts flags (--env, --volume, --dockerfile),
// captures the remaining arguments as the command to execute, and sets up default environment
// variables (TERM, COLORTERM, ANTHROPIC_API_KEY) and volume mounts (Docker socket, SSH agent).
func ParseConfig(args []string, environment []string) Config {
	lookup := make(map[string]string)
	for _, variable := range environment {
		key, value, ok := strings.Cut(variable, "=")
		if ok {
			lookup[key] = value
		}
	}

	var (
		additionalEnv  stringSlice
		volumes        stringSlice
		dockerfilePath string
		network        string
	)

	fs := flag.NewFlagSet("contagent", flag.ContinueOnError)
	fs.Var(&additionalEnv, "env", "environment variable")
	fs.Var(&volumes, "volume", "volume mount")
	fs.StringVar(&dockerfilePath, "dockerfile", "", "Dockerfile path")
	fs.StringVar(&network, "network", "default", "	Connect to a container network")

	// Ignore errors since we want to capture remaining args
	_ = fs.Parse(args)

	programArgs := fs.Args()

	var env []string
	value, ok := lookup["TERM"]
	if !ok {
		value = "xterm-256color"
	}
	env = append(env, fmt.Sprintf("TERM=%s", value))

	value, ok = lookup["COLORTERM"]
	if !ok {
		value = "truecolor"
	}
	env = append(env, fmt.Sprintf("COLORTERM=%s", value))

	value = lookup["ANTHROPIC_API_KEY"]
	env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", value))

	// Set SSH_AUTH_SOCK for SSH agent access in container
	env = append(env, "SSH_AUTH_SOCK=/run/host-services/ssh-auth.sock")

	env = append(env, additionalEnv...)

	// Add Docker socket and SSH agent socket mounts
	defaultVolumes := []string{
		"/var/run/docker.sock:/var/run/docker.sock",
		"/run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock",
	}
	allVolumes := append(defaultVolumes, volumes...)

	return Config{
		ImageName:      ImageName("contagent:latest"),
		WorkingDir:     "/app",
		DockerfilePath: dockerfilePath,
		StopTimeout:    DefaultStopTimeout,
		TTYRetries:     DefaultTTYRetries,
		RetryDelay:     DefaultRetryDelay,
		GitUser: GitUserConfig{
			Name:  "Contagent",
			Email: "contagent@example.com",
		},
		Args:    Command(programArgs),
		Env:     Environment(env),
		Volumes: allVolumes,
		Network: network,
	}
}
