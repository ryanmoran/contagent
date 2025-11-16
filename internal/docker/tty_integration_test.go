//go:build integration
// +build integration

package docker_test

import (
	"testing"
)

// TTY functionality is tested via container attach tests in container_test.go
// since TTY is an internal implementation detail of the Attach() method.
//
// Key TTY behaviors tested through container attach:
// - Resize handling with real containers
// - Monitoring TTY size changes
// - Signal handling (SIGWINCH)
// - Retry logic for initial resize
//
// See TestContainerAttach in container_test.go for integration tests.

func TestTTYDocumentation(t *testing.T) {
	t.Run("TTY is tested via Container.Attach", func(t *testing.T) {
		t.Log("TTY resize, monitoring, and retry logic are tested through container attach tests")
		t.Log("See container_test.go for integration tests that cover TTY behavior")
	})
}
