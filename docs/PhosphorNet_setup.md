# PhosphorNet Setup Guide

## Purpose

This guide gets a local PhosphorNet station running from a fresh checkout.

PhosphorNet currently runs as three command-line programs:

- `phosphor`: trusted terminal client.
- `phosphord`: node daemon that hosts doors.
- `switchboard`: relay/rendezvous scaffold.

For local development, you usually run `phosphord` in one terminal and `phosphor` in another.

## Requirements

Required:

- Go matching `go.mod`.
- A terminal that works well with Bubble Tea alternate-screen programs.

Optional:

- Python 3.11+ for Python doors.
- SQLite command-line tools if you want to inspect local state directly.

The project uses pure-Go SQLite through `modernc.org/sqlite`, so CGO is not required for the current default path.

## First Local Station

From the repository root, create a node configuration:

```bash
go run ./cmd/phosphord init --name localbox --out node.toml
```

This writes a `node.toml` file containing:

- station display name
- node Ed25519 public key
- node private key
- listen address
- doors directory
- SQLite database path
- runtime defaults
- admin access seeded from a local passport

It also seeds the default station policy in the SQLite database so `strategy_demo` starts disabled on fresh stations until an admin enables it.

If the default local passport does not exist, `phosphord init` creates it at:

```text
~/.config/phosphornet/passport.toml
```

The generated or reused passport public key is added to `access.admins` in `node.toml`, which makes the bundled `Station Admin` door visible to that passport.

You can choose a different admin passport path:

```bash
go run ./cmd/phosphord init --name localbox --out node.toml --admin-passport ./dev/admin-passport.toml
```

Start the node:

```bash
go run ./cmd/phosphord serve --config node.toml
```

By default, the node listens on `:7707` and serves TLS-wrapped WebSocket sessions at:

```text
wss://127.0.0.1:7707/ws
```

`phosphord` derives a self-signed TLS certificate from the station's Ed25519 node key. `phosphor` accepts self-signed station certificates by default because station identity still comes from the signed Ed25519 node challenge plus known-node pinning.

The trust stack is deliberately split:

```text
TLS
  Is this transport encrypted?

Ed25519 node key
  Is this the same station identity as before?

Public CA certificate
  Does this hostname match a WebPKI certificate?

Self-signed station certificate
  Acceptable for PhosphorNet because station identity is pinned separately.
```

On a first connection, the trusted client presents that distinction in a local Bubble Tea trust screen before pinning the station: the Ed25519-signed station name, TLS encrypted or not encrypted, certificate self-signed or domain-authenticated, hostname verification status, and whether the Ed25519 station identity is new or already pinned.

In another terminal, connect with the client:

```bash
go run ./cmd/phosphor connect --addr wss://127.0.0.1:7707/ws --quick
```

`--quick` is intended for local development. It stores a disposable passport and known-node file under:

```text
/tmp/phosphornet-quick/
```

## Persistent Local Passport

For a normal local identity, create a passport:

```bash
go run ./cmd/phosphor passport create
```

Show its public key and fingerprint:

```bash
go run ./cmd/phosphor passport show
```

Connect without `--quick`:

```bash
go run ./cmd/phosphor connect --addr wss://127.0.0.1:7707/ws
```

Default local client files live under:

```text
~/.config/phosphornet/passport.toml
~/.config/phosphornet/known_nodes.toml
```

The passport private key stays on the client machine. Nodes only see the public key and a challenge-response signature.

## Health Check

While `phosphord` is running, check the local health endpoint:

```bash
curl -k https://127.0.0.1:7707/healthz
```

Expected response:

```text
ok
```

The switchboard scaffold has the same style of health endpoint:

```bash
go run ./cmd/switchboard serve --listen :7710
curl http://127.0.0.1:7710/healthz
```

## Build Binaries

Build the three executables:

```bash
go build ./cmd/phosphor
go build ./cmd/phosphord
go build ./cmd/switchboard
```

Run tests:

```bash
go test ./...
```

If your default Go cache is not writable in a sandboxed environment, redirect the caches:

```bash
GOCACHE=/tmp/phosphornet-gocache GOMODCACHE=/tmp/phosphornet-gomodcache go test ./...
```

Compile Python SDK files:

```bash
python3 -m py_compile \
  sdk/python/phosphornet/runtime.py \
  sdk/python/phosphornet/ctx.py \
  sdk/python/phosphornet/ui.py
```

## Reset Local Development State

For a repository-local node created with the default paths, stop `phosphord`, then remove:

```text
node.toml
phosphornet.db
```

For quick client identity state, remove:

```text
/tmp/phosphornet-quick/
```

For persistent client identity state, remove only the files you intentionally want to replace:

```text
~/.config/phosphornet/passport.toml
~/.config/phosphornet/known_nodes.toml
```

Be careful with `passport.toml`: it is the user's portable identity key.

For real station backup and restore, do not use the local reset list as an operator procedure. Back up and restore `node.toml`, the configured SQLite database, node private key material, and custom bundled doors together. See `docs/PhosphorNet_database_lifecycle.md`.

## Common Setup Issues

Port already in use:

- Edit `listen_addr` in `node.toml`, then reconnect with a matching `--addr`.

Python door fails to load:

- Confirm Python 3.11+ is available as `python3`.
- Run the Python compile command above.
- Confirm the door manifest has `runtime = "stdio"`, a command such as `command = ["python3", "app.py"]`, and `[isolation] mode = "host"` for trusted local direct execution.

Known-node error after regenerating `node.toml`:

- The client pinned the old node key for that address.
- For local development, remove the matching entry from `known_nodes.toml`, use `--quick` with a fresh quick directory, or reconnect with `--replace-known-node`.

No doors appear:

- Confirm `doors_dir` points at a directory containing `*/manifest.toml`.
- Restart `phosphord` after adding or changing door manifests.
