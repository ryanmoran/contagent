package docker

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/cli/cli/streams"
	"github.com/moby/moby/client"
	"github.com/ryanmoran/contagent/internal"
)

type TTY struct {
	client     DockerClient
	out        *streams.Out
	id         string
	maxRetries int
	retryDelay time.Duration
	writer     internal.Writer
}

// NewTTY creates a TTY handler for monitoring and resizing the container's terminal.
// The maxRetries parameter controls how many times to retry initial resize operations,
// and retryDelay specifies the base delay between retries.
func NewTTY(client DockerClient, out *streams.Out, id string, maxRetries int, retryDelay time.Duration, writer internal.Writer) TTY {
	return TTY{
		client:     client,
		out:        out,
		id:         id,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
		writer:     writer,
	}
}

// Monitor monitors the terminal for resize events (SIGWINCH) and automatically resizes
// the container's TTY to match. If the initial resize fails, it retries with exponential
// backoff up to the configured maximum retries. Returns nil after starting background
// monitoring goroutines, or an error if the context is cancelled during setup.
func (t TTY) Monitor(ctx context.Context) error {
	err := t.Resize(ctx)
	if err != nil {
		go func() {
			var err error
			for retry := range t.maxRetries {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(retry+1) * t.retryDelay):
					if err = t.Resize(ctx); err == nil {
						return
					}
				}
			}
			if err != nil {
				t.writer.Fatalf("failed to resize tty: %v", err)
			}
		}()
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGWINCH)
	go func() {
		defer signal.Stop(sigchan)
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigchan:
				_ = t.Resize(ctx)
			}
		}
	}()

	return nil
}

// Resize resizes the container's TTY to match the current terminal dimensions.
// Returns nil if the terminal has zero size (no resize needed) or if the resize succeeds.
// Returns an error if the Docker API call fails.
func (t TTY) Resize(ctx context.Context) error {
	height, width := t.out.GetTtySize()

	if height == 0 && width == 0 {
		return nil
	}

	_, err := t.client.ContainerResize(ctx, t.id, client.ContainerResizeOptions{
		Height: height,
		Width:  width,
	})
	if err != nil {
		return err
	}

	return nil
}
