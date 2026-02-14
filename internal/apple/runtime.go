package apple

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/ryanmoran/contagent/internal"
	"github.com/ryanmoran/contagent/internal/runtime"
)

const MaxSleepTime = math.MaxInt32

// Compile-time check that Runtime implements runtime.Runtime.
var _ runtime.Runtime = (*Runtime)(nil)

// Runtime implements runtime.Runtime for Apple Container.
// It shells out to the `container` CLI tool.
type Runtime struct {
	runner CommandRunner
}

// NewRuntime creates a Runtime that uses the system `container` CLI.
func NewRuntime() *Runtime {
	return &Runtime{runner: DefaultRunner{}}
}

// NewRuntimeWithRunner creates a Runtime with a custom CommandRunner (for testing).
func NewRuntimeWithRunner(runner CommandRunner) *Runtime {
	return &Runtime{runner: runner}
}

// BuildImage builds a container image using `container build`.
func (r *Runtime) BuildImage(ctx context.Context, dockerfilePath string, imageName internal.ImageName, w internal.Writer) (runtime.Image, error) {
	err := r.runner.Run(ctx, nil, w.GetWriter(), os.Stderr,
		"container", "build",
		"--tag", string(imageName),
		"--file", dockerfilePath,
		".",
	)
	if err != nil {
		return runtime.Image{}, fmt.Errorf("failed to build image %q: %w", imageName, err)
	}
	return runtime.Image{Name: string(imageName)}, nil
}

// CreateContainer creates a container using `container create` with `sleep infinity`
// as the initial command. The actual command is run later via `container exec` in Attach.
func (r *Runtime) CreateContainer(ctx context.Context, opts runtime.CreateContainerOptions) (runtime.Container, error) {
	args := []string{"create", "--name", string(opts.SessionID), "--ssh"}

	for _, env := range opts.Env {
		args = append(args, "--env", env)
	}

	for _, vol := range opts.Volumes {
		args = append(args, "--volume", vol)
	}

	if opts.WorkingDir != "" {
		args = append(args, "--workdir", opts.WorkingDir)
	}

	args = append(args, opts.Image.Name, "sleep", strconv.Itoa(MaxSleepTime))

	err := r.runner.Run(ctx, nil, os.Stdout, os.Stderr, "container", args...)
	if err != nil {
		return nil, fmt.Errorf("failed to create container %q: %w", opts.SessionID, err)
	}

	return &Container{
		name:        string(opts.SessionID),
		cmd:         []string(opts.Args),
		env:         []string(opts.Env),
		workingDir:  opts.WorkingDir,
		stopTimeout: opts.StopTimeout,
		runner:      r.runner,
	}, nil
}

// HostAddress returns the hostname that Apple containers use to reach the host.
func (r *Runtime) HostAddress() string {
	return "host.container.internal"
}

// Close is a no-op for Apple Container runtime (no persistent client).
func (r *Runtime) Close() error {
	return nil
}
