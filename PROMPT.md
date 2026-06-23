# Phase 0 — Resolve open questions for the Matchlock microVM rewrite

Read `PLAN.md` in this repo. It describes a rewrite of contagent's runtime from
Docker containers to Matchlock microVMs.

**Your job is to complete Phase 0**: investigate the Matchlock Go SDK and answer
the seven open questions listed in the plan. The Matchlock source is at
https://github.com/jingkaihe/matchlock — read `pkg/sdk` and `examples/go` there.

## What to produce

Create `docs/matchlock-sdk-findings.md` with your answers. For each of the seven
questions below, provide:

- The answer, with exact Go symbols (type names, method names, full signatures).
- A tag: `confirmed`, `cli-only`, or `not-available`.
- A short code snippet showing how we'd call it from contagent.

### The seven questions

1. **Mount system.** Can you mount an arbitrary host directory into the VM at a
   chosen path? Is the mount writable in-guest, and do writes propagate back to
   the host? This determines topology A vs. B for repo transport.

2. **Interactive PTY.** Confirm `ExecInteractive` (or equivalent) exists, its
   signature, and how terminal resize (SIGWINCH) is surfaced.

3. **Detached + reattach.** Are "run detached" and "exec into a running sandbox"
   available in the Go SDK, or only via the CLI?

4. **Port-forward.** SDK or CLI-only?

5. **Lifecycle.** Exact semantics of `client.Launch`, `client.Close(code)`,
   `client.Remove()` — what each cleans up, and whether `Remove` also drops
   mounted volumes/overlays.

6. **Network allowlist.** Confirm `AllowHost`, `AddHost(name, ip)`,
   `AddSecret(name, value, host)` signatures. Can the allowlist be mutated after
   launch, or only at build time?

7. **Build path.** How to produce/run a VM from a Dockerfile or OCI image, and
   where image caching lives.

## After answering

Based on your findings, add a **Recommendations** section at the bottom of the
findings doc that states:

- Whether we should use topology A (push-from-inside) or B (push-from-host).
- Which features require shelling out to the `matchlock` CLI vs. using the SDK.
- Any SDK gaps or surprises that affect the plan.

## Branching

- All new code must be based off the `v2` branch.
- When creating pull requests, set `v2` as the base branch.

## Ground rules

- Do not guess. If a capability isn't in the SDK source, say `not-available` or
  `cli-only` — don't fabricate method names.
- Read the actual Go source in the Matchlock repo. Do not rely on READMEs alone.
- Keep the findings doc concise — tables and code blocks, not prose.
