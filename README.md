# contagent

A tool that provides isolated, containerized development environments with
seamless Git integration. Run commands in a fresh Docker container with
automatic repository setup and cleanup.

## Features

- 🐳 **Automatic Container Management**: Spins up isolated Docker containers on-demand
- 🔄 **Git Integration**: Clones your repository into the container with bidirectional sync via HTTP server
- 🖥️ **Full TTY Support**: Interactive terminal with proper size handling and signal forwarding
- 🧹 **Automatic Cleanup**: Removes containers and stops servers after execution
- 🔐 **SSH Agent & Docker Socket Forwarding**: Access host SSH keys and Docker daemon from container

## Requirements

- **Docker**: Must be installed and running
- **Git**: Required for repository operations
- **Go**: Version 1.21+ for building from source

## Installation

### From Source

```bash
git clone https://github.com/ryanmoran/contagent.git
cd contagent
go install .
```

### Running Directly

```bash
go run . [command] [args...]
```

## Quick Start

### Basic Usage

Start a shell in a containerized environment:

```bash
contagent /bin/bash
```

Run a specific command:

```bash
contagent go test ./...
```

Execute an AI agent session:

```bash
contagent claude
```

### With Custom Environment Variables

```bash
contagent --env CUSTOM_VAR=value --env ANOTHER=test claude
```

### With Volume Mounts

```bash
contagent --volume /host/path:/container/path /bin/bash
```

## How It Works

`contagent` orchestrates isolated development environments:

1. **Session Initialization**: Generates a unique session ID (e.g., `contagent-1234`) and branch name (e.g., `contagent/1234`)

2. **Git HTTP Server**: Starts a local Git server on the host (random port) to allow the container to pull from and push to your repository

3. **Docker Image**: Builds a container image from the specified Dockerfile

4. **Repository Setup**: Creates a tar archive of your Git repository and copies it into the container at `/app` with:
   - A new branch for the session
   - Remote configured to point back to the host Git server
   - All tracked files from HEAD (uncommitted changes are ignored)

5. **Container Execution**: Runs your command with:
   - Full TTY support (interactive terminal)
   - Automatic terminal resizing
   - Signal forwarding (Ctrl+C, etc.)
   - Access to host Docker socket and SSH agent

6. **Cleanup**: After execution or interruption:
   - Container is removed (10 second graceful shutdown timeout)
   - Git server is stopped
   - All resources are cleaned up

### Architecture

```
Host Machine                          Container (contagent-1234)
├── Git Repository                    ├── /app (cloned repo)
├── Git HTTP Server (random port) ←──┤    └── origin → host.docker.internal:port
├── Docker Socket ────────────────────┤ /var/run/docker.sock
└── SSH Agent ────────────────────────┤ /run/host-services/ssh-auth.sock
```

## Configuration

### Command-Line Flags

- `--env KEY=VALUE`: Add environment variable to container (can be used multiple times)
- `--volume HOST:CONTAINER`: Mount host directory into container (can be used multiple times)

### Environment Variables

The following environment variables are automatically passed through to the container:

- `TERM`: Terminal type (defaults to `xterm-256color`)
- `COLORTERM`: Color support (defaults to `truecolor`)
- `ANTHROPIC_API_KEY`: API key for AI agent
- `SSH_AUTH_SOCK`: SSH agent socket (automatically configured)

### Default Volume Mounts

- `/var/run/docker.sock:/var/run/docker.sock`: Docker socket for Docker-in-Docker
- `/run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock`: SSH agent access

### Container Settings

- **Image Name**: `contagent:latest`
- **Working Directory**: `/app`
- **Stop Timeout**: 10 seconds
- **TTY Resize Retries**: 10 attempts with exponential backoff (10ms, 20ms, 30ms, ...)

## Development

### Building

```bash
go build -o contagent .
```

### Running Tests

Unit tests:

```bash
go test ./...
```

Integration tests (requires Docker):

```bash
go test -tags integration ./...
```

### Docker Networking

The container is configured with `host.docker.internal:host-gateway` extra host mapping, allowing it to reach the host Git server at `http://host.docker.internal:<port>`.

### Git Server Details

- Listens on `127.0.0.1:0` (random available port)
- Uses `git-http-backend` CGI for Git protocol support
- Requires `GIT_HTTP_EXPORT_ALL=true` to serve repositories
- Requires `GIT_HTTP_ALLOW_PUSH=true` to accept pushes from container

### Signal Handling

- Gracefully handles `SIGINT` (Ctrl+C) and `SIGTERM`
- Triggers container shutdown with 10 second timeout
- Ensures cleanup even on interruption
- Terminal restored to normal mode on exit

## Use Cases

### AI-Assisted Development

Run AI agents in an isolated environment with full repository access:

```bash
contagent claude
```

The container has access to:

- Your repository code and history
- Docker (via socket mount) for running tests/builds
- SSH agent for authenticated Git operations

### Reproducible Testing

Test your code in a clean environment:

```bash
contagent go test ./...
```

### Isolated Experiments

Try risky operations without affecting your local environment:

```bash
contagent /bin/bash
# Inside container: experiment freely
# Changes are isolated to the container
```

### CI/CD Development

Test CI workflows locally in the same container environment:

```bash
contagent /bin/bash -c "go build && go test && go vet"
```

## Troubleshooting

## License

MIT License - see LICENSE file for details

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Run the test suite: `go test ./...`
5. Submit a pull request
