# BotKeep Runtime Compatibility Design

Date: 2026-10-14
Status: Approved design, implementation pending
Branch: `botkeep/runtime`
Baseline: `1f3cc524dddbc9d59ca2b312f4408a813efa8f71`

## Goal

Make CottenDNS a first-class, reliable backend process inside a restricted BotKeep container without changing the tunnel wire protocol or diverging unnecessarily from upstream.

Success means BotKeep can supervise CottenDNS on an arbitrary allocated UDP port, determine readiness through the existing health interface, recover cleanly from failure, and verify the direct custom-port path in CI.

## Constraints

BotKeep runs without root, systemd, Docker, or control over the provider's public port mapping. The server may therefore listen on a non-standard allocated port such as UDP/43629 instead of UDP/53. The process must behave correctly under a non-interactive supervisor and must not wait on stdin after a startup failure.

The existing CottenDNS protocol, packet format, encryption behavior, ARQ, MTU discovery, resolver algorithms, domain rotation, and normal UDP/TCP/53 deployment behavior must remain unchanged.

## Existing Capabilities To Reuse

CottenDNS already provides the primitives BotKeep needs:

- `SIGTERM` / `SIGINT` cancellation through `signal.NotifyContext`.
- Optional HTTP health and Prometheus serving via `--metrics-address`.
- `/healthz` and detailed JSON health.
- Runtime transport state including whether UDP is up.
- Configurable `UDP_HOST` and `UDP_PORT`.
- Client resolver entries that can include explicit ports.

These capabilities must be reused rather than replaced by BotKeep-specific duplicates.

## Scope

### 1. Non-interactive startup semantics

The server currently calls `waitForExitInput()` on several startup failures. In a supervised container this can leave a failed process blocked waiting for stdin instead of exiting and letting the supervisor restart it.

Add an explicit runtime mode that controls whether startup failures pause for Enter. The default remains compatible with existing interactive usage. BotKeep uses non-interactive mode and receives an immediate non-zero exit code on configuration, key, or codec startup failure.

Preferred interface:

- New server flag: `--non-interactive`.
- When enabled, fatal startup errors never read stdin.
- When omitted, current interactive behavior is preserved.

The flag changes process UX only. It must not affect packet processing, networking, or configuration semantics.

### 2. Health contract for BotKeep supervision

Do not add a new health subsystem. BotKeep will start CottenDNS with an internal-only metrics address, for example:

```text
--metrics-address 127.0.0.1:19090
```

BotKeep considers the process ready only when:

- the process remains alive;
- `GET /healthz?details=1` succeeds;
- `status == "ok"`;
- `transports.udp_up == true`.

The endpoint remains bound to loopback in BotKeep deployments and must never expose encryption keys, client addresses, queried domains, destinations, or payloads.

No new wire-facing health traffic is introduced.

### 3. Custom-port direct-mode qualification

Add an automated end-to-end test covering the deployment shape BotKeep actually uses:

```text
client -> resolver label 127.0.0.1:<random-port> -> CottenDNS server
```

The test must allocate an ephemeral/non-53 UDP port rather than relying on privileged port 53. It must validate the tunnel protocol rather than only UDP reachability.

Minimum assertions:

- server starts on the chosen custom port;
- health reports UDP ready;
- client accepts the custom-port resolver;
- session initialization succeeds;
- at least one application-level flow traverses the tunnel successfully;
- clean shutdown completes without hanging.

The test must not require root, systemd, Docker, or external DNS infrastructure.

### 4. Accurate transport labels

Current client log labels use hard-coded strings such as `UDP/53` and `TCP/53`, even when a resolver entry explicitly uses another port. This is misleading in BotKeep direct mode.

Preserve protocol names while displaying the real endpoint when the port is non-standard. For example:

```text
UDP/43629
TCP/43629
```

For ordinary port 53 deployments, existing `UDP/53` and `TCP/53` wording remains unchanged.

This is a presentation/telemetry change only. Resolver dialing behavior must not change.

### 5. BotKeep deployment documentation

Add `docs/BOTKEEP.md` with a minimal supported deployment recipe:

- run as an unprivileged process;
- use the provider-assigned UDP port through `UDP_PORT`;
- disable unused listeners for the direct-mode profile;
- enable loopback health with `--metrics-address`;
- enable `--non-interactive`;
- let the outer BotKeep supervisor own restart policy;
- never hard-code a particular provider allocation such as 43629;
- state clearly that direct custom-port mode is not equivalent to delegated public DNS/53 and therefore is not sufficient by itself for recursive-resolver national-network survival.

Include a sample config intended for BotKeep direct mode. The sample must contain placeholders for domain, key path, and allocated port and must not contain real secrets.

## Non-goals

This phase does not implement Iran Survival resolver ranking, ISP-specific resolver pools, whitelist discovery, CottenRouter integration, public authoritative port-53 fronting, protocol changes, new encryption methods, or changes to WhiteDNS Android.

Those are separate client/front-end projects and should not be coupled to runtime compatibility.

## Error Handling

Fatal pre-run errors in non-interactive mode exit immediately with a non-zero status. Once the server is running, existing cancellation and listener error behavior is retained.

The outer BotKeep supervisor is responsible for restart/backoff. CottenDNS itself must not grow a second internal restart loop; keeping one process supervisor avoids duplicate lifecycle ownership and restart storms.

If the optional metrics listener cannot bind, startup remains fatal when `--metrics-address` was explicitly requested, because BotKeep relies on that endpoint for readiness.

## Compatibility

- Existing configs remain valid.
- Existing server invocation remains valid.
- Default interactive behavior remains unchanged.
- No packet-format or crypto changes.
- No resolver-selection changes.
- Normal UDP/TCP/53 deployments retain their current behavior.

The fork should stay easy to rebase onto WhiteDNS upstream.

## Testing

Implementation must add focused tests for:

1. interactive vs non-interactive fatal startup behavior without hanging;
2. custom-port transport label formatting;
3. existing health JSON readiness semantics;
4. unprivileged custom-port end-to-end tunnel operation;
5. full `go test ./...` and `go vet ./...` regression gates.

Where possible, test helper functions directly rather than spawning a whole process. The custom-port tunnel qualification is intentionally the exception because it exists to prove cross-component behavior.

## Rollout

Implementation lands only on `botkeep/runtime` first. After all tests pass, build a Linux AMD64 candidate from that branch and validate it on the existing BotKeep server using its provider allocation. Validation includes startup, loopback health, direct client scan, SOCKS/application flow, forced process termination followed by outer-supervisor recovery, and clean container restart.

Only after that qualification should the fork's main branch or BotKeep runtime pin move to the candidate.

## Future Work

A separate design will cover Iran Survival resolver intelligence. That work should focus on client-side resolver persistence, carrier-specific known-good pools, ranking, progressive fallback, and offline-shipped survival lists. A separate public authoritative DNS/53 front is also required for true recursive-resolver operation during national-network conditions.