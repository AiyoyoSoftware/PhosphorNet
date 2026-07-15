# PhosphorNet Door Authoring Guide

## Purpose

This guide explains how to author PhosphorNet content as doors.

For exact Lua function signatures and fields, see
`docs/PhosphorNet_lua_api.md`.

A door is a server-side program hosted by `phosphord`. It receives a typed context, returns a declarative UI tree, and requests side effects through structured effects. The client renders the UI locally and sends semantic events back.

The boundary is the product:

```text
Client renders. Node thinks. Doors define behavior.
```

## Choose A Runtime

Use Lua for new doors by default:

- embedded in `phosphord`
- small and script-like
- strict sandbox by default
- no Python process startup cost

Use Python when:

- a proof door already uses the Python SDK
- a richer script benefits from Python syntax or libraries
- you explicitly set `runtime = "stdio"` with a command or Podman image in the manifest

Do not make each door a web server. Doors are invoked by `phosphord` through the runtime contract.

## Door Directory

Create one directory per door under the configured `doors_dir`:

```text
doors/
  hello/
    manifest.toml
    app.lua
```

Minimal manifest:

```toml
id = "hello"
name = "Hello"
entry = "app.lua"
visibility = "public"
access = "public"
```

Restart `phosphord` after adding or changing a manifest.

Door visibility:

| Value | Meaning |
|---|---|
| `public` | Show the door in the client door rail. |
| `private` | Hide the door from the normal door rail. |
| `hidden` | Hide the door from the normal door rail. |

Door access:

| Value | Meaning |
|---|---|
| `public` | Any authenticated station user can open the door. |
| `invite_only` | Only allowlisted public keys or fingerprints can open the door. |
| `admin` | Only `admin` or `sysop` sessions can open the door. |

Invite-only door example:

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

Admin-only door example:

```toml
id = "admin"
name = "Station Admin"
entry = "app.lua"
visibility = "public"
access = "admin"
capabilities = [
  "admin:read_station",
  "admin:read_users",
  "admin:set_door_policy",
  "admin:reload_manifests",
  "notify:all",
]
```

Admin-only access does not make a door powerful by itself. Privileged effects require an `admin` or `sysop` session and the matching manifest capability.

Common capabilities:

```text
state:user:read
state:user:write
state:room:read
state:room:write
state:global:read
state:global:write
broadcast:room
broadcast:door
broadcast:user
notify:self
notify:room
notify:door
notify:user
notify:all
capture_keys
transition:open_door
action:<rule-id>
admin:read_station
admin:read_users
admin:read_doors
admin:read_runtime
admin:read_storage
admin:read_logs
admin:set_user_roles
admin:set_station_access
admin:set_door_policy
admin:set_door_settings
admin:reload_manifests
admin:reorder_doors
admin:set_station_notice
admin:set_maintenance
admin:moderate_users
```

The older `permissions = [...]` manifest field is deprecated. `phosphord` still maps known legacy values such as `shared_state`, `raw_keys`, `global_state`, and `maintenance` to capabilities for compatibility.

## Door Settings

Doors can declare operator-editable settings in `manifest.toml`.

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

The manifest declares the schema and defaults. Station admins edit live values from the Station Admin door, and `phosphord` stores those edits in SQLite-backed node state. The manifest file is not rewritten.

Supported setting types:

| Type | Use |
|---|---|
| `string` | Single-line text. |
| `textarea` | Longer plain text. |
| `bool` | True/false toggles. |
| `int` | Whole numbers. |
| `select` | One string from `options = [...]`. |
| `markdown` | Longer Markdown text. |

Lua doors read resolved values from `ctx.settings`:

```lua
local motd = ctx.settings.motd or "Welcome."
local show_online = ctx.settings.show_online_users ~= false
```

Python doors use the same resolved values as a dictionary:

```python
motd = ctx.settings.get("motd", "Welcome.")
show_online = ctx.settings.get("show_online_users", True)
```

## Minimal Lua Door

```lua
local ui = phosphornet.ui

function view(ctx)
  return ui.screen({
    ui.header("HELLO"),
    ui.panel("Welcome", {
      ui.text("Station: " .. (ctx.node.name or "unknown")),
      ui.text("You are " .. (ctx.user.fingerprint or "unknown")),
      ui.button("ping-button", "Ping", "ping"),
    }),
    ui.status("Lua door loaded.")
  })
end

function update(ctx, event)
  if event and event.action == "ping" then
    ctx.effects.notify("pong")
  end
  return view(ctx)
end
```

Lua lifecycle functions may return `nil`; missing lifecycle functions are treated as no-ops.

## Minimal Python Door

Manifest:

```toml
id = "hello_python"
name = "Hello Python"
runtime = "stdio"
command = ["python3", "app.py"]
visibility = "public"
access = "public"

[isolation]
mode = "host"
```

Door:

```python
import asyncio

from phosphornet import run_module, ui


async def view(ctx):
    return ui.screen(
        [
            ui.header("HELLO PYTHON"),
            ui.panel(
                "Welcome",
                [
                    ui.text(f"Station: {ctx.node.get('name', 'unknown')}"),
                    ui.text(f"You are {ctx.user.get('fingerprint', 'unknown')}"),
                    ui.button("ping-button", "Ping", action="ping"),
                ],
            ),
            ui.status("Python door loaded."),
        ]
    )


async def update(ctx, event):
    if event.get("action") == "ping":
        ctx.effects.notify("pong")
    return await view(ctx)


if __name__ == "__main__":
    asyncio.run(run_module(globals()))
```

## Podman-Isolated Stdio Door

For trusted local development, `mode = "host"` is the explicit direct-process opt-out. For third-party or otherwise untrusted stdio doors, keep `runtime = "stdio"` and provide a Podman image. Podman is the default when `isolation.mode` is omitted:

```toml
id = "weather"
name = "Weather"
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

The container still receives one canonical runtime request JSON document on stdin and must write one canonical runtime response JSON document to stdout. For image doors, package the door code inside the image; do not rely on the station's door directory being mounted into the container.

Python door images can use the bundled SDK image as a base:

```Dockerfile
FROM localhost/phosphornet/python-door-sdk:latest

COPY app.py /door/app.py
ENV PHOSPHORNET_DOOR_ENTRY=/door/app.py
```

Build the base image from the repository root:

```bash
podman build -t localhost/phosphornet/python-door-sdk:latest sdk/python
```

## Lifecycle Functions

Supported lifecycle names:

| Function | When it runs |
|---|---|
| `init` | Door startup or initialization pass. |
| `view` | Render the current UI. |
| `update` | Handle a client event. |
| `on_join` | A session enters the door room. |
| `on_leave` | A session leaves the door room. |
| `tick` | Reserved lifecycle name; the node does not currently schedule it. |

Each door currently provides one shared interaction scope. A session entering a door joins that scope.

## Runtime Context

Doors receive `ctx` with:

| Field | Meaning |
|---|---|
| `ctx.session` | Session metadata, including session ID. |
| `ctx.user` | User public key, fingerprint, and role. |
| `ctx.node` | Node ID and station name. |
| `ctx.room` | Current room ID and door ID. |
| `ctx.state` | Shortcut for user-scoped state. |
| `ctx.states.user` / `ctx.states["user"]` | User-scoped state. |
| `ctx.states.room` / `ctx.states["room"]` | Room-scoped state. |
| `ctx.states.global` / `ctx.states["global"]` | Door-global state. |
| `ctx.settings` | Resolved operator settings from the manifest defaults plus station admin edits. |
| `ctx.store` | Explicit state getter/setter helpers over user, room, and global scopes. |
| `ctx.nav` | Per-user subview stack helpers for multi-page doors. |
| `ctx.presence` | Live room and door presence snapshots. |
| `ctx.permissions` | Runtime permission metadata. |
| `ctx.admin` | Admin-only context, present only for admin/sysop sessions when the door has matching `admin:read_*` capabilities. |
| `ctx.effects` | Structured effect helpers. |

Lua uses table field access:

```lua
local visits = tonumber(ctx.state.visit_count or 0) or 0
```

Python uses dictionaries and the `ctx.state` property:

```python
visits = int(ctx.state.get("visit_count", 0))
```

## UI Components

Available protocol v1 components:

| Component | Lua helper | Python helper |
|---|---|---|
| `screen` | `ui.screen(children, style)` | `ui.screen(children, style=style)` |
| `header` | `ui.header(text)` | `ui.header(text)` |
| `text` | `ui.text(text)` | `ui.text(value)` |
| `markdown` | `ui.markdown(text)` | `ui.markdown(value)` |
| `status` | `ui.status(text)` | `ui.status(value)` |
| `panel` | `ui.panel(title, children, style)` | `ui.panel(title, children, style)` |
| `menu` | `ui.menu(id, items)` | `ui.menu(identifier, items)` |
| `list` | `ui.list(id, items)` | `ui.list(identifier, items)` |
| `dynamic_list` | `ui.dynamic_list(id, items)` | `ui.dynamic_list(identifier, items)` |
| `input` | `ui.input(id, placeholder, value)` | `ui.input(identifier, placeholder, value)` |
| `textarea` | `ui.textarea(id, placeholder, value)` | `ui.textarea(identifier, placeholder, value)` |
| `button` | `ui.button(id, label, action)` | `ui.button(identifier, label, action)` |
| `checkbox` | `ui.checkbox(id, label, checked, action)` | return dict manually for now |
| `log` | `ui.log(id, children, style)` | `ui.log(identifier, children, style)` |
| `grid` | `ui.grid(id, rows)` | `ui.grid(identifier, rows)` |

Use `ui.markdown()` for forum posts, help copy, rules, changelogs, and long-form station text. The trusted client renders markdown locally with its terminal markdown renderer; doors still send plain markdown source as text.

Container components that render other content, currently `screen`, `panel`, and `log`, may carry `style.background`. Buttons, inputs, menus, and other leaf or interactive controls keep their local trusted-client styling.

Gradient backgrounds support `direction = "vertical"`, `"horizontal"`, or `"diagonal"` and either `from` / `to` colors or up to eight ordered `stops`:

```lua
ui.panel("Station", {
  ui.text("PHOSPHOR LABS LOG"),
}, {
  background = {
    kind = "gradient",
    direction = "vertical",
    from = "#18122b",
    to = "#2b124c",
  },
})
```

```lua
ui.log("machine-room", rows, {
  background = {
    kind = "gradient",
    direction = "diagonal",
    stops = {
      { at = 0.0, color = "#111827" },
      { at = 0.5, color = "#312e81" },
      { at = 1.0, color = "#7c2d12" },
    },
  },
})
```

`screen` may also set `capture_keys = true` when the door explicitly needs raw key events while focused. Trusted client shortcuts still win, focused text inputs keep their local typing behavior, and `phosphord` rejects key capture unless the manifest declares `capture_keys`.

Menus use items:

```lua
ui.menu("main-menu", {
  ui.item("Record Visit", "record_visit"),
  ui.item("Reset", "reset"),
})
```

```python
ui.menu(
    "main-menu",
    [
        ui.item("Record Visit", action="record_visit"),
        ui.item("Reset", action="reset"),
    ],
)
```

Keep UI trees semantic and small. Do not encode raw ANSI output or terminal layouts.

Transcript-style doors can ask the trusted client to open at the bottom:

```python
return ui.screen([
    ui.log("chat-log", lines),
    ui.input("chat-message", "/msg #station", dock="bottom"),
], scroll="bottom")
```

The client keeps bottom-pinned views at the bottom only while the user is already at the bottom; scrolling upward preserves the user's position. `dock="bottom"` is explicit door intent for the final input or textarea to sit at the viewport bottom when the transcript is shorter than the viewport.

## Events

Allowed event kinds:

```text
action
select
submit
key
focus
```

Common event fields:

| Field | Meaning |
|---|---|
| `kind` | Event kind. |
| `target` | Component ID. |
| `action` | Semantic action string from a button or menu item. |
| `key` | Raw key value when key capture is explicitly used. |
| `values` | Submitted input and textarea values by component ID. |

Button example:

```lua
if event.action == "save" then
  ctx.effects.notify("Saved.")
end
```

Key-capture example:

```lua
local screen = ui.screen({
  ui.header("ORDER"),
  ui.text("Press = or - to reorder while this panel is focused."),
})
screen.capture_keys = true
return screen
```

```lua
if event.kind == "key" and event.key == "=" then
  -- handle plus key
end
```

Input submit example in Python:

```python
if event.get("kind") == "submit" and event.get("target") == "message":
    text = event.get("values", {}).get("message", "").strip()
```

Prefer `action`, `select`, and `submit`. Use raw `key` only for doors that genuinely need it.

## State

State scopes:

| Scope | Use for |
|---|---|
| `user` | preferences, drafts, personal counters, per-user progress |
| `room` | chat messages, shared board state, players, room topic |
| `global` | door-wide settings or indexes requiring `state:global:*` capabilities and admin/sysop writes |

Lua user-state shortcut:

```lua
ctx.state.visit_count = (tonumber(ctx.state.visit_count or 0) or 0) + 1
```

Structured room-state write:

```lua
ctx.effects.set_state("room", "topic", "general")
```

Python room-state write:

```python
ctx.effects.set_state("room", "messages", messages[-30:])
```

Explicit state helper examples:

```lua
local posts = ctx.store:get("room", "posts", {})
ctx.store:append("room", "posts", {
  from = ctx.user.fingerprint,
  title = "First post",
  body = "Hello from the board.",
}, 100)
ctx.store:set("user", "profile", { display_name = "Ada" })
```

```python
posts = ctx.store.get("room", "posts", [])
ctx.store.append(
    "room",
    "posts",
    {
        "from": ctx.user.get("fingerprint", "unknown"),
        "title": "First post",
        "body": "Hello from the board.",
    },
    limit=100,
)
ctx.store.set("user", "profile", {"display_name": "Ada"})
```

Available state operations:

| Operation | Helper |
|---|---|
| `get` | `ctx.store:get(scope, key, fallback)` / `ctx.store.get(scope, key, default)` |
| `set` | `ctx.store:set(scope, key, value)` / `ctx.store.set(scope, key, value)` |
| `append` | `ctx.store:append(scope, key, value, limit)` / `ctx.store.append(scope, key, value, limit=None)` |
| `delete` | `ctx.store:delete(scope, key)` / `ctx.store.delete(scope, key)` |
| `clear` | `ctx.store:clear(scope)` / `ctx.store.clear(scope)` |
| `replace` | `ctx.store:replace(scope, value)` / `ctx.store.replace(scope, value)` |
| `all` | `ctx.store:all(scope)` / `ctx.store.all(scope)` |

`phosphord` applies state operations atomically. If one operation is invalid or unauthorized, none of the operations in that response are committed.

`ctx.effects.set_state`, `delete_state`, `clear_state`, and `replace_state` remain available for lower-level effect code. For ordinary durable door data like forum posts, guestbook entries, room topics, and per-user preferences, prefer `ctx.store`.

## Subviews

Doors can build multiple pages by rendering different content from a per-user navigation stack. The client still renders one declarative tree; the door owns the current subview in user-scoped state.

Lua:

```lua
local ui = phosphornet.ui

function view(ctx)
  local page = ctx.nav:current("home")
  if page == "posts" then
    return ui.screen({
      ui.header("POSTS"),
      ui.back_button("back"),
      ui.panel("Posts", { ui.text("Forum posts go here.") }),
    })
  end

  return ui.screen({
    ui.header("FORUM"),
    ui.nav_button("open-posts", "Posts", "posts"),
  })
end

function update(ctx, event)
  if ctx.nav:handle(event, "home") then
    return view(ctx)
  end
  return view(ctx)
end
```

Python:

```python
from phosphornet import ui


async def view(ctx):
    page = ctx.nav.current("home")
    if page == "posts":
        return ui.screen(
            [
                ui.header("POSTS"),
                ui.back_button("back"),
                ui.panel("Posts", [ui.text("Forum posts go here.")]),
            ]
        )
    return ui.screen([ui.header("FORUM"), ui.nav_button("open-posts", "Posts", "posts")])


async def update(ctx, event):
    if ctx.nav.handle(event, default="home"):
        return await view(ctx)
    return await view(ctx)
```

Navigation actions are semantic button actions:

| Action | Meaning |
|---|---|
| `nav:push:<view>` | Push a subview name onto the current user's door-local stack. |
| `nav:back` | Pop one subview, or return to the provided default. |
| `nav:reset:<view>` | Replace the stack with one subview. |

## Effects

Doors request side effects; `phosphord` applies them.

Notify:

```lua
ctx.effects.notify("Message sent.", "info", "self")
```

```python
ctx.effects.notify("Message sent.", level="info", target="self")
```

Notify targets:

```text
self
room
door
user
all
```

Broadcast:

```lua
ctx.effects.broadcast({ kind = "action", target = "chat", action = "room_changed" }, "room")
```

```python
ctx.effects.broadcast({"kind": "action", "target": "chat", "action": "room_changed"}, scope="room")
```

Broadcast scopes:

```text
room
door
user
```

Current broadcast behavior re-renders matching live sessions after shared state changes.

Protocol version 1 supports `open_door` transitions, which are applied by
`phosphord`. The other declared transition values are reserved and doors must
not emit them.

Host actions are fixed commands owned by the station operator and executed by the separate `phosphor-actiond` process. A door requests a rule by ID; it never supplies executable or argument text.

Lua:

```lua
ctx.effects.action("host-status", "status-1", {format = "short"})
```

Python:

```python
ctx.effects.action("host-status", "status-1", {"format": "short"})
```

The manifest needs `action:host-status`, and the actiond rule must independently include the door ID in `allowed_doors`. The optional input is serialized as JSON to the fixed command's stdin.

The result returns through a node-generated `action_result` update:

```lua
function update(ctx, event)
  if event.kind == "action_result" and event.action_result.request_id == "status-1" then
    local result = event.action_result
    if result.ok then
      ctx.store:set("user", "last_status", result.stdout)
    else
      ctx.effects.notify(result.error or "Action failed", "error")
    end
    return view(ctx)
  end

  if event.action == "run_status" then
    ctx.effects.action("host-status", "status-1", {format = "short"})
  end
  return view(ctx)
end
```

Action effects are accepted only from `update`, one per response. `phosphord` bounds callback chains, and actiond caps execution time and captured output. Treat stdout and stderr as untrusted command output before presenting them in the UI.

For doors with multiple actions, map semantic UI actions to fixed rule IDs in door code. Never take a rule ID from `event.values`, a text input, or another user-controlled field:

```lua
local choices = {
  run_uptime = {rule_id = "demo-uptime", selection = "uptime"},
  run_disk_usage = {rule_id = "demo-disk-usage", selection = "disk_usage"},
}

function update(ctx, event)
  if event.kind == "action_result" then
    -- Correlate event.action_result.request_id and handle bounded output.
    return view(ctx)
  end

  local choice = choices[event.action or ""]
  if choice then
    ctx.effects.action(choice.rule_id, "tools:" .. choice.selection, {
      source = "tools",
      selection = choice.selection,
    })
  end
  return view(ctx)
end
```

The shipped `doors/action_demo/` door is the complete version of this pattern. It demonstrates three typed choices, result persistence, failure display, and UI output clipping, and bundles its matching rules as `doors/action_demo/actiond.example.toml`. It starts disabled; copy or activate those rules as described in `docs/PhosphorNet_configuration.md` before enabling it.

## Presence

Presence is live and in-memory. It is included in the runtime context:

Lua:

```lua
for _, user in ipairs(ctx.presence.room_users or {}) do
  local fingerprint = user.fingerprint or "unknown"
end
```

Python:

```python
for user in ctx.presence.get("room_users", []):
    fingerprint = user.get("fingerprint", "unknown")
```

Presence is useful for room rosters, join/leave notices, and multiplayer proofs. It is not durable state.

Reconnects create fresh sessions. During a short disconnect grace window, `phosphord` can reopen the user's previous door if it is still safe and cancel the pending `on_leave`, so door authors should treat lifecycle hooks as live-room signals rather than durable attendance history. Scroll position, focus, and unsent client input are not restored by the node.

## Security Rules For Door Authors

Do:

- return semantic UI components
- use structured effects
- keep state JSON-shaped
- keep actions named and explicit
- keep doors inspectable

Do not:

- emit raw terminal escape sequences
- assume access to client files, shell, clipboard, or private keys
- build client-side scripting into UI payloads
- make a door depend on arbitrary filesystem access
- ask the client to render opaque blobs
- use `unsafe` Lua sandbox settings for hosted community content

Doors define behavior. The client remains trusted local software.

## Iteration Workflow

1. Create `doors/<id>/manifest.toml`.
2. Write the Lua entrypoint or stdio command.
3. Restart `phosphord`.
4. Connect with `phosphor`.
5. Open the door from the door rail.
6. Watch the node terminal for load/runtime errors.
7. Add focused tests when touching shared SDK, runtime, protocol, or storage behavior.

For Python doors, compile during development:

```bash
python3 -m py_compile doors/<id>/app.py
```

For Lua doors, keep the script small enough to inspect and exercise through the running node. Runtime-level behavior belongs in Go tests under `internal/runtime`.

## Content Patterns

Good doors:

- lobby or welcome station
- shared chat room
- turn-based game
- guestbook
- bulletin board
- small dashboard
- collaborative checklist
- puzzle room

Keep the first version boring, typed, and easy to debug. A good PhosphorNet door feels like a place, but it should still be built from small plain pieces.
