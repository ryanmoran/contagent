package apple

import (
	"context"
	"errors"
	"io"
	"os/exec"
)

// CommandRunner is an interface for running external commands.
// It enables dependency injection for testing.
type CommandRunner interface {
	// Run executes a command and waits for it to complete.
	Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error

	// Start begins executing a command without waiting for completion.
	// Returns a Process that can be used to wait for the command to finish.
	Start(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) (Process, error)
}

// Process represents a running command that can be waited on.
type Process interface {
	// Wait waits for the process to exit. Returns the exit code and any error.
	// A non-zero exit code is returned with a nil error.
	// A non-nil error indicates something went wrong beyond a normal exit.
	Wait() (exitCode int, err error)
}

// DefaultRunner implements CommandRunner using os/exec.
type DefaultRunner struct{}

// Run executes a command and waits for it to complete.
func (r DefaultRunner) Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // callers are responsible for safe inputs
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// Start begins executing a command without waiting for completion.
func (r DefaultRunner) Start(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) (Process, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // callers are responsible for safe inputs
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &processWrapper{cmd: cmd}, nil
}

// processWrapper wraps *exec.Cmd to implement the Process interface.
type processWrapper struct {
	cmd *exec.Cmd
}

// Wait waits for the wrapped command to exit.
func (p *processWrapper) Wait() (int, error) {
	err := p.cmd.Wait()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}
