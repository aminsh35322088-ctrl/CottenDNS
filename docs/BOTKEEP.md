# Running CottenDNS on BotKeep

This profile is for **direct custom-port mode** on a restricted BotKeep-style container: no root, no systemd, no Docker, and no assumption that public UDP/TCP 53 is available.

## Runtime model

BotKeep owns the process lifecycle. CottenDNS should run as an ordinary unprivileged child process and should **not** supervise or restart itself.

Start the server with the provider-assigned UDP port configured in `server_config.botkeep.toml` and bind health only to loopback:

```bash
./CottenDns_Server_Linux_AMD64 \
  --config server_config.botkeep.toml \
  --non-interactive \
  --metrics-address 127.0.0.1:<health-port>
```

`<health-port>` is an internal TCP port chosen by the supervisor. It does not need to be exposed publicly.

The shipped `server_config.botkeep.toml` uses `UDP_PORT = 10053` only as an unprivileged example. Replace it with the UDP allocation assigned to the actual BotKeep instance. Do not assume any particular provider allocation is stable across different instances.

## Readiness contract

The outer supervisor should consider CottenDNS ready only when all three conditions are true:

1. the process is still alive;
2. `GET http://127.0.0.1:<health-port>/healthz?details=1` succeeds; and
3. the JSON response has `status == "ok"` and `transports.udp_up == true`.

The detailed health endpoint is intended for local supervision and exposes runtime counters and transport state, not encryption keys or tunnel payloads.

If the explicitly requested metrics/health listener cannot bind, startup fails. This is deliberate: a BotKeep supervisor that depends on health must not treat an unobservable process as ready.

## Failure and restart behavior

Use `--non-interactive` under BotKeep. Fatal startup failures then exit non-zero immediately instead of waiting for an Enter key that a background container supervisor will never provide.

For shutdown, send `SIGTERM` or `SIGINT` and allow CottenDNS to drain through its existing context cancellation path. If the process exits unexpectedly or becomes unhealthy, the **outer BotKeep supervisor** should restart it with bounded backoff. Do not add a second restart loop inside CottenDNS; one lifecycle owner avoids restart storms and duplicate processes competing for the same UDP port.

Keep `encrypt_key.txt` on persistent storage. Replacing or losing that file changes the shared key and breaks existing clients until they are updated.

## Direct client endpoint

In direct mode the client resolver entry points straight at the BotKeep public allocation, including its non-standard port, for example:

```text
203.0.113.10:<allocated-udp-port>
```

CottenDNS supports explicit resolver ports, so the client does not require the server to listen on 53 for this direct path.

## Official WhiteDNS Android compatibility

The official `WhiteDNS/WhiteDNS-Android` app currently pins its CottenDNS client submodule to the same `1f3cc524dddbc9d59ca2b312f4408a813efa8f71` baseline this BotKeep branch was forked from. This BotKeep work does not change the tunnel wire format, encryption methods, session protocol, MTU behavior, query carriers, or resolver dialing semantics, so the official Android CottenDNS client remains protocol-compatible.

Android resolver profiles preserve explicit non-standard endpoints such as `203.0.113.10:10053`, and the runtime renderer writes those endpoints to the CottenDNS resolver file unchanged. That is the correct way to configure BotKeep direct mode.

The Android **scanner** workflow intentionally strips explicit ports before bulk resolver scanning because it targets normal recursive DNS resolver discovery. Therefore, for a direct BotKeep backend on a provider-assigned port, configure the `IP:port` endpoint in a manual/saved resolver profile rather than expecting a scanner result to preserve the custom port.

## What this mode does not solve

Direct custom-port mode is **not** the same as delegated authoritative DNS reachable on public UDP/TCP 53.

A normal recursive resolver follows DNS delegation and contacts authoritative nameservers on standard DNS ports; it will not discover a CottenDNS backend hidden only behind an arbitrary provider port. Therefore this BotKeep direct profile, by itself, is **not sufficient for national-network / recursive-resolver survival** where the client must rely on whitelisted recursive resolvers.

That deployment needs a separate publicly reachable authoritative DNS front on UDP/TCP 53 (or equivalent infrastructure that preserves the DNS tunnel flow) in front of the CottenDNS backend. Resolver ranking and Iran-specific survival pools belong on the client side and are intentionally outside this server-runtime profile.

## Recommended BotKeep supervisor sequence

1. Start CottenDNS with `--non-interactive` and a loopback `--metrics-address`.
2. Poll detailed health until `status=ok` and `udp_up=true`.
3. Only then advertise the service as ready.
4. On `SIGTERM`, forward the signal to CottenDNS and wait for clean exit before force-killing.
5. On unexpected exit or health failure, restart with bounded backoff.
6. Keep the config and encryption key on persistent storage.

The repository E2E gate `TestBotKeepCustomPortEndToEnd` exercises this shape on an ephemeral non-53 UDP port, validates loopback health, creates a real tunnel session, round-trips application data, and verifies graceful shutdown.
