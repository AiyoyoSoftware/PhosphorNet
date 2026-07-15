# PhosphorNet Door Runtime Contract

## Purpose

This document defines the canonical protocol version 1 contract between `phosphord` and door runtimes.

The default runtime backend is embedded Lua through `gopher-lua`, and Python remains available as a stdio command when a door packages or imports the Python SDK itself. This contract is interpreter-agnostic: every runtime implements the same request and response envelope rather than adding runtime-specific message shapes.

Contract version:

```text
phosphornet.door.runtime.v1
```

Client sessions also advertise compatibility in the WebSocket `hello` before auth. The node expects the current runtime protocol version, JSON UI schema version, supported components, supported style features, supported event kinds, and render limits. If the client cannot render the current contract, `phosphord` rejects the session with a typed `client_incompatible` error instead of sending a render tree the client may misinterpret.

## Lifecycle

`phosphord` invokes doors through these lifecycle names:

```text
init
view
update
on_join
on_leave
tick
```

Lifecycle meaning:

- `init`: initialize door-owned state or emit startup effects.
- `view`: return the current declarative UI tree.
- `update`: handle a scoped UI event from the client.
- `on_join`: handle a user/session entering the door's room.
- `on_leave`: handle a user/session leaving the door's room.
- `tick`: handle scheduled node-driven time progression.

Missing lifecycle functions are treated as no-ops by the Lua invoker and Python stdio SDK helper.

## Reconnect And Session Recovery

Alpha reconnect behavior is intentionally small and explicit:

- A reconnect always authenticates as a new WebSocket session with a new session ID.
- If the previous connection dropped recently, `phosphord` may reopen the previous active door for the same passport when that door still exists and the new session can still access it.
- If the previous door is missing, disabled, or no longer accessible, the new session opens `lobby`.
- Reopening the previous door during the disconnect grace window does not run another `on_join` and cancels the delayed `on_leave`.
- If the user does not reconnect before the grace window expires, `on_leave` runs for the old active door and the old session is removed.
- Presence is live-only and in-memory. Disconnected sessions are not durable presence records.
- Client scroll position, focused component state, and input drafts are not recovered across reconnect.

The current default grace window is 5 seconds and node-owned. It exists to avoid noisy leave/rejoin churn during brief network drops, not to provide durable session resume.

## Request Envelope

Every runtime invocation receives one typed request envelope.

```json
{
  "contract_version": "phosphornet.door.runtime.v1",
  "door": {
    "id": "chat",
    "name": "Chat"
  },
  "lifecycle": "update",
  "ctx": {
    "session": {"id": "s1"},
    "user": {
      "public_key": "base64-ed25519-public-key",
      "fingerprint": "abcd1234",
      "role": "member"
    },
    "node": {
      "id": "base64-ed25519-node-key",
      "name": "localbox"
    },
    "room": {
      "id": "door:chat",
      "door_id": "chat"
    },
    "state": {
      "user": {},
      "room": {},
      "global": {}
    },
    "settings": {
      "motd": "Welcome to this PhosphorNet station.",
      "show_online_users": true
    },
    "presence": {
      "room_users": [],
      "door_users": []
    },
    "permissions": {
      "role": "member",
      "capabilities": [
        "state:room:read",
        "state:room:write"
      ],
      "can_write_global": false
    }
  },
  "event": {
    "kind": "action",
    "target": "chat-actions",
    "action": "send_message"
  }
}
```

`event` is present for `update` and omitted for lifecycle hooks that do not handle client input.

## Event Types

Allowed event kinds:

```text
action
select
submit
key
focus
```

Doors should prefer semantic events such as `action`, `select`, and `submit`. Raw `key` events should only be used when a door explicitly needs key capture.

`screen.capture_keys = true` opts the active door into raw key delivery while the remote viewport is focused. Trusted client chrome shortcuts still take precedence. `phosphord` rejects key-capture screens unless the door manifest declares `capture_keys`.

## State Scopes

State is loaded into three scopes:

```text
user
room
global
```

Scope meaning:

- `user`: per-door, per-user state.
- `room`: per-door shared state. Each door currently provides one shared interaction scope.
- `global`: per-door node-global state. Writes require an admin/sysop role and `state:global:write`.

State values are door-owned JSON objects. The contract around state mutation is typed through `state_ops`.

Door SDKs expose explicit getter/setter helpers over these same state objects. Lua doors can use `ctx.store:get`, `ctx.store:set`, `ctx.store:append`, `ctx.store:delete`, `ctx.store:clear`, `ctx.store:replace`, and `ctx.store:all`; Python doors use the same names as methods on `ctx.store`. These helpers still emit typed `state_ops`; they do not give doors direct database access.

`phosphord` filters state scopes by manifest read capabilities before invoking the door, and rejects write operations without the matching `state:*:write` capability.

## Settings

`ctx.settings` contains resolved operator settings for the current door. The door manifest declares setting keys, types, labels, options, and defaults; Station Admin edits live values; `phosphord` stores those edits in SQLite-backed node state and overlays them on top of manifest defaults before invoking the door.

Settings are read-only from a door's point of view. Doors that need ordinary durable user, room, or global data should continue to use `ctx.store` and `state_ops`.

## Response Envelope

Every runtime invocation returns one typed response envelope.

```json
{
  "contract_version": "phosphornet.door.runtime.v1",
  "view": {
    "component": "screen",
    "scroll": "bottom",
    "children": []
  },
  "state_ops": [],
  "broadcasts": [],
  "notifies": [],
  "transitions": [],
  "actions": [],
  "admin_ops": []
}
```

`screen.scroll = "bottom"` is an optional viewport hint for transcript-like doors. The trusted client scrolls to the bottom when opening that door, and keeps the viewport at the bottom on later renders only if the user was already at the bottom. If the user has scrolled upward, new renders preserve that scroll position. A final `input` or `textarea` may set `dock = "bottom"` to request a bottom composer when the transcript is shorter than the viewport.

## Effects

Doors request side effects by returning structured effects. `phosphord` owns effect application.

### State Ops

Allowed state operations:

```text
set
delete
clear
replace
```

Example:

```json
{
  "scope": "room",
  "op": "set",
  "key": "topic",
  "value": "general"
}
```

`phosphord` applies state operations atomically across scopes. If any operation is invalid or unauthorized, none of the operations are committed.

### Broadcasts

Broadcast scopes:

```text
room
door
user
```

Broadcast payloads carry a typed UI event. Fanout is owned by the node, not by door code.

### Notifies

Notify targets:

```text
self
room
door
user
all
```

The node decides which sessions receive notifications.

### Transitions

Transition kinds:

```text
open_door
close_door
room
```

Transitions describe navigation intent. The trusted client and node still own session boundaries.

Protocol version 1 supports `open_door`, which lets one door hand off to
another. `close_door` and `room` are reserved contract values and must not be
emitted by doors.

### Host Actions

Actions delegate a fixed, operator-defined host command to `phosphor-actiond`. They are allowed only from `update` responses and a response may request at most one action.

```json
{
  "request_id": "status-1",
  "rule_id": "host-status",
  "input": {"format": "short"}
}
```

The door manifest must declare the rule-specific capability:

```toml
capabilities = ["action:host-status"]
```

`phosphord` validates that capability, sends a `phosphornet.action.v1` JSON request over the configured Unix socket, and never executes the command itself. `phosphor-actiond` performs a second authorization check against the rule's explicit `allowed_doors` list.

When execution finishes or the daemon is unavailable, `phosphord` invokes the same door's `update` again with a node-generated event. Clients cannot submit this event kind.

```json
{
  "kind": "action_result",
  "action_result": {
    "request_id": "status-1",
    "rule_id": "host-status",
    "ok": true,
    "exit_code": 0,
    "stdout": "ready\n",
    "stderr": "",
    "timed_out": false,
    "truncated": false
  }
}
```

Transport failures and command failures are returned with `ok = false` and an `error` string so the door can render or persist a useful result. Callback responses may request another action, but `phosphord` bounds the chain to four executions. Action requests and outcomes are recorded in the durable audit trail without copying command stdout or stderr into audit metadata.

### Admin Ops

Admin operations are node-owned privileged effects. Doors request them with `admin_ops`, and `phosphord` validates role, manifest capability, schema, and target before mutating station policy or runtime admin state.

Example:

```json
{
  "op": "set_door_enabled",
  "door_id": "strategy_demo",
  "enabled": true
}
```

Current admin op authority:

| Admin op | Required role | Required capability | Target |
|---|---|---|---|
| `set_user_role` | `admin` or `sysop` | `admin:set_user_roles` | user role policy |
| `set_door_roles` | `admin` or `sysop` | `admin:set_door_policy` | door role policy |
| `set_door_enabled` | `admin` or `sysop` | `admin:set_door_policy` | door enabled policy |
| `set_door_setting` | `admin` or `sysop` | `admin:set_door_settings` | door setting override |
| `reload_manifests` | `admin` or `sysop` | `admin:reload_manifests` | runtime manifest registry |
| `reorder_doors` | `admin` or `sysop` | `admin:reorder_doors` | trusted navigation order |
| `set_station_notice` | `admin` or `sysop` | `admin:set_station_notice` | station notice log |
| `clear_station_notices` | `admin` or `sysop` | `admin:set_station_notice` | station notice log |
| `set_maintenance` | `admin` or `sysop` | `admin:set_maintenance` | station maintenance flag |
| `record_maintenance_checkpoint` | `admin` or `sysop` | `admin:set_maintenance` | maintenance counter |
| `reset_maintenance` | `admin` or `sysop` | `admin:set_maintenance` | maintenance state and notices |
| `clear_event_log` | `admin` or `sysop` | `admin:read_logs` | in-memory runtime event log |
| `ban_key` | `admin` or `sysop` | `admin:moderate_users` | station moderation denylist |
| `unban_key` | `admin` or `sysop` | `admin:moderate_users` | station moderation denylist |
| `mute_key` | `admin` or `sysop` | `admin:moderate_users` | station moderation mutelist |
| `unmute_key` | `admin` or `sysop` | `admin:moderate_users` | station moderation mutelist |
| `set_user_rate_limit` | `admin` or `sysop` | `admin:moderate_users` | per-user station rate limits |
| `record_moderation_note` | `admin` or `sysop` | `admin:moderate_users` | operator moderation notes |

`clear_event_log` uses `admin:read_logs`; this is the current protocol authority boundary.

Station policy is stored in node-owned SQLite state. Bundled Station Admin is only the UI that submits these operations.

`phosphord` also records durable audit events for security/operator mutations and denials. Audited events include admin role changes, door policy changes, door setting changes, manifest reloads, maintenance changes, event-log clearing, moderation operations, failed or denied auth, denied privileged door effects, door access denials, and node key changes detected at startup. Audit records are written to SQLite `audit_events` and may also be mirrored to a JSON Lines file with `phosphord serve --audit-log-file`. `--audit-log-max-bytes` applies to both SQLite retention and optional JSONL file rotation.

Public-station moderation primitives are tracked separately in `docs/PhosphorNet_public_station_moderation.md`. Ban, mute, unban, unmute, per-user rate-limit, and moderation-note operations are node-owned `admin_ops` guarded by `admin:moderate_users` rather than encoded as ordinary roles or arbitrary door state. Muted sessions receive `ctx.permissions.muted = true`; `phosphord` also rejects generic muted write-like events while allowing navigation actions.

### Door Subviews

Door subviews are door-owned render states, not client-side routes. SDK navigation helpers maintain a per-user `__nav_stack` in user-scoped state:

```text
ctx.nav.current(default)
ctx.nav.push(view)
ctx.nav.back(default)
ctx.nav.reset(view)
ctx.nav.handle(event, default)
```

UI helper buttons emit semantic actions such as `nav:push:posts`, `nav:back`, and `nav:reset:home`. The door handles those actions during `update` and returns the next declarative UI tree.

## Current Implementation Notes

- Lua uses this envelope through `internal/runtime/lua_invoker.go`.
- Lua doors receive a `ctx` table with `state`, `states`, `store`, `nav`, `effects`, and an embedded `phosphornet.ui` SDK table.
- The strict Lua sandbox opens only small safe libraries by default and can be configured at the node or door manifest level.
- Stdio doors receive the canonical request JSON on stdin and must write the canonical response JSON on stdout. Stderr is treated as runtime diagnostics.
- Stdio invocation is bounded by node timeouts, stdout/stderr byte caps, and strict JSON response decoding.
- Stdio doors may run as an explicit `mode = "host"` trusted process or through `mode = "podman"` isolation. Podman is only a launch wrapper around the same stdin/stdout JSON ABI.
- Podman-isolated image doors are expected to contain their code inside the image; `phosphord` does not mount the station's real door directory into arbitrary third-party containers by default.
- Python doors use the same stdio envelope through `sdk/python/phosphornet/runtime.py` and `run_module`, but they still declare `runtime = "stdio"` rather than a Python-specific runtime name.
- Python doors receive matching `ctx.store` and `ctx.nav` helper objects.
- Existing Python and Lua doors may continue mutating `ctx.state`; the runtime converts those user-scope mutations into `replace` state ops when no explicit user state op was already emitted.
- `phosphord` applies `state_ops`, `admin_ops`, routes `notifies`, and treats `broadcasts` as a signal to re-render matching live sessions after shared state changes.
- Door effects are checked against manifest capabilities before they are applied.
- Manifest capabilities describe PhosphorNet effect authority, not filesystem, network, process, or container authority. Host/container resources belong to `[isolation]`.
- Room presence snapshots are live in memory and are included in runtime context. They are not recovered as durable session state after reconnects or node restarts.
- Runtime and door failures use stable node-to-client `error.code` values: `runtime_not_available`, `runtime_image_missing`, `runtime_timeout`, `runtime_bad_output`, `runtime_denied_by_policy`, `runtime_resource_limit`, `manifest_invalid`, and `door_crashed`.
- `open_door` transitions are applied by `phosphord` so doors can hand off to another door without client-side routing.
- Other declared transition kinds should currently be treated as reserved contract space rather than shipped behavior.
- `markdown` is a supported declarative UI component rendered locally by the trusted client.
- Container UI components that carry `children`, currently `screen`, `panel`, and `log`, may include `style.background` with a trusted-client-rendered solid color or gradient.
- The canonical JSON UI contract is owned by `internal/protocol`.
- The formal schema lives at `internal/protocol/schema/json-ui-v1.schema.json`.
- Golden fixtures live under `internal/protocol/testdata/ui_contract/v1`.
- Trusted-client rendering expectations live under `internal/client/testdata/ui_contract/v1/render`.
- Runtime responses are validated against the typed Go contract before the node accepts door output.

## Runtime Error Taxonomy

Node-to-client errors carry `type = "error"` and a stable `code` when the node can classify the failure.

| Code | Meaning |
|---|---|
| `runtime_not_available` | The selected runtime or executable is unavailable, including missing Podman. |
| `runtime_image_missing` | Podman isolation was selected but no image was declared, the image is missing, or the image cannot be pulled. |
| `runtime_timeout` | A runtime invocation exceeded its timeout. |
| `runtime_bad_output` | The runtime wrote malformed stdout or a response outside the protocol contract. |
| `runtime_denied_by_policy` | The manifest, isolation mode, event, or effect was denied by node policy. |
| `runtime_resource_limit` | stdout, stderr, memory, or another bounded runtime resource exceeded its cap. |
| `manifest_invalid` | A door manifest is malformed or violates the manifest contract. |
| `door_crashed` | The door process or Lua lifecycle failed after launch. |

Stderr remains diagnostics. It may be included in operator-facing errors, but it is not a second protocol channel and is capped by `phosphord`.

Gradient backgrounds use strict hex colors and are presentation hints only:

```json
{
  "component": "panel",
  "id": "hero",
  "style": {
    "background": {
      "kind": "gradient",
      "direction": "vertical",
      "from": "#18122b",
      "to": "#2b124c"
    }
  },
  "children": [
    {
      "component": "text",
      "text": "PHOSPHOR LABS LOG"
    }
  ]
}
```
