package runtime

import (
	"context"
	"io"
	"time"

	"github.com/ryanmoran/contagent/internal"
)

// Image represents a container image.
type Image struct {
	Name string
}

// CreateContainerOptions bundles the configuration for creating a container.
type CreateContainerOptions struct {
	SessionID   internal.SessionID
	Image       Image
	Args        internal.Command
	Env         internal.Environment
	Volumes     []string
	WorkingDir  string
	Network     string
	StopTimeout int
	TTYRetries  int
	RetryDelay  time.Duration
}

// Runtime is the interface that container runtimes must implement.
type Runtime interface {
	BuildImage(ctx context.Context, dockerfilePath string, imageName internal.ImageName, w internal.Writer) (Image, error)
	CreateContainer(ctx context.Context, opts CreateContainerOptions) (Container, error)
	HostAddress() string
	Close() error
}

// Container is the interface for interacting with a container.
type Container interface {
	CopyTo(ctx context.Context, content io.Reader, path string) error
	Start(ctx context.Context) error
	Attach(ctx context.Context, w internal.Writer) error
	Wait(ctx context.Context, w internal.Writer) error
	ForceRemove(ctx context.Context) error
}
