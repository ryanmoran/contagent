package apple_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/ryanmoran/contagent/internal/apple"
	"github.com/ryanmoran/contagent/internal/runtime"
	"github.com/stretchr/testify/require"
)

func createTestContainer(t *testing.T, runner *mockRunner) runtime.Container {
	t.Helper()
	rt := apple.NewRuntimeWithRunner(runner)
	container, err := rt.CreateContainer(context.Background(), runtime.CreateContainerOptions{
		SessionID:   "test-session",
		Image:       runtime.Image{Name: "myimage:latest"},
		Args:        []string{"echo", "hello"},
		Env:         []string{"FOO=bar"},
		WorkingDir:  "/app",
		StopTimeout: 10,
	})
	require.NoError(t, err)
	// Use fast timing for readiness checks in tests
	ac, ok := container.(*apple.Container)
	require.True(t, ok, "container should be *apple.Container type")
	ac.SetReadyCheckTiming(10, 1*time.Millisecond)
	// Reset calls so tests only see calls they care about
	runner.calls = nil
	return container
}

func TestContainerCopyTo(t *testing.T) {
	t.Run("starts container and pipes tar", func(t *testing.T) {
		runner := &mockRunner{}
		container := createTestContainer(t, runner)

		tarContent := bytes.NewReader([]byte("fake tar data"))
		err := container.CopyTo(context.Background(), tarContent, "/")
		require.NoError(t, err)

		// Should have three calls: start + exec true (readiness check) + exec tar
		require.Len(t, runner.calls, 4)

		// First call: container start
		require.Equal(t, "container", runner.calls[0].Name)
		require.Equal(t, runner.calls[0].Args, []string{"start", "test-session"})

		// Second call: readiness check (exec true)
		require.Equal(t, "container", runner.calls[1].Name)
		require.Equal(t, runner.calls[1].Args, []string{"exec", "test-session", "true"})

		// Third call: make working dir (exec true)
		require.Equal(t, "container", runner.calls[2].Name)
		require.Equal(t, runner.calls[2].Args, []string{"exec", "test-session", "mkdir", "-p", "/"})

		// Third call: container exec tar
		require.Equal(t, "container", runner.calls[3].Name)
		require.Equal(t, runner.calls[3].Args, []string{"exec", "--interactive", "test-session", "tar", "xf", "-", "-C", "/", "--warning", "no-timestamp"})
	})

	t.Run("only starts container once on multiple CopyTo calls", func(t *testing.T) {
		runner := &mockRunner{}
		container := createTestContainer(t, runner)

		err := container.CopyTo(context.Background(), bytes.NewReader(nil), "/")
		require.NoError(t, err)

		err = container.CopyTo(context.Background(), bytes.NewReader(nil), "/tmp")
		require.NoError(t, err)

		// Should have 4 calls: start + exec true (readiness) + exec tar + exec tar (no second start)
		require.Len(t, runner.calls, 6)
		require.Contains(t, runner.calls[0].Args, "start")
		require.Contains(t, runner.calls[1].Args, "true")  // readiness check
		require.Contains(t, runner.calls[2].Args, "mkdir") // make working dir
		require.Contains(t, runner.calls[3].Args, "tar")   // first CopyTo
		require.Contains(t, runner.calls[4].Args, "mkdir") // make working dir
		require.Contains(t, runner.calls[5].Args, "tar")   // second CopyTo
	})

	t.Run("returns error on start failure", func(t *testing.T) {
		callCount := 0
		runner := &mockRunner{
			runFunc: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
				callCount++
				if callCount == 2 { // first call is "create" in createTestContainer
					return errors.New("start failed")
				}
				return nil
			},
		}
		rt := apple.NewRuntimeWithRunner(runner)
		container, err := rt.CreateContainer(context.Background(), runtime.CreateContainerOptions{
			SessionID: "test",
			Image:     runtime.Image{Name: "img"},
			Args:      []string{"cmd"},
		})
		require.NoError(t, err)

		err = container.CopyTo(context.Background(), nil, "/")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to start container")
	})

	t.Run("retries readiness check until container is ready", func(t *testing.T) {
		readinessAttempts := 0
		runner := &mockRunner{
			runFunc: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
				for _, arg := range args {
					if arg == "true" {
						readinessAttempts++
						if readinessAttempts < 3 {
							return errors.New("container not ready")
						}
						return nil
					}
				}
				return nil
			},
		}
		rt := apple.NewRuntimeWithRunner(runner)
		container, err := rt.CreateContainer(context.Background(), runtime.CreateContainerOptions{
			SessionID: "test",
			Image:     runtime.Image{Name: "img"},
			Args:      []string{"cmd"},
		})
		require.NoError(t, err)
		ac, ok := container.(*apple.Container)
		require.True(t, ok, "container should be *apple.Container type")
		ac.SetReadyCheckTiming(10, 1*time.Millisecond)

		err = container.CopyTo(context.Background(), bytes.NewReader(nil), "/")
		require.NoError(t, err)
		require.Equal(t, 3, readinessAttempts)
	})

	t.Run("returns error when container never becomes ready", func(t *testing.T) {
		runner := &mockRunner{
			runFunc: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
				for _, arg := range args {
					if arg == "true" {
						return errors.New("container not ready")
					}
				}
				return nil
			},
		}
		rt := apple.NewRuntimeWithRunner(runner)
		container, err := rt.CreateContainer(context.Background(), runtime.CreateContainerOptions{
			SessionID: "test",
			Image:     runtime.Image{Name: "img"},
			Args:      []string{"cmd"},
		})
		require.NoError(t, err)
		ac, ok := container.(*apple.Container)
		require.True(t, ok, "container should be *apple.Container type")
		ac.SetReadyCheckTiming(5, 1*time.Millisecond)

		err = container.CopyTo(context.Background(), nil, "/")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to become ready")
	})

	t.Run("returns error on exec tar failure", func(t *testing.T) {
		runner := &mockRunner{
			runFunc: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
				// Fail the exec tar command, but not the readiness check
				for _, arg := range args {
					if arg == "tar" {
						return errors.New("tar failed")
					}
				}
				return nil
			},
		}
		rt := apple.NewRuntimeWithRunner(runner)
		container, err := rt.CreateContainer(context.Background(), runtime.CreateContainerOptions{
			SessionID: "test",
			Image:     runtime.Image{Name: "img"},
			Args:      []string{"cmd"},
		})
		require.NoError(t, err)
		ac, ok := container.(*apple.Container)
		require.True(t, ok, "container should be *apple.Container type")
		ac.SetReadyCheckTiming(10, 1*time.Millisecond)

		err = container.CopyTo(context.Background(), nil, "/")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to copy content to container")
	})
}

func TestContainerStart(t *testing.T) {
	t.Run("is a no-op", func(t *testing.T) {
		runner := &mockRunner{}
		container := createTestContainer(t, runner)

		err := container.Start(context.Background())
		require.NoError(t, err)
		require.Len(t, runner.calls, 0) // No calls made
	})
}

func TestContainerAttach(t *testing.T) {
	t.Run("starts exec with correct args", func(t *testing.T) {
		runner := &mockRunner{}
		container := createTestContainer(t, runner)
		w := &mockWriter{}

		err := container.Attach(context.Background(), func() {}, w)
		require.NoError(t, err)

		require.Len(t, runner.calls, 1)
		call := runner.calls[0]
		require.Equal(t, "container", call.Name)

		// Should use Start (not Run) - check args
		argsStr := strings.Join(call.Args, " ")
		require.Contains(t, argsStr, "exec")
		require.Contains(t, argsStr, "--tty")
		require.Contains(t, argsStr, "--interactive")
		require.Contains(t, argsStr, "--workdir")
		require.Contains(t, argsStr, "/app")
		require.Contains(t, argsStr, "--env")
		require.Contains(t, argsStr, "FOO=bar")
		require.Contains(t, argsStr, "test-session")
		require.Contains(t, argsStr, "echo")
		require.Contains(t, argsStr, "hello")
	})

	t.Run("returns error on exec failure", func(t *testing.T) {
		runner := &mockRunner{
			startFunc: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) (apple.Process, error) {
				return nil, errors.New("exec failed")
			},
		}
		container := createTestContainer(t, runner)
		w := &mockWriter{}

		err := container.Attach(context.Background(), func() {}, w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to exec in container")
	})
}

func TestContainerWait(t *testing.T) {
	t.Run("waits for process and reports exit code 0", func(t *testing.T) {
		runner := &mockRunner{
			startFunc: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) (apple.Process, error) {
				return &mockProcess{exitCode: 0}, nil
			},
		}
		container := createTestContainer(t, runner)
		w := &mockWriter{}

		err := container.Attach(context.Background(), func() {}, w)
		require.NoError(t, err)

		// Use a writer that captures output
		outWriter := &capturingWriter{}
		err = container.Wait(context.Background(), outWriter)
		require.NoError(t, err)
		require.Contains(t, outWriter.output, "Container exited with status: 0")
	})

	t.Run("waits for process and reports non-zero exit code", func(t *testing.T) {
		runner := &mockRunner{
			startFunc: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) (apple.Process, error) {
				return &mockProcess{exitCode: 42}, nil
			},
		}
		container := createTestContainer(t, runner)
		w := &mockWriter{}

		err := container.Attach(context.Background(), func() {}, w)
		require.NoError(t, err)

		outWriter := &capturingWriter{}
		err = container.Wait(context.Background(), outWriter)
		require.NoError(t, err)
		require.Contains(t, outWriter.output, "Container exited with status: 42")
	})

	t.Run("returns nil when no process is running", func(t *testing.T) {
		runner := &mockRunner{}
		container := createTestContainer(t, runner)
		outWriter := &capturingWriter{}

		err := container.Wait(context.Background(), outWriter)
		require.NoError(t, err)
	})

	t.Run("returns error on process error", func(t *testing.T) {
		runner := &mockRunner{
			startFunc: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) (apple.Process, error) {
				return &mockProcess{exitCode: -1, err: errors.New("process crashed")}, nil
			},
		}
		container := createTestContainer(t, runner)
		w := &mockWriter{}

		err := container.Attach(context.Background(), func() {}, w)
		require.NoError(t, err)

		outWriter := &capturingWriter{}
		err = container.Wait(context.Background(), outWriter)
		require.Error(t, err)
		require.Contains(t, err.Error(), "process error in container")
	})
}

func TestContainerForceRemove(t *testing.T) {
	t.Run("removes container successfully", func(t *testing.T) {
		runner := &mockRunner{}
		container := createTestContainer(t, runner)

		err := container.ForceRemove(context.Background())
		require.NoError(t, err)

		require.Len(t, runner.calls, 1)
		call := runner.calls[0]
		require.Equal(t, "container", call.Name)
		require.Contains(t, call.Args, "delete")
		require.Contains(t, call.Args, "--force")
		require.Contains(t, call.Args, "test-session")
	})

	t.Run("returns error on failure", func(t *testing.T) {
		runner := &mockRunner{
			runFunc: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
				for _, arg := range args {
					if arg == "delete" {
						return errors.New("delete failed")
					}
				}
				return nil
			},
		}
		container := createTestContainer(t, runner)

		err := container.ForceRemove(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to force remove container")
	})
}

// capturingWriter captures Printf output for test requireions.
type capturingWriter struct {
	output string
}

func (w *capturingWriter) Print(v ...interface{}) {}
func (w *capturingWriter) Printf(format string, v ...interface{}) {
	w.output += fmt.Sprintf(format, v...)
}
func (w *capturingWriter) Println(v ...interface{}) {}
func (w *capturingWriter) Warning(v ...interface{}) {}
func (w *capturingWriter) Warningf(format string, v ...interface{}) {
}
func (w *capturingWriter) Fatal(v ...interface{})                 {}
func (w *capturingWriter) Fatalf(format string, v ...interface{}) {}
func (w *capturingWriter) GetWriter() io.Writer                   { return &bytes.Buffer{} }
