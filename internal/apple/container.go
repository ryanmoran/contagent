package apple

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ryanmoran/contagent/internal"
	"github.com/ryanmoran/contagent/internal/runtime"
)

// Compile-time check that Container implements runtime.Container.
var _ runtime.Container = (*Container)(nil)

// Container implements runtime.Container for Apple Container.
// The lifecycle differs from Docker:
//  1. CopyTo: starts the container (running `sleep infinity`), then pipes tar via exec
//  2. Start: no-op (already started in CopyTo)
//  3. Attach: runs the actual command via `container exec --tty --interactive`
//  4. Wait: waits for the exec process to exit
type Container struct {
	name           string
	cmd            []string
	env            []string
	workingDir     string
	stopTimeout    int
	runner         CommandRunner
	started        bool
	process        Process
	readyRetries   int
	readyBaseDelay time.Duration
}

// InspectUser returns the uid and gid of the container's default user.
// It starts the container if not already running, then execs `id -u` and `id -g`.
func (c *Container) InspectUser(ctx context.Context) (runtime.ImageUser, error) {
	if !c.started {
		err := c.runner.Run(ctx, nil, os.Stdout, os.Stderr,
			"container", "start", c.name,
		)
		if err != nil {
			return runtime.ImageUser{}, fmt.Errorf("failed to start container %q for user inspection: %w", c.name, err)
		}

		if err := c.waitForRunning(ctx); err != nil {
			return runtime.ImageUser{}, fmt.Errorf("container %q failed to become ready for user inspection: %w", c.name, err)
		}

		c.started = true
	}

	var uidBuf, gidBuf bytes.Buffer

	if err := c.runner.Run(ctx, nil, &uidBuf, os.Stderr,
		"container", "exec", c.name, "id", "-u",
	); err != nil {
		return runtime.ImageUser{}, fmt.Errorf("failed to get uid from container %q: %w", c.name, err)
	}

	if err := c.runner.Run(ctx, nil, &gidBuf, os.Stderr,
		"container", "exec", c.name, "id", "-g",
	); err != nil {
		return runtime.ImageUser{}, fmt.Errorf("failed to get gid from container %q: %w", c.name, err)
	}

	uid, err := strconv.Atoi(strings.TrimSpace(uidBuf.String()))
	if err != nil {
		return runtime.ImageUser{}, fmt.Errorf("unexpected output from id -u in container %q: %w", c.name, err)
	}

	gid, err := strconv.Atoi(strings.TrimSpace(gidBuf.String()))
	if err != nil {
		return runtime.ImageUser{}, fmt.Errorf("unexpected output from id -g in container %q: %w", c.name, err)
	}

	return runtime.ImageUser{UID: uid, GID: gid}, nil
}

// CopyTo starts the container and copies content via `container exec tar`.
// Apple Container cannot copy files into a stopped container, so we start it first
// with `sleep infinity`, then pipe the tar archive via exec.
func (c *Container) CopyTo(ctx context.Context, content io.Reader, path string) error {
	if !c.started {
		err := c.runner.Run(ctx, nil, os.Stdout, os.Stderr,
			"container", "start", c.name,
		)
		if err != nil {
			return fmt.Errorf("failed to start container %q: %w", c.name, err)
		}

		if err := c.waitForRunning(ctx); err != nil {
			return fmt.Errorf("container %q failed to become ready: %w", c.name, err)
		}

		c.started = true
	}

	err := c.runner.Run(ctx, nil, os.Stdout, os.Stderr,
		"container", "exec", c.name,
		"mkdir", "-p", path,
	)
	if err != nil {
		return fmt.Errorf("failed to create path to content %q: %w", c.name, err)
	}

	err = c.runner.Run(ctx, content, os.Stdout, os.Stderr,
		"container", "exec", "--interactive", c.name,
		"tar", "xf", "-", "-C", path, "--warning", "no-timestamp",
	)
	if err != nil {
		return fmt.Errorf("failed to copy content to container %q: %w", c.name, err)
	}

	return nil
}

// Start is a no-op for Apple Container — the container was already started in CopyTo.
func (c *Container) Start(ctx context.Context) error {
	return nil
}

// Attach runs the actual user command inside the container using
// `container exec --tty --interactive`. Apple Container handles TTY natively.
func (c *Container) Attach(ctx context.Context, cancel context.CancelFunc, w internal.Writer) error {
	args := []string{"exec", "--tty", "--interactive"}

	if c.workingDir != "" {
		args = append(args, "--workdir", c.workingDir)
	}

	for _, env := range c.env {
		args = append(args, "--env", env)
	}

	args = append(args, c.name)

	// TODO: Remove this login shell workaround once apple/container ships a release
	// that includes apple/containerization >= 0.26.5. Two bugs in containerization
	// (PRs #550 and #562, merged Feb 2026) caused `container exec` to ignore the
	// container's configured PATH and environment entirely, falling back to a hardcoded
	// default PATH that omits user-local directories (e.g. ~/.local/bin). Both fixes
	// landed in containerization 0.26.5 and are present in apple/container main, but
	// the latest release (v0.10.0) pins containerization 0.26.3 and does not include
	// them. Wrapping in a login shell sources profile files and resolves PATH correctly
	// regardless of the containerization version in use.
	quoted := make([]string, len(c.cmd))
	for i, arg := range c.cmd {
		quoted[i] = "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
	}
	args = append(args, "/bin/sh", "-l", "-c", "exec "+strings.Join(quoted, " "))

	proc, err := c.runner.Start(ctx, os.Stdin, os.Stdout, os.Stderr, "container", args...)
	if err != nil {
		return fmt.Errorf("failed to exec in container %q: %w", c.name, err)
	}
	c.process = proc

	return nil
}

// Wait waits for the exec process (started in Attach) to exit.
// It handles context cancellation (e.g. from SIGINT/SIGTERM) by stopping the
// container gracefully before waiting for the exec process to exit.
func (c *Container) Wait(ctx context.Context, w internal.Writer) error {
	if c.process == nil {
		return nil
	}

	type result struct {
		exitCode int
		err      error
	}

	done := make(chan result, 1)
	go func() {
		code, err := c.process.Wait()
		done <- result{code, err}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			return fmt.Errorf("process error in container %q: %w", c.name, r.err)
		}
		w.Printf("\nContainer exited with status: %d\n", r.exitCode)
	case <-ctx.Done():
		w.Println("\nReceived signal, stopping container...")
		stopCtx := context.Background()
		if err := c.runner.Run(stopCtx, nil, nil, nil,
			"container", "stop",
			"--time", strconv.Itoa(c.stopTimeout),
			c.name,
		); err != nil {
			w.Warningf("failed to stop container: %v", err)
		}
		<-done
	}

	return nil
}

// waitForRunning polls the container until it is ready to accept exec commands.
// It retries a lightweight exec (`true`) with exponential backoff.
func (c *Container) waitForRunning(ctx context.Context) error {
	maxRetries := c.readyRetries
	if maxRetries == 0 {
		maxRetries = 10
	}

	delay := c.readyBaseDelay
	if delay == 0 {
		delay = 100 * time.Millisecond
	}

	for i := range maxRetries {
		err := c.runner.Run(ctx, nil, nil, nil,
			"container", "exec", c.name, "true",
		)
		if err == nil {
			return nil
		}

		if i == maxRetries-1 {
			return fmt.Errorf("container not ready after %d attempts: %w", maxRetries, err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		delay *= 2
	}

	return nil
}

// ForceRemove forcibly removes the container using `container delete --force`.
func (c *Container) ForceRemove(ctx context.Context) error {
	err := c.runner.Run(ctx, nil, os.Stdout, os.Stderr,
		"container", "delete", "--force", c.name,
	)
	if err != nil {
		return fmt.Errorf("failed to force remove container %q: %w", c.name, err)
	}
	return nil
}
