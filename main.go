package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ryanmoran/contagent/internal"
	"github.com/ryanmoran/contagent/internal/docker"
	"github.com/ryanmoran/contagent/internal/git"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic occurred: %v", r)
			os.Exit(1)
		}
	}()

	if err := run(os.Args, os.Environ()); err != nil {
		log.Fatal(err)
	}
}

func run(args, env []string) error {
	cleanupMgr := internal.NewCleanupManager()
	defer cleanupMgr.Execute()

	config := internal.ParseConfig(args[1:], env)

	// Create context with cancellation for proper goroutine cleanup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals to cancel context and cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	w := internal.NewStandardWriter()

	session := internal.GenerateSession()

	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w\nThis is a system error - check file system permissions", err)
	}

	remote, err := git.NewServer(workingDirectory, w)
	if err != nil {
		return fmt.Errorf("failed to start git server in directory %q: %w", workingDirectory, err)
	}
	cleanupMgr.Add("git-server", remote.Close)

	client, err := docker.NewDefaultClient()
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w\nMake sure Docker is installed and running (try 'docker ps')", err)
	}
	cleanupMgr.Add("docker-client", func() error {
		client.Close()
		return nil
	})

	image, err := client.BuildImage(ctx, config.DockerfilePath, config.ImageName, w)
	if err != nil {
		return fmt.Errorf("failed to build docker image %q from %q: %w", config.ImageName, config.DockerfilePath, err)
	}

	container, err := client.CreateContainer(
		ctx,
		session.ID(),
		image,
		config.Args,
		config.Env,
		config.Volumes,
		config.WorkingDir,
		config.StopTimeout,
		config.TTYRetries,
		config.RetryDelay,
	)
	if err != nil {
		return fmt.Errorf("failed to create container %q from image %q: %w", session.ID(), image.Name, err)
	}
	cleanupMgr.Add("container", func() error {
		return container.ForceRemove(ctx)
	})

	archive, err := git.CreateArchive(
		workingDirectory,
		fmt.Sprintf("http://host.docker.internal:%d", remote.Port()),
		session.Branch(),
		config.GitUser.Name,
		config.GitUser.Email,
		w,
	)
	if err != nil {
		return fmt.Errorf("failed to create git archive from %q on branch %q: %w", workingDirectory, session.Branch(), err)
	}
	cleanupMgr.Add("archive", archive.Close)

	err = container.CopyTo(ctx, archive, "/")
	if err != nil {
		return fmt.Errorf("failed to copy git archive to container %q: %w", session.ID(), err)
	}

	err = container.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start container %q: %w", session.ID(), err)
	}

	err = container.Attach(ctx, w)
	if err != nil {
		return fmt.Errorf("failed to attach to container %q: %w\nThis may indicate a TTY configuration issue", session.ID(), err)
	}

	err = container.Wait(ctx, w)
	if err != nil {
		return fmt.Errorf("failed to wait for container %q: %w", session.ID(), err)
	}

	return nil
}
