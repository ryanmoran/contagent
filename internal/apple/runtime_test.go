package apple_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/ryanmoran/contagent/internal/apple"
	"github.com/ryanmoran/contagent/internal/runtime"
	"github.com/stretchr/testify/require"
)

// mockRunner implements apple.CommandRunner for testing.
type mockRunner struct {
	runFunc   func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error
	startFunc func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) (apple.Process, error)
	calls     []mockCall
}

type mockCall struct {
	Name string
	Args []string
}

func (m *mockRunner) Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
	m.calls = append(m.calls, mockCall{Name: name, Args: args})
	if m.runFunc != nil {
		return m.runFunc(ctx, stdin, stdout, stderr, name, args...)
	}
	return nil
}

func (m *mockRunner) Start(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) (apple.Process, error) {
	m.calls = append(m.calls, mockCall{Name: name, Args: args})
	if m.startFunc != nil {
		return m.startFunc(ctx, stdin, stdout, stderr, name, args...)
	}
	return &mockProcess{exitCode: 0}, nil
}

// mockProcess implements apple.Process for testing.
type mockProcess struct {
	exitCode int
	err      error
	waitCh   chan struct{} // if set, Wait blocks until closed
}

func (p *mockProcess) Wait() (int, error) {
	if p.waitCh != nil {
		<-p.waitCh
	}
	return p.exitCode, p.err
}

// mockWriter implements internal.Writer for testing.
type mockWriter struct {
	buf bytes.Buffer
}

func (w *mockWriter) Print(v ...interface{})                 { /* no-op */ }
func (w *mockWriter) Printf(format string, v ...interface{}) { /* no-op */ }
func (w *mockWriter) Println(v ...interface{})               { /* no-op */ }
func (w *mockWriter) Warning(v ...interface{})               { /* no-op */ }
func (w *mockWriter) Warningf(format string, v ...interface{}) {
	/* no-op */
}
func (w *mockWriter) Fatal(v ...interface{})                 { /* no-op */ }
func (w *mockWriter) Fatalf(format string, v ...interface{}) { /* no-op */ }
func (w *mockWriter) GetWriter() io.Writer                   { return &w.buf }

func TestRuntimeBuildImage(t *testing.T) {
	t.Run("builds image successfully", func(t *testing.T) {
		runner := &mockRunner{}
		rt := apple.NewRuntimeWithRunner(runner)
		w := &mockWriter{}

		image, err := rt.BuildImage(context.Background(), "./Dockerfile", "myimage:latest", w)
		require.NoError(t, err)
		require.Equal(t, "myimage:latest", image.Name)
		require.Len(t, runner.calls, 1)
		require.Equal(t, "container", runner.calls[0].Name)
		require.Contains(t, runner.calls[0].Args, "build")
		require.Contains(t, runner.calls[0].Args, "--tag")
		require.Contains(t, runner.calls[0].Args, "myimage:latest")
		require.Contains(t, runner.calls[0].Args, "--file")
		require.Contains(t, runner.calls[0].Args, "./Dockerfile")
	})

	t.Run("returns error on build failure", func(t *testing.T) {
		runner := &mockRunner{
			runFunc: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
				return errors.New("build failed")
			},
		}
		rt := apple.NewRuntimeWithRunner(runner)
		w := &mockWriter{}

		_, err := rt.BuildImage(context.Background(), "./Dockerfile", "myimage:latest", w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to build image")
	})
}

func TestRuntimeCreateContainer(t *testing.T) {
	t.Run("creates container with correct args", func(t *testing.T) {
		runner := &mockRunner{}
		rt := apple.NewRuntimeWithRunner(runner)

		container, err := rt.CreateContainer(context.Background(), runtime.CreateContainerOptions{
			SessionID:   "test-session",
			Image:       runtime.Image{Name: "myimage:latest"},
			Args:        []string{"echo", "hello"},
			Env:         []string{"FOO=bar", "BAZ=qux"},
			Volumes:     []string{"/host:/container"},
			WorkingDir:  "/app",
			StopTimeout: 10,
		})
		require.NoError(t, err)
		require.NotNil(t, container)

		require.Len(t, runner.calls, 1)
		call := runner.calls[0]
		require.Equal(t, "container", call.Name)
		require.Contains(t, call.Args, "create")
		require.Contains(t, call.Args, "--name")
		require.Contains(t, call.Args, "test-session")
		require.Contains(t, call.Args, "--ssh")
		require.Contains(t, call.Args, "--env")
		require.Contains(t, call.Args, "FOO=bar")
		require.Contains(t, call.Args, "BAZ=qux")
		require.Contains(t, call.Args, "--volume")
		require.Contains(t, call.Args, "/host:/container")
		require.Contains(t, call.Args, "--workdir")
		require.Contains(t, call.Args, "/app")
		require.Contains(t, call.Args, "myimage:latest")
		require.Contains(t, call.Args, "sleep")
		require.Contains(t, call.Args, "infinity")
	})

	t.Run("returns error on create failure", func(t *testing.T) {
		runner := &mockRunner{
			runFunc: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
				return errors.New("create failed")
			},
		}
		rt := apple.NewRuntimeWithRunner(runner)

		_, err := rt.CreateContainer(context.Background(), runtime.CreateContainerOptions{
			SessionID: "test-session",
			Image:     runtime.Image{Name: "myimage:latest"},
			Args:      []string{"echo"},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to create container")
	})
}

func TestRuntimeHostAddress(t *testing.T) {
	rt := apple.NewRuntime()
	require.Equal(t, "host.container.internal", rt.HostAddress())
}

func TestRuntimeClose(t *testing.T) {
	rt := apple.NewRuntime()
	err := rt.Close()
	require.NoError(t, err)
}
