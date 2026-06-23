# contagent → Matchlock microVM runtime: implementation plan

## Purpose

Reimplement [contagent](https://github.com/ryanmoran/contagent) so that its
runtime layer is **microVMs via the [Matchlock](https://github.com/jingkaihe/matchlock)
Go SDK** instead of Docker containers. The user-facing behavior (run an AI agent
or arbitrary command against the current repo in a disposable, isolated
environment) stays the same; the isolation boundary moves from a shared-kernel
container to a hardware-virtualized microVM.

Scope is **local development only**. No cloud / remote execution. Do not add a
provider-abstraction or control-plane layer for remote hosts.

This document is written to be executed by Claude Code. **Phase 0 is a
verification phase — do it first and do not write runtime code against SDK
methods you have not confirmed exist.**

---

## Why we're doing this

The headline use case is `contagent claude` — running an AI coding agent against
a repo. A container is a resource-isolation mechanism, not a security boundary:
the agent shares the host kernel, and a confused or prompt-injected agent's blast
radius is the host. A microVM gives kernel- and hardware-level separation. The
rewrite is only worth it if we *don't* reintroduce holes through that boundary
(see "What changes or drops").

---

## Guiding principles / invariants

1. **The VM boundary is the product.** Never punch a hole through it for
   convenience. No host Docker socket forwarding, no broad host filesystem
   exposure, no secrets materialized inside the VM.
2. **Secrets stay on the host.** The Anthropic API key (and any other
   credential) must never exist as plaintext inside the VM. Use Matchlock's
   secret-injection model.
3. **The durable artifact is a pushed git branch**, never the VM's filesystem.
   Everything in the VM is disposable.
4. **Default-deny egress.** The sandbox reaches only an explicit allowlist.
5. **Reuse contagent's existing host-side logic** (config resolution, session
   branch naming, signal/TTY handling, the git server) wherever possible; only
   the runtime swaps.

---

## Phase 0 — Verify the Matchlock SDK surface (DO THIS FIRST)

Read `pkg/sdk` and `examples/go` in the Matchlock repo and produce a short
findings note (`docs/matchlock-sdk-findings.md`) answering the questions below.
The answers determine the repo-transport topology and how much we build vs.
shell out to the `matchlock` CLI.

**Questions that must be answered before Phase 2:**

1. **Mount system.** Does the SDK let you mount an **arbitrary host directory**
   into the VM at a chosen path? Get the exact builder/method name and
   signature. Critically: is the mount **writable in-guest**, and do guest
   writes **propagate back to the host directory**, or is it an isolated overlay
   (writes stay in the VM)? This single answer selects topology A vs. B in
   Phase 2.
2. **Interactive PTY.** Confirm `ExecInteractive` (or equivalent), its exact
   signature, and **how terminal resize is surfaced** (so we can forward
   SIGWINCH / window-size changes).
3. **Detached + reattach.** Are "run detached" and "exec into a running
   sandbox" available **in the Go SDK**, or only via the `matchlock` CLI? If
   CLI-only, we shell out.
4. **Port-forward.** SDK or CLI-only? Same decision.
5. **Lifecycle.** Exact semantics of `client.Launch`, `client.Close(code)`,
   `client.Remove()` — what each cleans up, and whether `Remove` also drops
   mounted volumes / overlays.
6. **Network allowlist.** Confirm `AllowHost`, `AddHost(name, ip)`,
   `AddSecret(name, value, host)` signatures, and whether the allowlist can be
   mutated **after** launch (runtime add/delete) or only at build time.
7. **Build path.** How to produce/run a VM from a Dockerfile or OCI image
   (BuildKit-in-VM via the CLI vs. a pre-baked rootfs), and where image caching
   lives.

**Acceptance:** `docs/matchlock-sdk-findings.md` exists and answers all seven,
with the exact Go symbols (names + signatures) we will call, each tagged
`confirmed` or `cli-only` or `not-available`.

---

## Inherited feature set (confirm against current contagent source)

These are contagent's current features as understood from its README. Confirm
each against the actual source before mapping it; correct this list if it drifts.

- On-demand disposable isolated environment for a command or agent.
- `contagent claude` and `contagent <arbitrary command>`.
- Git integration: the repo is made available inside the environment, work
  happens on a session branch, and changes sync back to the host via a git
  server contagent runs.
- Full TTY support and host→guest signal forwarding.
- Automatic cleanup on exit.
- SSH agent forwarding and Docker socket forwarding.
- Build environment from a Dockerfile.
- Configuration via YAML and flags.
- `ANTHROPIC_API_KEY` made available to the agent.

---

## Architecture overview

- **Host orchestrator (Go, existing contagent process):** owns VM lifecycle,
  config resolution, session-branch naming (`contagent/<id>`), the git HTTP
  server, secret handling, and the egress allowlist.
- **microVM (Matchlock):** runs the OCI image rootfs and the command/agent.
- **Two independent channels:**
  - **In:** the repo enters the VM via a **mount** of a host temp directory.
  - **Out:** the agent's commits leave the VM as a **git push** of the session
    branch (topology A), or via mount write-back + host push (topology B).
- **Secrets:** injected by Matchlock so the VM sees only a placeholder; the real
  key is bound to `api.anthropic.com` on the host side.

---

## Repo transport (the heart of the rewrite)

Baseline design supplied by the project owner; both topologies share the same
seed-in step and differ only in how work comes back out. **Topology selection
depends on Phase 0 question 1.**

### Shared seed-in steps (both topologies)

1. On invocation, create a host **temp directory**.
2. Populate it with a **self-contained clone** of the current repo. Decide the
   dirty-tree policy explicitly:
   - `git clone --local <repo> <tmp>` — fast (hardlinked objects) but **drops
     uncommitted/untracked changes**.
   - `cp -a` of the working tree — **includes WIP**, slower, heavier `.git`.
   - Pick one as the default; consider a flag to switch. **Do not use
     `git worktree`** — its `.git` is a file pointing outside the mounted dir
     and breaks across the VM boundary.
3. Configure a remote in the temp clone pointing at contagent's git server URL
   (the address that will be reachable from inside the VM).
4. Launch the VM and **mount the temp dir** at the workspace path.
5. Inside the VM, create and check out the session branch `contagent/<id>`.

### Topology A — push-from-inside (works even if the mount is overlay-only)

Use this if Phase 0 finds the mount is **not** writable-with-host-propagation.

6A. Start contagent's git HTTP server bound to a host-reachable address and
    `AllowHost` / `AddHost` it so the VM can reach **only** that endpoint (plus
    `api.anthropic.com`).
7A. The agent works; on completion it runs `git push origin contagent/<id>`
    **from inside the VM**, over the network, to contagent's git server.
8A. The git server serves the **canonical** host repo, so the branch lands
    there. Because the canonical repo is non-bare with a checked-out branch,
    push a **different** branch (`contagent/<id>`, never `HEAD`) to avoid
    `receive.denyCurrentBranch` rejection. The user reviews/merges afterward.

Trade-off: the sandbox must have *some* network reachability to the git server.
Keep that allowlist entry as narrow as possible (single host/port).

### Topology B — push-from-host (only if mount is writable + propagates)

Use this if Phase 0 finds guest writes propagate back to the host temp dir. This
is the stronger threat model — prefer it if available.

6B. The agent commits **inside the VM**; writes propagate through the mount into
    the host temp dir.
7B. The VM stays **fully network-locked except `api.anthropic.com`** — no git
    server reachable from the sandbox at all.
8B. After the session, **contagent pushes from the host** (temp dir →
    canonical repo). Git credentials never enter the VM.

### Cleanup (both)

- Temp dir and VM volume/overlay are ephemeral and removed on exit.
- **Make cleanup conditional on a successful sync-out.** If the push (A) or the
  host-side push (B) fails, **retain the temp dir**, surface the error, and exit
  non-zero. Never silently discard a session's work.

---

## Feature mapping (contagent → Matchlock)

| contagent feature | Matchlock primitive (verify in Phase 0) | Work to do |
|---|---|---|
| Disposable isolated env | `sdk.New(image)` + `client.Launch` | Lifecycle wrapper |
| Run command | `Exec` / `ExecStream` (+ `workingDir`) | CLI dispatch |
| Run agent w/ TTY | `ExecInteractive` (PTY) | Port TTY + resize forwarding |
| Signal forwarding | host-side around `ExecInteractive` | Reuse existing plumbing |
| Repo in + session branch + sync out | mount + git (topology A/B) | Repo transport (above) |
| Build from Dockerfile | `matchlock build` (BuildKit-in-VM) | Shell out or pre-bake rootfs |
| Auto cleanup | `Close` / `Remove` + ephemeral overlay | Conditional teardown |
| API key injection | `AddSecret(key, val, "api.anthropic.com")` | Replace env pass-through |
| Config (YAML + flags) | builder options | Map config keys → builder calls |
| Egress control (new) | `AllowHost` default-deny (+ runtime add/delete?) | Allowlist from config |
| Detached + reattach | detached run + exec reattach | SDK if available, else CLI |
| Port forwarding | port-forward | SDK if available, else CLI |

---

## What changes or drops

- **Docker socket forwarding → drop, or invert.** Forwarding the host Docker
  socket defeats the isolation. If agents genuinely need Docker, run a daemon
  **inside** the VM (Matchlock has a docker-in-sandbox example). Default: drop.
- **SSH agent forwarding → reconsider.** Same blast-radius concern. If it only
  existed for git auth, the host-side git server (topology A) or host-side push
  (topology B) makes it unnecessary. Keep only if a real in-VM use survives.
- **`ANTHROPIC_API_KEY` env var → secret injection.** The VM sees a placeholder;
  the real key is bound to `api.anthropic.com` on the host. This is the security
  upgrade that justifies the rewrite.
- **`host.docker.internal` reachability → VM NIC + allowlist.** The git server
  is reached over the VM's network with an explicit `AddHost`/`AllowHost` entry
  (topology A only).
- **Selective agent config seeding.** If `~/.claude` skills/commands need to be
  present, seed them via tarball + `exec untar` — **not** `settings.json`
  (hooks there can be abused by the sandboxed agent).

---

## Phased implementation

Each phase ends with a runnable, testable increment.

### Phase 1 — Walking skeleton
- `contagent <cmd>` boots a VM via `Launch`, runs `<cmd>` via `ExecStream`,
  streams stdout/stderr, propagates the exit code, tears down via `Remove`.
- No git, no secrets, no network policy yet.
- **Acceptance:** `contagent echo hello` prints `hello` and exits 0; the VM is
  gone afterward (verify via `matchlock list`).

### Phase 2 — Repo transport
- Implement the shared seed-in steps and whichever topology Phase 0 selected.
- Reuse contagent's git server and `contagent/<id>` naming.
- Conditional cleanup gated on successful sync-out.
- **Acceptance:** running a command that edits a file results in a
  `contagent/<id>` branch on the canonical repo containing that edit; a failed
  push leaves the temp dir intact and exits non-zero.

### Phase 3 — Interactive agent parity
- Wire `ExecInteractive` for `contagent claude`; port TTY-resize and signal
  forwarding.
- **Acceptance:** `contagent claude` gives a usable interactive session; window
  resize and Ctrl-C behave as in the Docker implementation.

### Phase 4 — Secrets + egress policy
- `AddSecret` for the Anthropic key; default-deny allowlist from config;
  optional config-seeding tarball.
- **Acceptance:** the key is never present in the VM's environment or filesystem
  (grep the VM to prove it); the agent still reaches the API; all other egress is
  blocked.

### Phase 5 — Config, lifecycle, build polish
- Map the full YAML/flag surface to builder options.
- Detached/reattach and port-forward (SDK or CLI per Phase 0).
- Orphan pruning; Dockerfile-build vs. cached-rootfs decision for fast starts.
- **Acceptance:** existing contagent config files drive the new runtime with no
  feature regressions vs. the Docker version (minus the intentionally dropped
  socket/agent forwarding).

---

## Invariants to assert in code / tests

- Session branch pushed is `contagent/<id>`, never the checked-out branch.
- Mounted repo is a self-contained clone, never a `git worktree`.
- Cleanup runs **only** after a confirmed successful sync-out.
- No code path places a secret value into VM env, args, or files.
- Egress allowlist is default-deny; topology A adds exactly one git-server entry.

---

## Open questions to resolve (tracked, blocking where noted)

- **[blocks Phase 2]** Mount capability + write-back propagation (Phase 0 Q1) →
  topology A vs. B.
- **[blocks Phase 3]** PTY resize mechanism (Phase 0 Q2).
- **[blocks Phase 5]** Detached/reattach and port-forward availability in SDK
  (Phase 0 Q3/Q4).
- Dirty-tree default: `clone --local` (no WIP) vs. `cp -a` (WIP included).
- Build strategy: BuildKit-in-VM per run vs. pre-baked cached rootfs.

---

## Definition of done

`contagent claude` and `contagent <cmd>` run inside a Matchlock microVM with:
the repo seeded in and the session branch synced back, the Anthropic key
injected without ever entering the VM, default-deny egress, full interactive TTY,
and disposable cleanup that never loses work — with the Docker socket and SSH
agent forwarding intentionally removed (or replaced by in-VM equivalents).
