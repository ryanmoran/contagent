package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/docker/cli/cli/streams"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/moby/term"
	"github.com/ryanmoran/contagent/internal"
	"golang.org/x/sync/errgroup"
)

type Container struct {
	client DockerClient

	ID          string
	Name        string
	StopTimeout int
	TTYRetries  int
	RetryDelay  time.Duration
}

// Start starts the container. Returns an error if the container fails to start,
// which may indicate a misconfiguration or an unhealthy Docker daemon.
func (c Container) Start(ctx context.Context) error {
	_, err := c.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container %q: %w\nContainer may be misconfigured or Docker daemon may be unhealthy", c.Name, err)
	}

	return nil
}

// Attach attaches to the container's stdin, stdout, and stderr streams with TTY support.
// It sets the terminal to raw mode, monitors terminal resize events, and forwards I/O between
// the local terminal and the container. Returns an error if terminal setup fails, TTY monitoring
// fails, or container attachment fails.
func (c Container) Attach(ctx context.Context, w internal.Writer) error {
	stdin, stdout, _ := term.StdStreams()
	in := streams.NewIn(stdin)
	out := streams.NewOut(stdout)

	// Attempt initial resize - if it fails, the TTY monitor will retry
	height, width := out.GetTtySize()
	_, err := c.client.ContainerResize(ctx, c.ID, client.ContainerResizeOptions{
		Height: height,
		Width:  width,
	})
	if err != nil {
		w.Warningf("failed to resize tty: %v", err)
	}

	tty := NewTTY(c.client, out, c.ID, c.TTYRetries, c.RetryDelay, w)
	err = tty.Monitor(ctx)
	if err != nil {
		return fmt.Errorf("failed to monitor tty size: %w", err)
	}

	restore := sync.OnceFunc(func() {
		in.RestoreTerminal()
		out.RestoreTerminal()
	})

	err = in.SetRawTerminal()
	if err != nil {
		return fmt.Errorf("failed to set stdin to raw terminal mode: %w\nYour terminal may not support TTY operations", err)
	}

	response, err := c.client.ContainerAttach(ctx, c.ID, client.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return fmt.Errorf("failed to attach to container %q: %w\nContainer may have exited prematurely or Docker API is unreachable", c.Name, err)
	}

	// Use errgroup for coordinated goroutine management
	g, gctx := errgroup.WithContext(ctx)

	// Forward stdin to container
	g.Go(func() error {
		defer restore()
		defer response.Conn.Close()

		_, err := io.Copy(response.Conn, in)
		// Context cancellation is expected, not an error
		if gctx.Err() != nil {
			return nil
		}
		if err != nil {
			w.Warningf("stdin forwarding error: %v", err)
		}
		return nil
	})

	err = out.SetRawTerminal()
	if err != nil {
		return fmt.Errorf("failed to set stdout to raw terminal mode: %w\nYour terminal may not support TTY operations", err)
	}

	// Forward container output to stdout
	g.Go(func() error {
		defer restore()

		_, err := io.Copy(out, response.Reader)
		// Context cancellation is expected, not an error
		if gctx.Err() != nil {
			return nil
		}
		if err != nil && err != io.EOF {
			w.Warningf("stdout/stderr forwarding error: %v", err)
		}
		return nil
	})

	// Wait for both goroutines to complete
	// This doesn't block the main flow since the container will terminate
	// or the context will be cancelled
	go func() {
		_ = g.Wait()
	}()

	return nil
}

// Wait waits for the container to exit or for an interrupt signal (SIGINT, SIGTERM).
// If a signal is received, it attempts to gracefully stop the container with the configured
// timeout. Returns an error if waiting for the container fails.
func (c Container) Wait(ctx context.Context, w internal.Writer) error {
	wait := c.client.ContainerWait(ctx, c.ID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-wait.Error:
		if err != nil {
			return fmt.Errorf("failed to wait for container %q: %w\nDocker daemon may have encountered an error", c.Name, err)
		}
	case status := <-wait.Result:
		w.Printf("\nContainer exited with status: %d\n", status.StatusCode)
	case <-sigChan:
		w.Println("\nReceived signal, stopping container...")
		timeout := c.StopTimeout
		_, err := c.client.ContainerStop(ctx, c.ID, client.ContainerStopOptions{Timeout: &timeout})
		if err != nil {
			w.Warningf("failed to stop container: %v", err)
		}
	}
	return nil
}

// Remove removes the container from the Docker daemon.
// Returns an error if the container is still running or cannot be removed.
// Use ForceRemove to remove a running container.
func (c Container) Remove(ctx context.Context) error {
	_, err := c.client.ContainerRemove(ctx, c.ID, client.ContainerRemoveOptions{})
	if err != nil {
		return fmt.Errorf("failed to remove container %q: %w\nContainer may still be running - use ForceRemove if needed", c.Name, err)
	}

	return nil
}

// ForceRemove forcibly removes the container from the Docker daemon, even if it is still running.
// Returns an error if the container cannot be removed, which may indicate an inconsistent state.
func (c Container) ForceRemove(ctx context.Context) error {
	_, err := c.client.ContainerRemove(ctx, c.ID, client.ContainerRemoveOptions{
		Force: true,
	})
	if err != nil {
		return fmt.Errorf("failed to force remove container %q: %w\nContainer may be in an inconsistent state", c.Name, err)
	}

	return nil
}

// CopyTo copies content from a reader to the specified path inside the container.
// The content must be a tar archive. Returns an error if the container is not running,
// the path is invalid, or the copy operation fails.
func (c Container) CopyTo(ctx context.Context, content io.Reader, path string) error {
	_, err := c.client.CopyToContainer(ctx, c.ID, client.CopyToContainerOptions{
		DestinationPath: path,
		Content:         content,
	})
	if err != nil {
		return fmt.Errorf("failed to copy content to container %q at path %q: %w\nCheck that the container is running and path is valid", c.Name, path, err)
	}

	return nil
}
