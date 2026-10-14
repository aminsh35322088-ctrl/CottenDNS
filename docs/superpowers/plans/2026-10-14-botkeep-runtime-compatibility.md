# BotKeep Runtime Compatibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make CottenDNS safe to supervise inside BotKeep, qualify arbitrary UDP ports end-to-end, and report custom-port transports accurately without changing the tunnel protocol.

**Architecture:** Keep lifecycle ownership in BotKeep and reuse CottenDNS's existing signal handling and health/metrics server. Add only a non-interactive startup switch, endpoint-aware transport presentation, stronger E2E qualification around the already-custom-port test harness, and BotKeep deployment documentation/config.

**Tech Stack:** Go, standard `flag`/`net`/`net/http`/`os/exec` packages, existing CottenDNS test harness and TOML configuration.

**Spec:** `docs/superpowers/specs/2026-10-14-botkeep-runtime-compatibility-design.md`

## Global Constraints

- No tunnel wire-format, encryption, ARQ, MTU, resolver-selection, or domain-rotation behavior changes.
- Existing server invocation remains compatible; interactive pause behavior remains the default.
- BotKeep mode must work unprivileged and on arbitrary provider-assigned UDP ports.
- Reuse `/healthz?details=1`; do not add a second health subsystem or an internal restart loop.
- No real secrets or fixed provider allocation values in repository samples.

## Review Focus

- Invalid config with `--non-interactive` must exit promptly and non-zero without reading stdin.
- Existing invocation without `--non-interactive` must retain the current pause helper path.
- Custom IPv4 and IPv6 resolver endpoints must format ports correctly without breaking DoT/DoH labels.
- A non-53 UDP port must carry a real session and application flow, not only answer a DNS probe.
- Explicit health binding failure must remain fatal, while successful health must report `status=ok` and `udp_up=true` only after the UDP runtime is actually live.

---

### Task 1: Non-interactive server startup failures

**Files:**
- Modify: `cmd/server/main.go`
- Create: `cmd/server/main_test.go`

**Interfaces:**
- Produces: `pauseOnStartupFailure(nonInteractive bool, in io.Reader, out io.Writer)` (or an equivalently focused helper) used by all pre-run fatal startup paths.
- Produces: server CLI flag `--non-interactive` with default `false`.

- [ ] **Step 1: Write failing helper tests**

Add tests proving the helper returns immediately without consuming input when `nonInteractive=true`, and follows the existing prompt/read behavior when false. The test must not invoke `main()` or call `os.Exit`.

- [ ] **Step 2: Run the focused tests and confirm RED**

Run: `go test ./cmd/server -run 'Test.*StartupFailure' -count=1`
Expected: FAIL because the helper/flag behavior does not exist yet.

- [ ] **Step 3: Implement the minimal startup UX change**

Add `--non-interactive`; route config-load, key-setup, codec-setup, codec-set, and missing-domain fatal paths through the helper. Do not alter runtime/listener failure semantics.

- [ ] **Step 4: Run focused and package tests**

Run: `go test ./cmd/server -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

Commit message: `feat(server): add non-interactive startup mode`

### Task 2: Endpoint-aware transport labels

**Files:**
- Modify: `internal/client/transport.go`
- Modify: the resolver/scan log call sites that have a concrete resolver endpoint.
- Modify: `internal/client/transport_chain_test.go` and/or add a focused `internal/client/transport_label_test.go`.

**Interfaces:**
- Produces: `resolverTransport.labelForEndpoint(resolverLabel string) string` (exact receiver/name may vary only if an existing naming pattern is clearly better).
- Behavior: UDP/TCP use the endpoint's explicit port; missing ports retain 53; DoT/DoH remain `DoT`/`DoH`.

- [ ] **Step 1: Write failing label tests**

Cover `1.1.1.1:43629 -> UDP/43629`, `[2001:db8::1]:43629 -> UDP/43629`, `1.1.1.1 -> UDP/53`, TCP equivalents, malformed/host-only fallback, and unchanged DoT/DoH.

- [ ] **Step 2: Run focused tests and confirm RED**

Run: `go test ./internal/client -run 'Test.*Transport.*Label' -count=1`
Expected: FAIL because endpoint-aware labeling is absent.

- [ ] **Step 3: Implement the label helper and use it only where an endpoint is known**

Do not change transport selection or dialing. Keep `resolverTransport.String()` stable for generic chain/status text and use the endpoint-aware helper for scan/resolver telemetry that names a resolver.

- [ ] **Step 4: Run client tests**

Run: `go test ./internal/client -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

Commit message: `fix(client): report custom resolver transport ports`

### Task 3: BotKeep custom-port readiness and tunnel qualification

**Files:**
- Modify: `test/e2e/tunnel_e2e_test.go`
- Modify only if needed: `internal/udpserver/dns_upstream_health_test.go`

**Interfaces:**
- Reuses: existing `freeUDPPortForHost`, explicit `host:port` resolver file, real built client/server binaries, and TCP echo flow.
- Adds: loopback metrics address for the server and a helper that polls detailed `/healthz` JSON until `status == "ok"` and `transports.udp_up == true`.

- [ ] **Step 1: Add a failing BotKeep-specific E2E readiness assertion**

Create `TestBotKeepCustomPortEndToEnd` (or refactor the existing harness so this named test is explicit) that starts the server on an ephemeral non-53 UDP port with `--non-interactive --metrics-address 127.0.0.1:<ephemeral>`, waits for health readiness, uses a resolver entry containing that exact custom port, establishes the real session, and round-trips application data.

- [ ] **Step 2: Add clean shutdown assertion**

After the application flow succeeds, send the platform-appropriate graceful termination used by the current Linux CI path and require the server process to exit within a bounded timeout. Keep hard kill only as cleanup fallback.

- [ ] **Step 3: Run the named E2E and confirm it exposes missing behavior before the implementation is complete**

Run: `go test -tags e2e -run TestBotKeepCustomPortEndToEnd -timeout 120s ./test/e2e/`
Expected before Task 1 integration: FAIL on unknown `--non-interactive`; after Task 1: PASS.

- [ ] **Step 4: Run health unit tests plus the named E2E**

Run: `go test ./internal/udpserver -run 'TestMetricsHandler' -count=1`
Run: `go test -tags e2e -run TestBotKeepCustomPortEndToEnd -timeout 120s ./test/e2e/`
Expected: PASS.

- [ ] **Step 5: Commit**

Commit message: `test(e2e): qualify BotKeep custom-port runtime`

### Task 4: BotKeep deployment profile and operator contract

**Files:**
- Create: `docs/BOTKEEP.md`
- Create: `server_config.botkeep.toml`
- Modify: `CONFIG_PRESETS.md` only if needed to point operators to the profile; do not add a behavioral preset unless tests prove it is necessary.

**Interfaces:**
- Documents invocation: `CottenDns_Server ... --config server_config.botkeep.toml --non-interactive --metrics-address 127.0.0.1:<health-port>`.
- Documents readiness contract: process alive + detailed health `status=ok` + `transports.udp_up=true`.

- [ ] **Step 1: Add the minimal sample config**

Use placeholders/comments for `DOMAIN`, `UDP_PORT`, and `ENCRYPTION_KEY_FILE`; disable unused TCP/DoT/DoH/ACME listeners for direct mode; never include a real key or `43629` as a required value.

- [ ] **Step 2: Write `docs/BOTKEEP.md`**

Cover unprivileged runtime, arbitrary provider allocation, supervisor ownership, loopback health, graceful stop/restart, and the limitation that direct custom-port mode is not delegated authoritative DNS/53 and therefore is not by itself a national-network recursive-resolver solution.

- [ ] **Step 3: Validate shipped TOML**

Run the existing config/template tests that parse shipped server configs, extending them only if this new file is not automatically covered.
Expected: PASS.

- [ ] **Step 4: Commit**

Commit message: `docs: add BotKeep deployment profile`

### Task 5: Full regression and candidate build

**Files:**
- No product-code changes unless a regression is found and fixed with a new focused test.

**Interfaces:**
- Produces: Linux AMD64 server candidate from `botkeep/runtime` suitable for BotKeep qualification.

- [ ] **Step 1: Run Go regression gates**

Run: `gofmt -w` on changed Go files, `go vet ./...`, and `go test ./...`.
Expected: all PASS.

- [ ] **Step 2: Run the BotKeep E2E gate**

Run: `go test -tags e2e -run TestBotKeepCustomPortEndToEnd -timeout 120s ./test/e2e/`
Expected: PASS.

- [ ] **Step 3: Build Linux AMD64 server candidate**

Run with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o <candidate> ./cmd/server` and record the branch head SHA in the artifact name or validation notes.

- [ ] **Step 4: Validate on the existing BotKeep host without replacing the known-good binary first**

Start the candidate on the provider-assigned UDP allocation with a loopback health port, verify detailed health, direct resolver scan, real SOCKS/application flow, graceful termination, and supervisor recovery. Promote/pin only after these checks pass.

- [ ] **Step 5: Commit any validation-only documentation update, then report exact evidence**

Do not claim completion unless the commands above pass and the deployed candidate is verified.
