# Phase 1 — Walking skeleton: Matchlock runtime

## Goal

Implement a new `matchlock` runtime so that `contagent <cmd>` can boot a
Matchlock microVM, run `<cmd>`, stream stdout/stderr, propagate the exit code,
and tear down the VM. This is the walking skeleton — no git transport, no
secrets, no network policy. Those come in later phases.

**Acceptance criteria for the entire phase:**
`contagent --runtime matchlock echo hello` prints `hello` and exits 0. The VM
is gone afterward (verify via `matchlock list`).

## Issues

Work is tracked as individual files in `issues/`. Each file has a `Status`
field at the top: `open`, `in_progress`, or `done`. Update the status when
you claim or complete an issue, and commit the status change with your code.

| Issue | Summary | Depends on |
|-------|---------|------------|
| [P1-1](issues/P1-1.md) | Add Matchlock SDK dependency and create package skeleton | — |
| [P1-2](issues/P1-2.md) | Implement image building via CLI | P1-1 |
| [P1-3](issues/P1-3.md) | Implement sandbox lifecycle (create, launch, teardown) | P1-1 |
| [P1-4](issues/P1-4.md) | Implement InspectUser and stub CopyTo | P1-3 |
| [P1-5](issues/P1-5.md) | Implement command execution and output streaming | P1-3 |
| [P1-6](issues/P1-6.md) | Wire matchlock runtime into config and main.go | P1-2, P1-3, P1-4, P1-5 |
| [P1-7](issues/P1-7.md) | End-to-end validation and cleanup | P1-6 |

To find ready work, look for issues with `Status: open` whose dependencies are
all `Status: done`.

## Context

Read these files before starting any issue:

- `PLAN.md` — Full rewrite plan. Phase 1 is the walking skeleton.
- `docs/matchlock-sdk-findings.md` — Phase 0 findings. Contains exact SDK
  symbols and signatures.
- `internal/runtime/runtime.go` — The `Runtime` and `Container` interfaces
  that must be implemented.
- `internal/apple/` — Reference implementation. The apple runtime follows a
  similar CLI-shelling pattern and is a good model for structure and testing.

### Key SDK symbols (from Phase 0 findings)

```go
// Builder
sdk.New(image string) *SandboxBuilder
builder.MountHostDir(guestPath, hostPath string) *SandboxBuilder

// Client lifecycle
client.Launch(b *SandboxBuilder) (string, error)
client.Close(timeout time.Duration) error
client.Remove() error

// Execution
client.Exec(ctx, command string, opts *ExecOptions) (*ExecResult, error)
client.ExecStream(ctx, command string, opts *ExecStreamOptions) (*ExecResult, error)
client.ExecInteractive(ctx, command string, opts *ExecInteractiveOptions) (*ExecInteractiveResult, error)

type ExecOptions struct {
    WorkingDir string
    User       string
}
type ExecStreamOptions struct {
    WorkingDir string
    User       string
    Stdout     io.Writer
    Stderr     io.Writer
    Stdin      io.Reader
}
type ExecResult struct {
    ExitCode   int
    Stdout     string
    Stderr     string
    DurationMS int64
}
```

### Architecture decision

The matchlock runtime follows the same `Runtime` + `Container` interface pattern
as the docker and apple runtimes. The matchlock `Container` implementation holds
an `sdk.Client` and manages the VM lifecycle through the SDK methods above.

The apple runtime is the closest analog: it also creates a long-running sandbox
first, then execs commands into it. Use it as a structural reference.

**Key difference from docker/apple:** The matchlock runtime uses `MountHostDir`
to mount the repo into the VM instead of copying a tar archive via `CopyTo`.
This means `CopyTo` is a no-op for matchlock. The mount is configured during
`CreateContainer` and the repo enters the VM automatically when the sandbox
launches.

## Ground rules

- **Do not modify the `Runtime` or `Container` interfaces.** The matchlock
  runtime must conform to them as-is. If something doesn't fit, note it as a
  follow-up issue but make it work within the current interface.
- **Follow existing patterns.** The apple runtime (`internal/apple/`) is the
  structural reference. Match its conventions for error messages, compile-time
  checks, test structure, and command execution.
- **No git transport in this phase.** The git server, archive creation, and
  branch management are reused from existing code in `main.go`. Phase 1 only
  adds the matchlock runtime — the existing git flow calls `CopyTo` which is a
  no-op for matchlock.
- **No secrets injection in this phase.** Pass `ANTHROPIC_API_KEY` as a plain
  env var for now. Phase 4 replaces this with `AddSecret`.
- **No network policy in this phase.** The sandbox runs with default networking.
  Phase 4 adds `AllowHost` and default-deny egress.
- **All new code must be based off the `v2` branch.** When creating pull
  requests, set `v2` as the base branch.
- **Update issue status** when claiming or completing work. Commit the status
  change with the code so tracking stays in sync with the implementation.
