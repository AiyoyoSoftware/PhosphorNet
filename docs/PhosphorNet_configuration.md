# PhosphorNet Configuration Guide

## Purpose

This guide documents the current configuration files used by `phosphor`, `phosphord`, doors, and the switchboard scaffold.

Configuration is intentionally plain TOML where possible. Secrets such as passport private keys and node private keys should be treated as private local files.

## Node Configuration

Create a starter node config:

```bash
go run ./cmd/phosphord init --name localbox --out node.toml
```

`phosphord init` creates or reuses a local admin passport, writes its public key into `access.admins`, and seeds the default station policy in SQLite. Fresh stations start with `strategy_demo` disabled until an admin explicitly enables it from the Station Admin door. By default, the admin passport path is:

```text
~/.config/phosphornet/passport.toml
```

Override it with:

```bash
go run ./cmd/phosphord init --name localbox --out node.toml --admin-passport ./dev/admin-passport.toml
```

Example `node.toml`:

```toml
name = "localbox"
listen_addr = ":7707"
node_id = "base64-ed25519-public-key"
private_key = "base64-ed25519-private-key"
doors_dir = "./doors"
database = "./phosphornet.db"

[tls]
enabled = true

[access]
mode = "public"
allowlist = []
admins = ["base64-ed25519-admin-public-key"]

[runtime]
default_runtime = "lua"

[runtime.lua]
profile = "strict"
max_memory_kb = 65536
max_execution_ms = 5000
call_stack_size = 120
registry_size = 20480
registry_max_size = 81920
registry_grow_step = 32
```

Fields:

| Field | Meaning |
|---|---|
| `name` | Station display name shown to clients and doors. |
| `listen_addr` | HTTPS/WebSocket listen address used by `phosphord`. |
| `node_id` | Public Ed25519 node identity. |
| `private_key` | Private Ed25519 node key. Keep this private. |
| `doors_dir` | Directory scanned for `*/manifest.toml`. |
| `database` | SQLite database path for users and door state. |
| `tls.enabled` | Serve `wss://` and `https://` with a self-signed certificate derived from the node's Ed25519 key. Defaults to `true`. |
| `access.mode` | Station access mode: `public` or `invite_only`. |
| `access.allowlist` | Public keys or fingerprints allowed into an `invite_only` station. |
| `access.admins` | Public keys or fingerprints that authenticate as station admins. |
| `runtime.default_runtime` | Runtime used when a manifest does not specify one. |
| `runtime.lua` | Default Lua sandbox settings. |

Start a node with:

```bash
go run ./cmd/phosphord serve --config node.toml
```

`phosphord` serves:

```text
/ws       WebSocket client sessions
/healthz  health check
```

With the default `[tls].enabled = true`, those endpoints are served as `wss://.../ws` and `https://.../healthz`. The TLS certificate is self-signed and transport-only; station identity still comes from the signed Ed25519 node challenge and known-node pinning.

## Station Access

Stations default to public access:

```toml
[access]
mode = "public"
allowlist = []
admins = []
```

Invite-only stations authenticate passports normally, then deny users whose public key or fingerprint is not allowlisted:

```toml
[access]
mode = "invite_only"
allowlist = [
  "base64-ed25519-public-key",
  "ABCD1234",
]
admins = [
  "base64-ed25519-admin-public-key",
]
```

The allowlist accepts exact public keys and passport fingerprints. Admins are allowed into invite-only stations even if they are not also listed in `allowlist`. Station role assignments affect what an admitted user can do after they enter; they do not, by themselves, bypass station admission.

Admin users are configured with:

```toml
[access]
admins = [
  "base64-ed25519-admin-public-key",
  "WXYZ-1234-ABCD",
]
```

Use `phosphor passport show` to print the public key and fingerprint for a local passport. The private key still stays on the client machine.

## Admission, Access, Capabilities

Do not confuse these layers:

```text
Station admission
  Whether a passport can enter the station at all.
  Current modes: public, invite_only.

Session role
  What the admitted passport can do.
  Current roles: member, admin, sysop.

Door access
  Whether the admitted session can open a specific door.
  Current manifest values: public, invite_only, admin.

Door capabilities
  Which privileged effects that door may request from phosphord.
```

An admin session opening an admin-only door does not automatically make that door powerful. Privileged operations require both an admin/sysop session and the matching door capability.

## Runtime Defaults

Lua is the default runtime. Other languages use the generic stdio runtime by declaring an argv command:

```toml
runtime = "stdio"
command = ["python3", "app.py"]

[isolation]
mode = "host"
```

Runtime resolution is conservative:

- `runtime = "lua"` uses the embedded Lua invoker.
- `runtime = "stdio"` defaults to Podman isolation when `isolation.mode` is omitted, or runs a trusted host command only when `[isolation] mode = "host"` explicitly opts out.
- omitted `runtime` on `.lua` entrypoints uses the embedded Lua invoker.
- omitted `runtime` on any other entrypoint uses `runtime.default_runtime`.

There is no Python-specific runtime name. Python doors are stdio doors and must declare `runtime = "stdio"` plus an explicit command or Podman image.

Direct host stdio invocation is deliberately boring and must be explicit with `[isolation] mode = "host"`:

- `command = [...]` is executed as explicit argv, not through a shell.
- the process working directory is the door directory.
- the canonical runtime request is written to stdin as JSON.
- the canonical runtime response is read from stdout as JSON.
- stderr is kept as runtime diagnostics, not protocol output.
- `phosphord` enforces timeout, stdout size, stderr size, and malformed-JSON handling.

Podman isolation is the default stdio execution profile, not a new runtime protocol:

```toml
runtime = "stdio"

[isolation]
image = "localhost/phosphornet/weather-door:0.1.0"
network = "none"
read_only = true
timeout_ms = 1500
memory = "128m"
cpus = 0.25
pids_limit = 64
```

For Podman mode, either implicit through omitted `mode` or explicit with `mode = "podman"`, `phosphord` constructs a `podman run` argv without a shell, pipes the same request JSON to container stdin, and decodes the same response JSON from container stdout. The default profile is intentionally harsh: `network = "none"`, read-only rootfs, `--cap-drop=ALL`, `--security-opt=no-new-privileges`, `--userns=keep-id`, `memory = "128m"`, `cpus = 0.25`, and `pids_limit = 64` when those fields are omitted.

Third-party image doors should package their code inside the image. `phosphord` does not mount the station's real door directory into arbitrary containers by default.

Manifest capabilities and isolation are separate:

- `capabilities = [...]` controls which structured PhosphorNet effects a door may request.
- `[isolation]` controls host/container resources for the door process.

Open containment work remains: environment allowlisting and richer named resource profiles are not part of the current stdio backend yet.

## Lua Sandbox

Lua sandbox settings can be defined at the node level and overridden per door manifest.

Profiles:

| Profile | Libraries opened by default | Intended use |
|---|---|---|
| `strict` | `base`, `table`, `string`, `math` | Default hosted-door profile. |
| `standard` | `base`, `coroutine`, `table`, `string`, `math` | More Lua language features without filesystem libraries. |
| `unsafe` | includes `io`, `os`, `debug`, `package`, `channel` | Local experiments only. |
| `custom` | explicit `libraries` list | Narrow custom host policy. |

In non-unsafe profiles, `collectgarbage`, `dofile`, and `loadfile` are removed from globals.

Example door override:

```toml
[sandbox]
profile = "standard"
max_memory_kb = 32768
max_execution_ms = 1500
libraries = ["base", "table", "string", "math"]
```

Keep hosted doors on `strict` unless a specific door has a clear reason to change.

## Door Manifests

Each door lives in its own directory under `doors_dir`:

```text
doors/
  hello/
    manifest.toml
    app.lua
```

Minimal Lua manifest:

```toml
id = "hello"
name = "Hello"
entry = "app.lua"
visibility = "public"
access = "public"
```

Python-over-stdio manifest:

```toml
id = "chat"
name = "Chat"
runtime = "stdio"
command = ["python3", "app.py"]
visibility = "public"
access = "public"

[isolation]
mode = "host"

capabilities = [
  "state:user:read",
  "state:user:write",
  "state:room:read",
  "state:room:write",
  "broadcast:room",
  "notify:room",
]
```

Python image doors can build on the SDK base image defined at `sdk/python/Dockerfile`:

```Dockerfile
FROM localhost/phosphornet/python-door-sdk:latest

COPY app.py /door/app.py
ENV PHOSPHORNET_DOOR_ENTRY=/door/app.py
```

```bash
podman build -t localhost/phosphornet/python-door-sdk:latest sdk/python
podman build -t localhost/phosphornet/my-python-door:0.1.0 doors/my_python_door
```

Fields:

| Field | Meaning |
|---|---|
| `id` | Stable door identifier used by the node and protocol. |
| `name` | Display name shown in the client door rail. |
| `entry` | Lua entrypoint file, resolved inside the door directory. |
| `command` | `stdio` argv, for example `["python3", "app.py"]` or `["./door-binary"]`. |
| `runtime` | Optional runtime name. Defaults to node runtime settings. |
| `visibility` | Client-facing visibility: `public`, `private`, or `hidden`. |
| `access` | Door access mode: `public`, `invite_only`, or `admin`. |
| `allowlist` | Public keys or fingerprints allowed into an `invite_only` door. |
| `capabilities` | Runtime authorities this door may request. Privileged effects require both user role and door capability. |
| `permissions` | Deprecated compatibility metadata. Known legacy values are mapped to capabilities during manifest loading. |
| `sandbox` | Optional Lua sandbox override. |
| `settings` | Optional door-specific operator setting schema. Manifest settings declare keys, labels, types, and defaults; edited values are stored in SQLite by the Station Admin door. |

Entrypoints must stay inside their own door directory. `phosphord` rejects manifests that try to escape with parent paths.

Capability checks are node-owned:

- `access` decides who can open a door.
- `capabilities` decide which effects that door may ask `phosphord` to apply.
- Admin operations require an `admin` or `sysop` session and the matching `admin:*` capability on the door manifest.
- State, broadcast, notify, transition, and key-capture effects are rejected when the manifest lacks the required capability.
- `permissions = [...]` is deprecated. It remains only as a compatibility bridge for older manifests.

Door settings are declared as manifest schema, not as live station config:

```toml
[settings.motd]
type = "textarea"
label = "Message of the day"
default = "Welcome to this PhosphorNet station."

[settings.show_online_users]
type = "bool"
label = "Show online users"
default = true

[settings.theme_tagline]
type = "string"
label = "Station tagline"
default = "terminal-native public square"
```

Supported MVP setting types:

| Type | Meaning |
|---|---|
| `string` | Single-line text. |
| `textarea` | Longer plain text. |
| `bool` | True/false value. |
| `int` | Whole number. |
| `select` | One string from `options = [...]`. |
| `markdown` | Longer Markdown text rendered by doors that choose to display it as Markdown. |

The manifest supplies defaults. Station-specific edits are made from Station Admin and stored under node state in SQLite, so changing a lobby MOTD or forum policy does not require editing door files or restarting `phosphord`. At runtime, doors read the resolved typed values through `ctx.settings`.

Station policy itself is stored as node-owned state, not as ordinary door-global state. Legacy admin-door global policy is migrated into node state on first read when needed.

Door visibility controls how the client presents accessible doors:

| Visibility | Meaning |
|---|---|
| `public` | Listed in the client door rail. |
| `private` | Accessible by policy but hidden from the normal door rail. |
| `hidden` | Accessible by policy but hidden from the normal door rail. |

Door access is enforced by `phosphord` when listing and opening doors. Supported modes:

| Access | Meaning |
|---|---|
| `public` | Any authenticated station user can open the door. |
| `invite_only` | Only allowlisted public keys or fingerprints can open the door. |
| `admin` | Only `admin` or `sysop` sessions can open the door. |

Invite-only door:

```toml
id = "members"
name = "Members"
entry = "app.lua"
visibility = "private"
access = "invite_only"
allowlist = [
  "base64-ed25519-public-key",
]
```

Admin-only door:

```toml
id = "admin"
name = "Station Admin"
entry = "app.lua"
visibility = "public"
access = "admin"
```

## Client Files

Default client files:

```text
~/.config/phosphornet/passport.toml
~/.config/phosphornet/known_nodes.toml
```

Override paths when connecting:

```bash
go run ./cmd/phosphor connect \
  --addr wss://127.0.0.1:7707/ws \
  --passport ./dev/passport.toml \
  --known-nodes ./dev/known_nodes.toml
```

`passport.toml` contains the user's Ed25519 identity key. Do not commit real passports.

`known_nodes.toml` stores SSH-like key pins by node address. If a node key changes for the same address, the client refuses the connection.

For local testing after regenerating a node, replace the stored pin explicitly:

```bash
go run ./cmd/phosphor connect \
  --addr wss://127.0.0.1:7707/ws \
  --replace-known-node
```

Use this only when you intentionally replaced the node identity.

## Database

The node SQLite database stores:

- known users
- user profiles
- station policy, roles, moderation state, and door settings
- scoped door state
- legacy user door state

Known user records include `first_seen` and `last_seen` timestamps. `last_seen` is updated whenever a passport authenticates successfully.

Current scoped state model:

| Scope | Scope ID | Meaning |
|---|---|---|
| `user` | user public key | Per-user state for one door. |
| `room` | room ID | Shared room state for one door. |
| `global` | node global scope | Door-wide state across users and rooms. |

State values are JSON objects. `phosphord` applies state operations atomically. Global state writes require an `admin` or `sysop` role.

The Station Admin door can inspect scoped state summaries by door, scope, scope ID, byte size, and update timestamp. Cross-door state clearing is intentionally not exposed as a one-click action until a dedicated node-owned admin effect and confirmation flow exist.

`phosphord serve` logs the absolute database path and SQLite schema version on startup. See `docs/PhosphorNet_database_lifecycle.md` for backup, restore, migration, and deletion expectations.

## Switchboard Scaffold

The current `switchboard` command is a health-checkable relay/rendezvous scaffold:

```bash
go run ./cmd/switchboard serve --listen :7710
```

It currently serves:

```text
/healthz
```

The switchboard does not own passports, users, doors, or station data.

## Deployment Notes

For local or private nodes:

- bind `listen_addr` to localhost or a LAN address as appropriate.
- keep `node.toml` permissions restricted because it contains `private_key`.
- keep `database` on persistent storage.
- restart `phosphord` after changing node config or door manifests.

For public nodes:

- set `[tls].enabled = false` only when you intentionally want plain `ws://` and `http://` for a local or proxied deployment.
- expose the WebSocket path as `/ws`.
- preserve the client/node security boundary: the node sends JSON UI, not terminal control output or client code.
