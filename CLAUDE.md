# AGENTS.md

**contagent** is a Go CLI that starts a Docker container, runs a local Git HTTP
server on the host, copies the current Git repository into the container with
the remote configured to point back to the host, executes a command inside the
container with TTY support, and cleans up afterward.

## Commands

```bash
go build -o contagent .
go test ./...
go run . [command] [args...]
```

## Code Organization

```
/app/
├── main.go                    # Entry point - orchestrates the entire flow
└── internal/
    ├── cleanup.go             # Cleanup manager for resource cleanup
    ├── config.go              # CLI argument and environment variable parsing
    ├── session.go             # Session ID and branch name generation
    ├── types.go               # Shared types
    ├── writer.go              # I/O writer abstraction
    ├── docker/
    │   ├── client.go          # Docker client initialization and image building
    │   ├── container.go       # Container lifecycle
    │   ├── interface.go       # Docker client interface
    │   └── tty.go             # TTY size monitoring and resizing
    └── git/
        ├── server.go          # Git HTTP server using git-http-backend CGI
        └── archive.go         # Creates tar archive of Git repo for copying to container
```

## Main Flow

1. Initialize cleanup manager → Parse config → Generate session ID
2. Start Git HTTP server on random port → Create Docker client
3. Build image → Create container → Create Git archive with remote configured
4. Copy archive to container → Start container → Attach TTY
5. Wait for exit/signal → Cleanup (remove container, close Git server, close client)

## Configuration

- **Args**: Command and arguments to run in container
- **Env**: Passes through `TERM`, `COLORTERM`, `ANTHROPIC_API_KEY`
- **Session**: Random ID (e.g., `contagent-1234`) for container name and branch (e.g., `contagent/1234`)

## Important Details

### Docker Networking

Container configured with `ExtraHosts: []string{"host.docker.internal:host-gateway"}` so container can reach host Git server at `http://host.docker.internal:<port>`.

### Git Server

- Listens on `127.0.0.1:0` (random port)
- Requires `GIT_HTTP_EXPORT_ALL=true` and `GIT_HTTP_ALLOW_PUSH=true`

### Git Archive

- Checks out HEAD (ignores uncommitted changes)
- Creates new branch for session
- Configures remote to point to host server
- Archives `.git` and all tracked files

### TTY Handling

- Terminal set to raw mode
- Container resized on SIGWINCH
- Initial resize retries up to 10 times if container not ready

### Container Lifecycle

- Signal handling (SIGINT, SIGTERM) triggers context cancellation
- Cleanup manager ensures resources are freed in reverse order of allocation
- Container forcibly removed after execution

## Dependencies

- `github.com/moby/moby/client` - Docker client
- `github.com/docker/cli/cli/streams` - Terminal stream handling
- `github.com/moby/term` - Terminal utilities
- `github.com/stretchr/testify` - Testing assertions

## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Auto-syncs to JSONL for version control
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**

```bash
bd ready --json
```

**Create new issues:**

```bash
bd create "Issue title" -t bug|feature|task -p 0-4 --json
bd create "Issue title" -p 1 --deps discovered-from:bd-123 --json
bd create "Subtask" --parent <epic-id> --json  # Hierarchical subtask (gets ID like epic-id.1)
```

**Claim and update:**

```bash
bd update bd-42 --status in_progress --json
bd close bd-42 --reason "Completed" --json
```

**Complete work:**

```bash
bd close bd-42 --reason "Completed" --json
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Claim your task**: `bd update <id> --status in_progress`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" -p 1 --deps discovered-from:<parent-id>`
5. **Complete**: `bd close <id> --reason "Done"`
6. **Commit together**: Always commit the `.beads/issues.jsonl` file together with the code changes so issue state stays in sync with code state

### CLI Help

Run `bd <command> --help` to see all available flags for any command.
For example: `bd create --help` shows `--parent`, `--deps`, `--assignee`, etc.
