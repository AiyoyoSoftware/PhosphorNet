# PhosphorNet Lua Door API Reference

## Scope

This is the callable API reference for Lua doors hosted by `phosphord`. For a
tutorial and manifest examples, see `PhosphorNet_authoring_doors.md`. For the
runtime-independent wire contract, authorization rules, and effect schemas, see
`PhosphorNet_runtime_contract.md`.

Lua is embedded through `gopher-lua`. A door receives a context table, returns a
declarative UI table, and requests node-owned side effects through helper calls.
It does not render terminal output or access the client directly.

Both method forms are supported for context helpers:

```lua
ctx.store:get("user", "draft", "")
ctx.store.get("user", "draft", "")
```

The examples below use the colon form.

## Module And Globals

Every invocation defines:

```lua
ctx                    -- the current invocation context
phosphornet.ui         -- UI constructor table
```

A typical door aliases the UI table once:

```lua
local ui = phosphornet.ui
```

The door file is loaded afresh for each lifecycle invocation. Do not depend on
Lua globals surviving between calls; use `ctx.store` for durable data.

## Lifecycle API

A door may define any of these global functions:

```lua
function init(ctx) end
function view(ctx) end
function update(ctx, event) end
function on_join(ctx) end
function on_leave(ctx) end
function tick(ctx) end
```

| Function | Current `phosphord` behavior | Arguments | Return use |
|---|---|---|---|
| `init` | Accepted by the runtime invoker, but not automatically called by the current node | `ctx` | Reserved |
| `view` | Called when the node needs the current UI | `ctx` | Sent as the render tree |
| `update` | Called for a validated client event | `ctx`, `event` | Sent as the next render tree |
| `on_join` | Called when a session enters the door room | `ctx` | Ignored; effects are applied |
| `on_leave` | Called when a session leaves after any reconnect grace period | `ctx` | Ignored; effects are applied |
| `tick` | Accepted by the runtime invoker, but the current node has no tick scheduler | `ctx` | Reserved |

The Lua invoker always accepts one return value. Returning `nil`, or omitting a
lifecycle function, produces an empty `screen` in the runtime response. Only
the `view` and `update` return trees are rendered by the current node; hook code
should return `nil` and use effects. Effects requested before returning are
still collected. `update` is the only lifecycle that receives a second
argument.

## Context

The context exposes these fields:

| Field | Contents |
|---|---|
| `ctx.session` | Current session, including `id` |
| `ctx.user` | `public_key`, `fingerprint`, `role`, and profile fields |
| `ctx.node` | Station metadata, including `id`, `name`, fingerprint, access/maintenance state, and doors visible to the session |
| `ctx.room` | Implicit room `id` and `door_id` |
| `ctx.states` | Scoped state tables: `user`, `room`, and `global` |
| `ctx.state` | Compatibility alias for `ctx.states.user` |
| `ctx.settings` | Read-only resolved manifest settings |
| `ctx.presence` | `room_users`, `door_users`, and, when available, `all_users` |
| `ctx.permissions` | Session role, granted capabilities, global-write flag, and muted state |
| `ctx.users` | Known station users, without admin-only first/last-seen details |
| `ctx.admin` | Station administration snapshot when authorized; otherwise `nil` |
| `ctx.store` | Scoped state helper methods |
| `ctx.effects` | Structured side-effect helper methods |
| `ctx.nav` | Per-user subview navigation helper methods |

Optional or unauthorized fields may be absent. Treat them defensively:

```lua
local name = (ctx.user and ctx.user.display_name) or "anonymous"
local users = (ctx.presence and ctx.presence.room_users) or {}
```

State included in `ctx.states` is filtered by manifest read capabilities.
Settings are resolved from manifest defaults plus operator overrides. They are
a read-only snapshot in this context: mutating `ctx.settings` does not persist.
A privileged admin door can request a persisted setting change through the
corresponding admin operation.

Admin-only storage summaries, runtime events, operational user details, policy,
and runtime configuration are exposed below `ctx.admin` only when the session
has an `admin` or `sysop` role and the door declares the corresponding
`admin:read_*` capability. The protocol still contains legacy top-level
`storage` and `events` fields, but the current node does not populate them; door
code should use `ctx.admin.storage` and `ctx.admin.events`.

## Events

An `update` event has this shape:

```lua
{
  kind = "action",  -- action, select, submit, key, or focus
  target = "send",  -- component id, when applicable
  action = "send_message",
  key = "Enter",    -- key events only
  values = { message = "hello" }
}
```

Prefer semantic `action`, `select`, and `submit` events. Raw `key` delivery
requires `screen.capture_keys = true` and the manifest capability
`capture_keys`; trusted client shortcuts still take precedence.

`phosphord` may also call `update` with an internal `action_result` event after
a requested host action completes. This kind is node-generated and cannot be
sent by a client:

```lua
{
  kind = "action_result",
  action_result = {
    request_id = "status-1",
    rule_id = "host-status",
    ok = true,
    exit_code = 0,
    stdout = "ready\n",
    stderr = "",
    timed_out = false,
    truncated = false,
    error = nil,
  }
}
```

## UI Constructors

All constructors return ordinary Lua tables. A door can add supported fields to
the returned table before returning it.

### Containers And Text

```lua
ui.screen(children?, style?)
ui.panel(title, children?, style?)
ui.log(id, children?, style?)
ui.header(text)
ui.text(text)
ui.markdown(text)
ui.status(text)
```

`children` is an array of UI nodes. `style`, where accepted, is a style table.
Useful screen fields include:

```lua
local screen = ui.screen({...})
screen.scroll = "bottom"
screen.capture_keys = true
```

`scroll = "bottom"` is a transcript hint. The client preserves a user's manual
scroll position when they have scrolled upward.

### Controls

```lua
ui.button(id, text, action?)
ui.checkbox(id, text, checked, action?)
ui.input(id, placeholder?, value?, dock?)
ui.textarea(id, placeholder?, value?, dock?)
```

`dock` currently supports the bottom-composer hint `"bottom"`. Submitting an
input or textarea produces a `submit` event with exactly one entry in
`event.values`, keyed by that component's id. Activating a button or checkbox
copies all current input values into its `action` event; checkbox events also
include `values.checked` with the proposed new boolean as a string.

### Navigation Controls

```lua
ui.nav_button(id, text, view)
ui.back_button(id, text?)
```

These are buttons whose actions are generated as `nav:push:<view>` and
`nav:back`. Pass their events to `ctx.nav:handle` in `update`.

### Collections

```lua
ui.grid(id, rows)
ui.menu(id, items?)
ui.list(id, items?)
ui.dynamic_list(id, items?)
ui.item(label, action?)
```

`rows` is an array of string arrays. Collection items are tables containing
`label` and an optional `action`; `ui.item` constructs that shape. Activating a
`menu` item produces an `action` event. Activating a `list` item produces a
`select` event with its label in `values.label`. Each `dynamic_list` item acts
like a button and produces an `action` event with `values.label` plus the
current input values.

### Styles

`screen`, `panel`, and `log` accept a style table. The currently typed style is
a solid or gradient background. Colors must use `#rrggbb`; gradient direction
may be `vertical`, `horizontal`, or `diagonal`, and a gradient may use `from`
and `to` or up to eight stops whose `at` values are between `0` and `1`:

```lua
local style = {
  background = {
    kind = "gradient",
    direction = "vertical",
    from = "#101820",
    to = "#203040",
    stops = {
      { at = 0.0, color = "#101820" },
      { at = 1.0, color = "#203040" },
    },
  },
}
```

`phosphord` validates the returned root (which must be `screen`), component
fields, styles, tree depth, text lengths, and collection limits before sending
it. The trusted client also sanitizes remote content and negotiates its render
limits during session setup.

## State API

State scopes are `"user"`, `"room"`, and `"global"`. The node populates each
scope only when the manifest has the corresponding `state:*:read` capability;
without it, reads see an empty table or the supplied fallback. Every write is
rejected unless the manifest has the corresponding `state:*:write` capability.
Global writes additionally require an `admin` or `sysop` session.

```lua
ctx.store:get(scope, key, fallback?) -> value
ctx.store:set(scope, key, value)
ctx.store:delete(scope, key)
ctx.store:clear(scope)
ctx.store:replace(scope, table)
ctx.store:append(scope, key, item, limit?) -> items
ctx.store:all(scope) -> table
```

The helpers update the invocation's in-memory state immediately, so a later
read in the same lifecycle sees the change. They also emit typed state
operations for `phosphord` to validate and commit atomically.

`append` treats a missing or non-array value as an empty array, appends `item`,
and, when `limit > 0`, removes oldest items until the limit is met.

The lower-level equivalents are available on `ctx.effects`:

```lua
ctx.effects:set_state(scope, key, value)
ctx.effects:delete_state(scope, key)
ctx.effects:clear_state(scope)
ctx.effects:replace_state(scope, table)
```

Prefer `ctx.store` for ordinary door code.

For compatibility, direct mutations to the shared user table through
`ctx.state` or `ctx.states.user` are detected and persisted if no explicit
user-scope state operation was emitted. If an explicit user operation exists,
unrelated direct mutations are not captured automatically. New code should use
`ctx.store` exclusively so every mutation is represented by an operation.

## Navigation API

Subview navigation is stored in user state under the reserved key
`__nav_stack`; the door must have user-state read/write capabilities.

```lua
ctx.nav:current(fallback?) -> view
ctx.nav:push(view) -> view
ctx.nav:back(fallback?) -> view
ctx.nav:reset(view?) -> view
ctx.nav:handle(event, fallback?) -> handled
```

Defaults use `"main"`. `handle` recognizes `nav:back`, `nav:push:<view>`, and
`nav:reset:<view>` actions.

```lua
function update(ctx, event)
  if ctx.nav:handle(event, "home") then
    return view(ctx)
  end
  -- door-specific actions
  return view(ctx)
end
```

## Effect API

Effects are requests. `phosphord` validates their schema, target, session role,
and, where the effect family requires one, manifest capability before applying
them.

### Notifications

```lua
ctx.effects:notify(message, level?, target?, user_public_key?)
```

Defaults are `level = "info"` and `target = "self"`. Targets are `self`,
`room`, `door`, `user`, and `all`. A `user` target needs its public key. The
manifest needs the corresponding `notify:*` capability. Explicit `room`,
`door`, and `all` notifications go to matching peers and omit the source
session; use a separate `self` notification when the sender should also see it.

### Broadcasts

```lua
ctx.effects:broadcast(event, scope?, door_id?, room_id?, user_public_key?)
```

The default scope is `room`; valid scopes are `room`, `door`, and `user`.
`event` is decoded into the typed UI event fields shown earlier. Broadcasts
commonly trigger peer rerenders after shared state changes. In the current node
the event payload is not passed to a target door's `update`; matching peer
sessions are rerendered by invoking their active door's `view`. The source
session is omitted because its `update` return already supplies its next view.
Broadcasts returned by `view` are not fanned out, which prevents render loops;
emit them from `update`, `on_join`, or `on_leave`.

```lua
ctx.effects:broadcast({
  kind = "action",
  target = "chat",
  action = "room_changed",
}, "room")
```

### Transitions

```lua
ctx.effects:transition(kind, door_id?, room_id?)
```

Declared kinds are `open_door`, `close_door`, and `room`. Only `open_door` is
implemented end to end in the current MVP and requires `transition:open_door`.

```lua
ctx.effects:transition("open_door", "chat")
```

### Host Actions

```lua
ctx.effects:action(rule_id, request_id, input?)
```

This requests one fixed `phosphor-actiond` rule. It is valid only from
`update`, requires the manifest capability `action:<rule_id>`, and is also
checked against the rule's `allowed_doors` list. `input` must be JSON-shaped
and is written to the command's stdin; it never changes the configured argv.

```lua
ctx.effects:action("host-status", "status-1", {format = "short"})
```

Handle the node-generated result event described above and correlate it with
`request_id`. One action is allowed per runtime response, with a bounded chain
of action callbacks.

### Profile Updates

```lua
ctx.effects:update_profile(display_name?, bio?, status_line?)
ctx.effects:reset_profile()
```

Pass `nil` to leave an individual profile field unchanged. The node validates
profile values before saving them. `reset_profile` clears the display name,
biography, and status line.

### Admin Operations

```lua
ctx.effects:admin_op({ op = "operation_name", ... })
```

Admin operations are intentionally schema-driven rather than separate Lua
functions. They require an `admin` or `sysop` session and the matching
`admin:*` manifest capability. See `PhosphorNet_runtime_contract.md` for the
current operation schemas and authority table.

## Sandbox

The default strict profile opens Lua's base, table, string, and math libraries.
The standard profile also opens coroutine support. `collectgarbage`, `dofile`,
and `loadfile` are removed. OS, IO, package loading, debug access, sockets, and
arbitrary filesystem access are not part of the door API.

An explicit `[sandbox].libraries` list overrides a profile's default library
set, but only `base`, `coroutine`, `table`, `string`, and `math` are recognized.
The legacy `unsafe` profile name is accepted for configuration compatibility but
is normalized to the strict profile.

Execution is bounded by node/manifest timeout, memory, call-stack, and registry
settings. The sandbox is an MVP-grade language sandbox, not a general host
isolation boundary.

## Complete Example

```lua
local ui = phosphornet.ui

local function messages(ctx)
  return ctx.store:get("room", "messages", {})
end

function view(ctx)
  local lines = {}
  for _, message in ipairs(messages(ctx)) do
    table.insert(lines, ui.text(message))
  end

  local screen = ui.screen({
    ui.header("NOTES"),
    ui.log("messages", lines),
    ui.input("message", "Write a note", "", "bottom"),
    ui.button("send", "Send", "send"),
  })
  screen.scroll = "bottom"
  return screen
end

function update(ctx, event)
  if event and event.action == "send" then
    local message = event.values and event.values.message or ""
    if message ~= "" then
      ctx.store:append("room", "messages", message, 100)
      ctx.effects:broadcast({
        kind = "action",
        target = "notes",
        action = "room_changed",
      }, "room")
    end
  end
  return view(ctx)
end
```

The manifest for this example needs at least:

```toml
capabilities = [
  "state:room:read",
  "state:room:write",
  "broadcast:room",
]
```
