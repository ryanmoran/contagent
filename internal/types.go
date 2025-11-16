package internal

// SessionID represents a unique session identifier for a container.
type SessionID string

// ImageName represents a Docker image name.
type ImageName string

// Command represents the command and arguments to execute in the container.
type Command []string

// Environment represents environment variables to pass to the container.
type Environment []string
