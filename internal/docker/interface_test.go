package docker_test

import (
	"github.com/moby/moby/client"
	"github.com/ryanmoran/contagent/internal/docker"
)

// Compile-time check that *client.Client implements DockerClient interface
var _ docker.DockerClient = (*client.Client)(nil)
