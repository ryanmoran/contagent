# Matchlock SDK Findings — Phase 0

Source: [`jingkaihe/matchlock`](https://github.com/jingkaihe/matchlock) —
`pkg/sdk/`, `pkg/api/`, and `examples/go/`.

---

## 1. Mount System — `confirmed`

The SDK provides multiple mount methods on `SandboxBuilder` (`pkg/sdk/builder.go`):

| Method | Signature | Write-back? |
|--------|-----------|-------------|
| `MountHostDir` | `(guestPath, hostPath string) *SandboxBuilder` | Yes — direct host FS |
| `MountHostDirReadonly` | `(guestPath, hostPath string) *SandboxBuilder` | No — read-only |
| `MountHostDirAs` | `(guestPath, hostPath string, uid, gid uint32) *SandboxBuilder` | Yes — with UID/GID |
| `MountHostDirReadonlyAs` | `(guestPath, hostPath string, uid, gid uint32) *SandboxBuilder` | No — read-only |
| `MountOverlay` | `(guestPath, hostPath string) *SandboxBuilder` | No — isolated snapshot |
| `MountMemory` | `(guestPath string) *SandboxBuilder` | No — in-memory tmpfs |
| `Mount` | `(guestPath string, cfg MountConfig) *SandboxBuilder` | Depends on config |

`MountConfig` (`pkg/sdk/types.go`):

```go
type MountConfig struct {
    Type     string  `json:"type"`
    HostPath string  `json:"host_path,omitempty"`
    Readonly bool    `json:"readonly,omitempty"`
    OwnerUID *uint32 `json:"owner_uid,omitempty"`
    OwnerGID *uint32 `json:"owner_gid,omitempty"`
}
```

Mount type constants (`pkg/api/config.go`):

```go
const (
    MountTypeMemory  = "memory"
    MountTypeHostFS  = "host_fs"
    MountTypeOverlay = "overlay"
)
```

**Key finding:** `MountHostDir` uses `MountTypeHostFS` — guest writes propagate
back to the host directory. This enables **Topology B** (push-from-host).

contagent usage:

```go
sandbox := sdk.New("agent-image:latest").
    MountHostDir("/workspace", tmpCloneDir)
```

---

## 2. Interactive PTY — `confirmed`

`ExecInteractive` (`pkg/sdk/exec.go`):

```go
func (c *Client) ExecInteractive(
    ctx context.Context,
    command string,
    opts *ExecInteractiveOptions,
) (*ExecInteractiveResult, error)
```

```go
type ExecInteractiveOptions struct {
    WorkingDir string
    User       string
    Rows       uint16
    Cols       uint16
    Stdin      io.Reader
    Stdout     io.Writer
    Resize     <-chan [2]uint16 // channel of [rows, cols] pairs
}

type ExecInteractiveResult struct {
    ExitCode   int
    DurationMS int64
}
```

**Resize mechanism:** The SDK does **not** handle SIGWINCH internally. The caller
provides a `Resize <-chan [2]uint16` channel. The SDK runs `pumpTTYResize`
internally, reading `[rows, cols]` pairs and sending `"exec_tty.resize"` RPCs.

contagent usage (from `examples/go/exec_modes/main.go`):

```go
resizeCh := make(chan [2]uint16, 4)
winchCh := make(chan os.Signal, 1)
signal.Notify(winchCh, syscall.SIGWINCH)

go func() {
    for {
        select {
        case <-stop:
            return
        case <-winchCh:
            c, r, _ := term.GetSize(stdinFD)
            resizeCh <- [2]uint16{uint16(r), uint16(c)}
        }
    }
}()

result, err := client.ExecInteractive(ctx, "sh", &sdk.ExecInteractiveOptions{
    WorkingDir: "/workspace",
    Rows:       uint16(rows),
    Cols:       uint16(cols),
    Stdin:      os.Stdin,
    Stdout:     os.Stdout,
    Resize:     resizeCh,
})
```

---

## 3. Detached + Reattach — `confirmed` (detached) / `cli-only` (reattach from new process)

**Detached mode** is available via `client.Launch()`:

```go
func (c *Client) Launch(b *SandboxBuilder) (string, error)
```

`Launch` calls `Create` with `LaunchEntrypoint = true`, booting the VM in
detached mode and returning the VM ID. After `Launch`, you can call any `Exec*`
method to run commands in the running sandbox — this is the SDK's
"exec into a running sandbox" pattern.

**Reattach from a new `Client` instance** is **not available** in the SDK. The
`Client` struct's `vmID` is set only by `Create`/`Launch`; there is no
`Attach(vmID)` or `Connect(vmID)` method. To reattach from a new process, shell
out to the CLI:

```
matchlock exec <vmID> -it sh
```

contagent usage (within a single session — sufficient for our use case):

```go
vmID, err := client.Launch(sandbox)
// ... later ...
result, err := client.ExecInteractive(ctx, "claude", &sdk.ExecInteractiveOptions{
    WorkingDir: "/workspace",
    Stdin:      os.Stdin,
    Stdout:     os.Stdout,
    Resize:     resizeCh,
})
```

---

## 4. Port Forward — `confirmed`

Available both at build time and runtime (`pkg/sdk/builder.go`,
`pkg/sdk/port_forward.go`):

**Builder (at create time):**

```go
func (b *SandboxBuilder) WithPortForward(localPort, remotePort int) *SandboxBuilder
func (b *SandboxBuilder) WithPortForwardAddresses(addresses ...string) *SandboxBuilder
```

**Runtime (after launch):**

```go
func (c *Client) PortForward(ctx context.Context, specs ...string) ([]api.PortForwardBinding, error)
func (c *Client) PortForwardWithAddresses(ctx context.Context, addresses []string, specs ...string) ([]api.PortForwardBinding, error)
```

Types (`pkg/api/port_forward.go`):

```go
type PortForwardBinding struct {
    Address    string `json:"address"`
    LocalPort  int    `json:"local_port"`
    RemotePort int    `json:"remote_port"`
}
```

Default bind address is `127.0.0.1`.

contagent usage:

```go
bindings, err := client.PortForward(ctx, "8080:8080")
```

---

## 5. Lifecycle — `confirmed`

### `client.Launch(b *SandboxBuilder) (string, error)`

- Calls `Create` with `LaunchEntrypoint = true`
- Boots the microVM with configured image, resources, network, and mounts
- Starts the image's ENTRYPOINT/CMD in detached mode
- Returns the VM ID

### `client.Close(timeout time.Duration) error`

- Idempotent (checks `c.closed` flag)
- Clears VFS hooks (`setVFSHooks(nil, nil, nil)`)
- Stops the network hook Unix socket server
- Defaults timeout to 2 seconds if `<= 0`
- Sends a `"close"` RPC with the timeout
- Closes stdin pipe to the RPC subprocess
- Waits for subprocess exit; kills it if timeout expires
- Returns `ErrCloseTimeout` if the process had to be killed

### `client.Remove() error`

- No-op if `vmID` is empty
- Runs `matchlock rm <vmID>` (CLI invocation)
- Deletes the VM's state directory on disk (includes overlay layers)

**Note:** `Remove` does not explicitly document dropping host-FS mounts because
those are just bind mounts — unmounted when the VM stops. The host temp directory
remains and must be cleaned up separately by the caller.

contagent usage:

```go
vmID, err := client.Launch(sandbox)
if err != nil { return err }
defer client.Remove()
defer client.Close(0)

// ... run commands, sync out ...
```

---

## 6. Network Allowlist — `confirmed`

### Builder-time (before launch)

```go
func (b *SandboxBuilder) AllowHost(hosts ...string) *SandboxBuilder
func (b *SandboxBuilder) AddHost(host, ip string) *SandboxBuilder
func (b *SandboxBuilder) AddSecret(name, value string, hosts ...string) *SandboxBuilder
func (b *SandboxBuilder) AddSecretWithPlaceholder(name, value, placeholder string, hosts ...string) *SandboxBuilder
```

### Runtime mutation (after launch) — `pkg/sdk/allow_list.go`

```go
func (c *Client) AllowListAdd(ctx context.Context, hosts ...string) (*AllowListUpdate, error)
func (c *Client) AllowListDelete(ctx context.Context, hosts ...string) (*AllowListUpdate, error)
```

```go
type AllowListUpdate struct {
    Added        []string
    Removed      []string
    AllowedHosts []string
}
```

**The allowlist can be mutated after launch.** `AllowListAdd`/`AllowListDelete`
send RPCs to the running VM. Requires `WithNetworkInterception()` on the builder
to enable runtime mutation.

Related types:

```go
// pkg/api/config.go
type Secret struct {
    Name        string
    Value       string
    Placeholder string
    Hosts       []string
}

type HostIPMapping struct {
    Host string `json:"host"`
    IP   string `json:"ip"`
}
```

contagent usage:

```go
sandbox := sdk.New("agent-image:latest").
    AllowHost("api.anthropic.com").
    AddHost("git-server", hostIP).
    AllowHost("git-server").
    AddSecret("ANTHROPIC_API_KEY", apiKey, "api.anthropic.com")
```

---

## 7. Build Path — `cli-only` (Dockerfile build) / `confirmed` (run from OCI image)

**Running from an OCI image** is straightforward in the SDK:

```go
sandbox := sdk.New("python:3.12-alpine")
sandbox := sdk.New("alpine:latest")
sandbox := sdk.New("myapp:latest")  // pre-built custom image
```

The `CreateOptions.Image` field takes any OCI image reference. The matchlock
daemon handles pulling and caching transparently.

**Building from a Dockerfile** requires the CLI:

```bash
matchlock build -f Dockerfile -t myapp:latest .
```

**Importing from Docker:**

```bash
docker save myapp:latest | matchlock image import myapp:latest
```

**Image cache location:** `~/.cache/matchlock/images/`

Cache management:

```bash
matchlock image ls
matchlock image rm <image>
matchlock build <image>   # pre-cache from registry for faster startup
```

contagent usage:

```go
// Build step (shell out to CLI)
cmd := exec.Command("matchlock", "build", "-f", "Dockerfile", "-t", imageName, ".")
if err := cmd.Run(); err != nil { return err }

// Then use the built image in the SDK
sandbox := sdk.New(imageName)
```

---

## Recommendations

### Topology: Use Topology B (push-from-host)

`MountHostDir` provides writable mounts with full host write-back propagation.
This enables Topology B, which is the stronger security posture:

- The VM stays **fully network-locked** except `api.anthropic.com` — no git
  server reachable from the sandbox.
- Git credentials and push operations remain entirely on the host side.
- The agent commits inside the VM; writes propagate through the mount; contagent
  pushes from the host after the session ends.

### Features requiring CLI shell-out

| Feature | Mechanism |
|---------|-----------|
| Dockerfile build | `matchlock build -f Dockerfile -t <tag> .` |
| Docker image import | `docker save <img> \| matchlock image import <img>` |
| Image cache management | `matchlock image ls`, `matchlock image rm` |
| VM removal | `client.Remove()` already shells out to `matchlock rm` internally |
| Cross-process reattach | `matchlock exec <vmID> -it sh` (not needed for contagent's single-session model) |

Everything else (launch, exec, interactive PTY, mounts, port-forward, network
allowlist, secrets, lifecycle) is available directly in the Go SDK.

### SDK gaps and surprises

1. **No cross-process reattach in SDK.** `Client` has no `Attach(vmID)` method.
   This is irrelevant for contagent's single-process model but would matter if
   we ever wanted a daemon mode.

2. **`Remove()` shells out internally.** `client.Remove()` runs
   `matchlock rm <vmID>` as a subprocess rather than using an RPC. This means
   the `matchlock` CLI binary must be on `$PATH` even when using the SDK.

3. **Runtime allowlist requires `WithNetworkInterception()`.** The builder must
   call `WithNetworkInterception()` to enable `AllowListAdd`/`AllowListDelete`
   after launch. Without it, the allowlist is frozen at create time.

4. **Host-FS mount cleanup is caller's responsibility.** `Remove()` cleans up
   overlay layers and VM state, but bind-mounted host directories are not
   deleted. contagent must clean up the temp clone directory itself (which aligns
   with the plan's conditional-cleanup requirement).

5. **SIGWINCH is caller-managed.** The SDK provides a clean `Resize` channel
   interface but does not listen for signals itself. contagent's existing
   signal-handling code maps directly to this pattern.

6. **`Close` timeout defaults to 2s.** If the VM process is slow to exit,
   `Close(0)` will use a 2-second timeout and then kill the subprocess. Consider
   using a longer timeout for interactive agent sessions.
