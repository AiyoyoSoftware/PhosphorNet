# PhosphorNet Technology Stack

## 1. Stack Summary

PhosphorNet uses a small, inspectable, cross-platform stack that matches the
product philosophy: modern internals, old-network simplicity, and low
operational friction.

Core stack:

```text
Client / CLI:
  Go + Bubble Tea

Node daemon:
  Go

Relay / switchboard:
  Go

Door scripting:
  Lua by default, stdio commands for Python or other languages

Transport:
  WebSocket

Protocol:
  JSON

Identity:
  Ed25519

Persistence:
  SQLite

Configuration:
  TOML
```

Product mantra:

```text
phosphor renders.
phosphord thinks.
doors define behavior.
switchboard helps nodes connect.
Ed25519 identifies.
WebSocket carries the session.
SQLite remembers.
```

## 2. Technology Choices

| Layer | Choice | Reason |
|---|---|---|
| Client CLI/TUI | Go | Single binary, fast startup, cross-platform, reliable terminal support |
| TUI framework | Bubble Tea | Event loop maps naturally to JSON UI and event messages |
| Styling | Lip Gloss | Panels, borders, colors, old-school terminal chrome |
| Markdown rendering | Glamour | Useful for boards, docs, help screens, and posts |
| Node daemon | Go | Concurrency, networking, static binaries, low operational friction |
| Host action daemon | Go + Unix socket JSON | Keeps explicitly configured host command execution outside `phosphord` and independently policy-checked |
| Relay | Go | WebSocket forwarding/rendezvous is a good fit for Go |
| Door language | Lua default, stdio commands supported | Lua keeps common doors embedded in `phosphord`; stdio keeps Python and other command-style doors possible |
| Door runtime | Embedded `gopher-lua`, plus generic stdio backend with optional Podman isolation | Lua gives a mostly standalone node with configurable sandboxing; stdio speaks the same JSON envelope over stdin/stdout whether launched directly or through a container wrapper |
| Transport | WebSocket | Proxy-friendly and easy to inspect |
| Protocol format | JSON | Human-readable and interoperable across Go, Lua, Python, and other stdio runtimes |
| Database | SQLite | Single-file persistence with no separate database service |
| Config | TOML | Human-editable node and door manifests |
| Identity | Ed25519 | SSH-like public-key identity, small keys, fast signatures |
| Schema lifecycle | Versioned SQL files plus runtime bootstrap | Keeps schema history inspectable while startup enforces the current schema |
| Logging | Go `log/slog` | Standard library, structured logging, no extra dependency |

## 3. Go Dependencies

## 3.1 TUI Client

Current libraries:

```text
github.com/charmbracelet/bubbletea
github.com/charmbracelet/lipgloss
github.com/charmbracelet/glamour
```

Use cases:

```text
bubbletea
  Main TUI event loop.

lipgloss
  Layout, borders, colors, panels, and visual styling.

glamour
  Markdown rendering for posts, help screens, node docs, and door descriptions.
```

Interactive component state is owned by the trusted client. Glamour renders the
protocol's Markdown component locally.

## 3.2 WebSocket

Current library:

```text
github.com/coder/websocket
```

Reasons:

- Clean context support.
- Good fit for structured async handlers.
- Modern API.
- Works well with Go HTTP servers.

Current session paths:

```text
phosphor ↔ phosphord
```

## 3.3 CLI Framework

Current library:

```text
github.com/spf13/cobra
```

Example command shape:

```bash
phosphor passport create
phosphor passport show
phosphor connect wss://localhost:7707

phosphord init --name localbox
phosphord serve

phosphor-actiond init
phosphor-actiond serve

switchboard serve
```

Cobra is not exciting, but it is reliable and familiar.

## 3.4 Config

Current library:

```text
github.com/pelletier/go-toml/v2
```

Config files:

```text
~/.config/phosphornet/passport.toml
~/.config/phosphornet/known_nodes.toml
/etc/phosphornet/node.toml
./doors/chat/manifest.toml
```

TOML is used for:

- Node configuration.
- Door manifests.
- Known nodes.
- Development profiles.

## 3.5 SQLite

Current driver:

```text
modernc.org/sqlite
```

Reason:

- Avoids CGO.
- Easier cross-compilation.
- Better fit for single-binary distribution.

The pure-Go driver avoids CGO and supports straightforward cross-compilation.

## 3.6 Migrations

Schema history is documented as plain SQL:

```text
migrations/
  001_init.sql
  002_scoped_state.sql
  003_user_last_seen.sql
  004_user_profiles.sql
```

Runtime bootstrap in `internal/storage` enforces the current schema and reports
the SQLite schema version.

## 3.7 Logging

Use standard library:

```text
log/slog
```

Avoid pulling in heavier logging frameworks until there is a real need.

## 3.8 Crypto

Use Go standard library where possible:

```text
crypto/ed25519
crypto/rand
crypto/sha256
encoding/base64
```

Passports are currently protected by local filesystem permissions. Encrypted
at-rest passport storage is tracked in `docs/PhosphorNet_roadmap.md`.

## 4. Door Runtime Stack

Doors should be tiny Lua programs or stdio commands that expose a small lifecycle API. The node owns networking, identity, persistence, and rendering transport. Doors own behavior.

No Flask. No FastAPI. No embedded HTTP server per door.

## 4.1 Lua Door SDK

Lua is the default runtime and is embedded in `phosphord` through `github.com/yuin/gopher-lua`.

Example Lua door:

```lua
local ui = phosphornet.ui

function view(ctx)
  return ui.screen({
    ui.header("LOCALBOX"),
    ui.panel("Actions", {
      ui.button("record-visit", "Record Visit", "record_visit"),
    }),
  })
end

function update(ctx, event)
  if event.action == "record_visit" then
    ctx.state.visit_count = (ctx.state.visit_count or 0) + 1
    ctx.effects.notify("Recorded a visit.")
  end
  return view(ctx)
end
```

Default Lua sandbox:

- Opens only `base`, `table`, `string`, and `math`.
- Removes file-loading helpers from the base library.
- Does not open `io`, `os`, `debug`, `package`, or `channel`.
- Applies a context timeout and memory ceiling per invocation.
- Allows node-level and door-level sandbox overrides.

Door manifest example:

```toml
id = "lobby"
name = "Lobby"
entry = "app.lua"

[sandbox]
profile = "strict"
max_memory_kb = 65536
max_execution_ms = 5000
```

## 4.2 Python Door SDK

Python remains supported through the generic stdio runtime contract. There is no Python-specific runtime name; a Python door manifest runs the script as an argv command:

```toml
runtime = "stdio"
command = ["python3", "app.py"]

[isolation]
mode = "host"
```

SDK layout:

```text
sdk/python/phosphornet/
  __init__.py
  ui.py
  runtime.py
  ctx.py
```

Example Python door:

```python
from phosphornet import ui

async def view(ctx):
    return ui.screen([
        ui.header("LOCALBOX"),
        ui.menu("main", [
            ui.item("Chat", action="open:chat"),
            ui.item("Strategy Demo", action="open:strategy_demo"),
        ]),
    ])

async def update(ctx, event):
    return await view(ctx)
```

The SDK has no required third-party Python dependencies. Door authoring remains
script-like and lightweight.

## 4.3 Door Runtime Model

Current runtime:

```text
phosphord invokes embedded Lua doors in-process by default.
phosphord ↔ stdio door command
            canonical JSON request on stdin
            canonical JSON response on stdout
```

The runtime uses a bounded request/response contract:

```text
phosphord invokes door method.
door returns render tree.
phosphord sends render tree to phosphor.
```

Host actions are a separate boundary from door runtime invocation:

```text
door action effect
  → phosphord checks action:<rule-id>
  → phosphor-actiond checks rule.allowed_doors
  → fixed argv command receives JSON input on stdin
  → structured result returns as an internal door update
```

The action protocol is `phosphornet.action.v1` JSON over a local Unix socket. It is not a generic door runtime and does not accept executable or argv text from doors.

## 4.4 Door Stdio Invocation

Stdio is the ABI for non-Lua doors. Host direct execution and Podman isolation are explicit execution profiles beneath that ABI, not separate runtime protocols:

```text
phosphord
  -> stdio invoker
      -> host process mode
      -> podman process mode
          -> container stdin/stdout uses the same runtime contract
```

Example request from `phosphord` to a stdio door:

```json
{
  "contract_version": "phosphornet.door.runtime.v1",
  "door": {
    "id": "chat",
    "name": "Chat"
  },
  "lifecycle": "update",
  "ctx": {
    "session": {
      "id": "s1"
    },
    "user": {
      "public_key": "ed25519:abc",
      "fingerprint": "ABCD",
      "role": "member"
    }
  },
  "event": {
    "kind": "submit",
    "target": "message_form",
    "values": {
      "message": "hello"
    }
  }
}
```

Example response from a stdio door:

```json
{
  "contract_version": "phosphornet.door.runtime.v1",
  "view": {
    "component": "screen",
    "children": []
  },
  "state_ops": []
}
```

This is the canonical stdio request/response boundary.

## 5. Protocol Layer

Protocol structs should live in a shared Go package:

```text
internal/protocol
```

The JSON protocol should be typed, small, and strict.

Avoid building an untyped pseudo-browser. Use explicit message and component types.

## 5.1 Message Types

Client → Node:

```text
hello
auth
open_door
event
close_door
```

Node → Client:

```text
challenge
auth_ok
auth_denied
door_list
render
notify
error
```

## 5.2 UI Node Shape

A simple early Go shape:

```go
type UINode struct {
    Component string   `json:"component"`
    ID        string   `json:"id,omitempty"`
    Text      string   `json:"text,omitempty"`
    Items     []Item   `json:"items,omitempty"`
    Children  []UINode `json:"children,omitempty"`
}

type Item struct {
    Label  string `json:"label"`
    Action string `json:"action"`
}
```

Avoid too much of this:

```go
Props map[string]any `json:"props,omitempty"`
```

A generic `props` map is convenient, but it can turn the protocol into a loose mini-DOM. Use explicit fields where possible.

## 5.3 Protocol Boundary

The protocol must remain declarative.

Allowed:

```json
{
  "component": "button",
  "label": "Send",
  "action": "send_message"
}
```

Forbidden by the client trust boundary:

```json
{
  "component": "raw",
  "content": "\u001b]52;c;..."
}
```

No raw terminal output. No remote code. No arbitrary client-side scripting.

## 6. Repository Layout

Use a monorepo.

```text
phosphornet/
  cmd/
    phosphor/
      main.go
    phosphord/
      main.go
    switchboard/
      main.go

  internal/
    protocol/
    identity/
    knownnodes/
    tui/
    client/
    node/
    relay/
    storage/
    config/
    doors/
    runtime/

  sdk/
    python/
      phosphornet/
        __init__.py
        ui.py
        runtime.py
        ctx.py

  doors/
    lobby/
      manifest.toml
      app.lua
    chat/
      manifest.toml
      app.lua
    strategy_demo/
      manifest.toml
      app.lua

  migrations/
    001_init.sql

  docs/
    architecture.md
    protocol.md
    security.md
    technology-stack.md
```

## 7. Process Model

## 7.1 Direct Station Path

```text
phosphor  ──WebSocket──>  phosphord  ──embedded VM──>  Lua door
                                      └─stdin/stdout──> Python door
```

Direct WSS connections are the supported deployment path over local networks,
private VPNs, and operator-managed Internet routing. The optional
`switchboard` command is currently an experimental health-checkable foundation;
relay implementation work is tracked in `docs/PhosphorNet_roadmap.md`.

## 8. Deliberate Technology Exclusions

## 8.1 gRPC

PhosphorNet does not use gRPC.

JSON over WebSocket and JSON over stdin/stdout are enough. The system should be inspectable, hackable, and easy to debug.

## 8.2 WASM

PhosphorNet does not use WASM as a privileged runtime model. The supported
runtime model is:

```text
embedded Lua + strict configurable sandboxing
generic stdio command runtime for Python and other external processes
```

## 8.3 Cloudflare-First Design

PhosphorNet is self-hosted and is not designed around Cloudflare Workers.
WebSocket and JSON remain compatible with conventional proxies without making
a particular cloud provider part of the architecture.

## 9. Dependency Reference

The current direct Go dependencies are declared in `go.mod` and include:

```text
github.com/charmbracelet/bubbletea
github.com/charmbracelet/glamour
github.com/charmbracelet/lipgloss
github.com/coder/websocket
github.com/pelletier/go-toml/v2
github.com/spf13/cobra
github.com/yuin/gopher-lua
modernc.org/sqlite
```

The Python SDK has no required third-party dependencies.

## 10. Version Targets

Current version targets:

```text
Go:
  1.26+

Python:
  3.11+

SQLite:
  Bundled through Go driver

Protocol:
  JSON v1

Door SDK:
  embedded phosphornet Lua SDK v0
  phosphornet Python SDK v0
```

Go 1.26+ is the module baseline for modern standard-library support and current tooling.

Python 3.11+ is a good baseline for optional Python doors that need async support, performance, and availability.

## 11. Packaging Strategy

Primary deliverables:

```text
phosphor
  Main CLI / TUI client.

phosphord
  Node daemon.

phosphor-actiond
  Optional allowlisted host-action daemon.

switchboard
  Relay/rendezvous service.
```

Distribution goals:

- Single Go binaries where possible.
- No required external database server.
- No required Docker for basic local use.
- Python required only on machines hosting Python doors.
- Users connecting to remote nodes only need `phosphor`.

## 12. Current Stack

```text
Language:
  Go 1.26+ for binaries
  Lua for default doors through gopher-lua
  Python 3.11+ for optional doors

Client:
  Bubble Tea
  Lip Gloss
  Glamour

Transport:
  WebSocket via github.com/coder/websocket
  JSON messages

Node:
  Go HTTP/WebSocket server
  SQLite via modernc.org/sqlite
  TOML config
  embedded gopher-lua runtime
  generic stdio runtime

Experimental switchboard:
  Go health endpoint and CLI foundation

Identity:
  Ed25519 using Go stdlib
  known-node key pinning
  local filesystem permission protection for passport files

Persistence:
  SQLite
  versioned SQL history plus runtime schema bootstrap

Packaging:
  Single Go binaries
  Lua SDK embedded in phosphord
  Python SDK vendored or installed locally for Python doors
```

This stack is intentionally boring in the right places. The character belongs
in useful homelab doors and the feeling of entering a machine with a clear
purpose—not in infrastructure complexity.
