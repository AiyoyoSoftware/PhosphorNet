# PhosphorNet Architecture

## 1. Concept

PhosphorNet is a self-hosted, terminal-native platform for safe homelab control
surfaces and other interactive station-hosted tools. Its station and door model
is inspired by BBSes, old online services, and personal-computer networking.

Every user can run a node. Every node can host doors. A user connects to any reachable node using a dedicated TUI client. The node sends a declarative JSON UI contract over WebSocket. The client renders that UI locally and sends back scoped events such as menu actions, form submissions, and approved key events.

The system is not a web browser, not SSH, and not a general remote-code execution environment. The client does not run node-provided code. Nodes run server-side doors and expose only structured UI descriptions to clients.

Core principle:

> Client renders. Node thinks. Doors define behavior. Ed25519 identifies. WebSocket carries the session.

The interpreter-facing door contract is defined in `docs/PhosphorNet_runtime_contract.md`.

## 2. Naming

Project name:

```text
PhosphorNet
```

Primary concepts:

```text
station
  A user-facing node/place in the network.

node
  The technical runtime that hosts doors and accepts connections.

door
  A hosted app, game, tool, room, or service exposed by a node.

switchboard
  A relay/rendezvous service that helps nodes connect.

passport
  A user's portable Ed25519 identity key.
```

Executable/service names:

```text
phosphor
  Main CLI and client launcher.

phosphord
  Node daemon.

switchboard
  Relay/rendezvous service.
```

Technical docs may still use “app” where clarity matters, but user-facing UI should prefer “door” for hosted experiences.

## 3. Goals

### 3.1 Product Goals

- Give homelab operators focused remote interfaces without distributing shell access or building a browser application for every tool.
- Allow any person to host a station: private, local, public, or community-oriented.
- Support server-side doors such as dashboards, maintenance tools, logs, chat, boards, and games.
- Use a consistent TUI client rather than arbitrary terminal output.
- Make identity portable across nodes using public-key authentication.
- Keep the system small, understandable, inspectable, and self-hostable.

### 3.2 Technical Goals

- TUI client written in Go using Bubble Tea / Lip Gloss.
- Node daemon written in Go.
- Server-side doors written in embedded Lua by default, with Python and other languages supported through stdio commands.
- WebSocket transport for client-to-node sessions.
- Declarative JSON UI protocol.
- Ed25519 challenge-response authentication.
- SQLite-backed node state.
- Direct encrypted station connections, with an experimental open-source relay/switchboard path for additional reachability.

## 4. Product Boundaries

The current architecture deliberately excludes arbitrary client-side scripting,
local execution of downloaded doors, raw terminal escape passthrough, implicit
host authority, and general shell access. The protocol remains small, semantic,
typed, and inspectable rather than becoming a browser or generic remote DOM.

Basic public-station survival primitives are not the same as advanced moderation. Local ban, mute, content hide/delete, door freeze, maintenance notice, per-user rate limiting, and recent activity inspection are defined separately in `docs/PhosphorNet_public_station_moderation.md`.

## 5. System Overview

```text
┌────────────────────────────┐
│          phosphor           │
│  Bubble Tea TUI renderer    │
│  JSON UI interpreter        │
│  event mapper               │
│  passport wallet            │
│  known node registry        │
└──────────────┬─────────────┘
               │
               │ WebSocket + JSON protocol
               │ Ed25519 auth
               ▼
┌────────────────────────────┐
│         phosphord           │
│  WebSocket server           │
│  auth / trust / ACL         │
│  door registry              │
│  embedded Lua door runtime  │
│  stdio door runtime         │
│  SQLite state               │
│  render/event protocol      │
└───────┬────────────────┬───┘
        │                │
        │ Unix socket    │ experimental
        │ + JSON         │
        ▼                ▼
┌────────────────────┐  ┌────────────────────────────┐
│ phosphor-actiond   │  │        switchboard         │
│ fixed argv rules   │  │  relay / rendezvous        │
│ door allowlists    │  │  node registration         │
│ bounded execution │  │  frame forwarding          │
└────────────────────┘  │  health endpoint today     │
                        └────────────────────────────┘
```

## 6. Core Components

## 6.1 phosphor

`phosphor` is the trusted local terminal application and main CLI. It connects to nodes, authenticates with the user's Ed25519 passport, renders JSON UI contracts, and sends scoped events back to the node.

The client should be simple in app logic but polished in user experience.

Responsibilities:

- Generate and store Ed25519 passport keys.
- Maintain known node records and node key pinning.
- Connect to nodes over WebSocket.
- Authenticate using challenge-response signatures.
- Render approved JSON UI components.
- Own trusted UI chrome such as connection status, node identity, trust warnings, and permission prompts.
- Map terminal input to semantic events.
- Send raw key events only when the active door requests that capability.
- Enforce protocol limits and security boundaries.

The client must not:

- Execute node-provided code.
- Print raw terminal escape sequences from nodes.
- Let remote doors draw over trusted client chrome.
- Give doors access to local files, clipboard, shell commands, or private keys.

## 6.2 phosphord

`phosphord` is the local or hosted node daemon. It hosts doors, owns node state, authenticates users, and generates UI contracts.

Responsibilities:

- Serve WebSocket sessions.
- Authenticate clients by Ed25519 challenge-response.
- Maintain users, roles, and access policy.
- Register available doors.
- Run embedded Lua doors by default and stdio doors either as explicit `mode = "host"` trusted commands or Podman-isolated image processes when configured.
- Persist door and system state in SQLite.
- Receive client events and dispatch them to door sessions.
- Send render trees, notifications, and typed errors.
- Optionally connect to switchboard relays.

A node can be:

- Public: discoverable and open or account-gated.
- Private: not publicly listed, but reachable by address.
- Secret: invite-only, public-key allowlist.
- Local-only: LAN or localhost.
- Community node: always-on public place.
- Personal node: a user's own station.

## 6.3 phosphor-actiond

`phosphor-actiond` is an optional, separate host-action process. It is the only component in the normal door flow that launches operator-configured host commands.

Responsibilities:

- Listen on a local Unix socket using the typed `phosphornet.action.v1` JSON protocol.
- Load fixed command argv arrays from its own TOML file.
- Require each rule to name the exact doors allowed to invoke it.
- Pass door-supplied JSON only on command stdin; never splice it into argv or shell text.
- Enforce per-rule timeouts and bounded stdout/stderr capture.
- Return exit status, stdout, stderr, timeout, and truncation metadata to `phosphord`.

Authorization is intentionally two-layered. `phosphord` requires the door manifest capability `action:<rule-id>`, while `phosphor-actiond` independently requires that door ID in the rule's `allowed_doors`. The Unix socket ownership and mode are the local caller-authentication boundary.

`phosphord` must not execute an action rule itself or accept arbitrary command/argument text from a door.

## 6.4 switchboard

`switchboard` is the experimental foundation for optional native relay and
rendezvous support. The current command exposes a health endpoint; direct WSS
station connections over LANs, private VPNs, or operator-managed Internet
routing do not require it.

Any relay implementation must remain transport-only. It must not own passport
identity, user accounts, door logic, or station source-of-truth data. Active
relay development is tracked only in `docs/PhosphorNet_roadmap.md`.

## 7. Identity Model

## 7.1 User Identity

User identity is based on Ed25519 keypairs. In user-facing language, this identity is the user's passport.

- Private key stays on the user's machine.
- Public key identifies the user across nodes.
- Nodes authenticate users by challenge-response.
- Users can have local handles on different nodes, but the key is the portable identity.

Example:

```text
User key: ed25519:abc123...
Display fingerprint: RAVEN-7KQF-LM92
Known as:
  netanel@midnight.exchange
  phosphor@ghostvm-dev
  netanel@localbox
```

The handle is contextual. The key is global.

## 7.2 Node Identity

Nodes also have Ed25519 keypairs.

The client stores known node keys similarly to SSH `known_hosts`.

On first connection:

```text
First time connecting to midnight.exchange.
Node key: ed25519:node_xyz...
Trust this node? [Yes] [No]
```

If the node key changes:

```text
WARNING: node identity changed.
This could be a reinstall or an impersonation.
```

Secret node invite links should include the expected node public key.

## 7.3 Authentication Flow

```text
1. Client connects to node.
2. Client sends hello with public key.
3. Node sends a typed random nonce challenge signed by the node key.
4. Client verifies the node challenge signature and pinned node identity.
5. Client signs a typed login payload using its private key.
6. Node verifies the client signature.
7. Node maps the public key to role/access policy.
8. Node sends auth_ok or auth_denied.
```

Typed signing payload example:

```json
{
  "purpose": "phosphornet.login.v1",
  "node_id": "ed25519:node_xyz",
  "client_public_key": "ed25519:user_abc",
  "nonce": "random-base64",
  "timestamp": "2026-05-03T12:00:00Z"
}
```

The client should never sign arbitrary opaque blobs for a node. Signatures should be domain-separated by purpose.

Typed node challenge example:

```json
{
  "purpose": "phosphornet.node_challenge.v1",
  "node_id": "ed25519:node_xyz",
  "node_name": "midnight.exchange",
  "client_public_key": "ed25519:user_abc",
  "nonce": "random-base64",
  "timestamp": "2026-05-10T12:00:00Z"
}
```

## 8. Access Model

Station admission controls who can enter the station at all:

```text
public
  Anyone with a valid key can connect.

invite_only
  Public key must be present in node allowlist.
```

Door access is separate from station admission. A user must first be admitted to the station, then the selected door's manifest access policy is checked:

```text
public
  Any admitted station user can open the door.

invite_only
  Reserved for door-level allowlist policy.

admin
  Only admin/sysop sessions can open the door.
```

Capabilities are a third layer. Door access controls who can open the door; capabilities control which privileged effects the door may ask `phosphord` to apply.

Current session roles:

```text
member
admin
sysop
```

`member` is the ordinary admitted-user role. `admin` can operate station policy through admin-capable doors. `sysop` is the highest local operator role. Guest-like display state may exist for users without a configured profile name, but `guest` is not a current station-policy role.

Public-station moderation should not be implemented primarily as roles. Bans, mutes, and rate-limit overrides are local station policy keyed by passport public key; roles describe authority after admission.

## 9. Door / App Model

Doors are server-side apps hosted by a node. Doors use embedded Lua by default,
while Python and other command-style doors are supported through the generic
stdio runtime.

The client never receives Lua or Python code. The client only receives JSON UI contracts.

## 9.1 Door Lifecycle

Door lifecycle interface:

```python
async def init(ctx):
    pass

async def view(ctx):
    return ui.screen(...)

async def update(ctx, event):
    return await view(ctx)
```

Additional supported lifecycle hooks:

```python
async def tick(ctx):
    pass

async def on_join(ctx, user):
    pass

async def on_leave(ctx, user):
    pass
```

## 9.2 Door Directory Layout

```text
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
```

Manifest example:

```toml
id = "strategy_demo"
name = "Iron Orchard"
entry = "app.lua"

capabilities = [
  "capture_keys",
  "state:room:read",
  "state:room:write",
  "broadcast:room"
]

[sandbox]
profile = "strict"
max_memory_kb = 65536
max_execution_ms = 5000
```

## 9.3 Door Runtime

The default runtime is embedded Lua through `gopher-lua`:

```text
phosphord ↔ embedded Lua VM
            typed runtime context in memory
            declarative UI table out
```

Lua is the default door runtime because it keeps `phosphord` close to standalone, avoids requiring a language toolchain for common doors, and gives the node a configurable sandbox boundary. The strict default sandbox opens only small safe standard-library subsets and the PhosphorNet door SDK table; broader library access must be explicitly configured at the node or door level.

Python and other command-style doors remain supported through the generic stdio backend and the same interpreter-agnostic envelope. They declare `runtime = "stdio"` with an explicit command or Podman image, not a Python-specific runtime name:

```text
phosphord ↔ stdio host command or Podman image
            JSON over stdin/stdout
```

Both Lua and stdio implement the same lifecycle names and response effects. Host direct execution and Podman are stdio launch profiles, not separate runtime protocols. Doors should never bypass the runtime contract to reach the client directly.

## 9.4 Door State

Doors access state through a controlled SDK, not arbitrary filesystem/database access.

Example SDK shape:

```python
game = ctx.store.get("room", "game", {})
ctx.store.set("room", "game", game)
ctx.effects.broadcast({"kind": "action", "target": "game", "action": "changed"}, scope="room")
```

Door state is stored as scoped JSON values in SQLite.

## 10. JSON UI Protocol

The JSON UI protocol is the core platform contract.

Doors produce semantic UI trees. Nodes serialize those trees. Clients render them with Bubble Tea / Lip Gloss.

The protocol should be small, strict, and typed.

## 10.1 Message Types

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

## 10.2 Session Flow

```text
client connects
node sends/receives handshake
client authenticates
client opens lobby door
node sends render tree
client renders UI
user acts
client sends event
node dispatches to door
door updates state
node sends new render tree
```

## 10.3 Example Render Message

```json
{
  "type": "render",
  "session_id": "s1",
  "active_door_id": "lobby",
  "render_revision": 1,
  "view": {
    "component": "screen",
    "children": [
      {
        "component": "header",
        "text": "LOCALBOX"
      },
      {
        "component": "menu",
        "id": "main",
        "items": [
          { "label": "Chat", "action": "open:chat" },
          { "label": "Strategy Demo", "action": "open:strategy_demo" }
        ]
      },
      {
        "component": "status",
        "text": "Connected as NETANEL-7KQF-LM92"
      }
    ]
  }
}
```

## 10.4 Example Event Message

```json
{
  "type": "event",
  "session_id": "s1",
  "active_door_id": "lobby",
  "render_revision": 1,
  "event_id": "8f7a2c2f0e7d4c7d9c4f932c8d5f70af",
  "event": {
    "kind": "action",
    "action": "open:chat",
    "target": "main"
  }
}
```

Every client event is tied to the active session, active door, and render
revision that produced the interactive control. `phosphord` rejects events with
mismatched session or door metadata, rejects duplicate `event_id` values within a
short live-session window, and rejects stale render revisions for submit-like
events (`action`, `select`, and `submit`). Raw key/focus-style traffic still
carries the revision but is not rejected only because a newer render has arrived.

## 10.5 Protocol v1 Component Set

```text
screen
header
text
markdown
panel
menu
list
dynamic_list
input
textarea
button
checkbox
status
log
grid
```

These are enough for:

- Lobby.
- Chat.
- Boards.
- Forms.
- Simple dashboards.
- Turn-based games.
- Strategy demo.

Content-bearing components may carry narrowly scoped presentation hints under
`style`. Protocol v1 limits these to trusted-client-rendered solid and gradient
backgrounds on `screen`, `panel`, and `log`; leaf controls such as buttons do
not accept remote visual styling.

The JSON UI contract is schema-owned by `internal/protocol`. The formal v1 schema is `internal/protocol/schema/json-ui-v1.schema.json`, with golden protocol fixtures in `internal/protocol/testdata/ui_contract/v1` and client render fixtures in `internal/client/testdata/ui_contract/v1/render`. These fixtures are the compatibility corpus for Go structs, Lua/Python SDK helpers, invalid cases, size limits, and trusted-client rendering expectations.

Clients advertise their current compatibility in the pre-auth `hello`: client version, runtime protocol version, JSON UI schema version, supported components, supported style features, supported event kinds, and render limits. `phosphord` rejects incompatible clients before auth completes instead of sending UI trees outside the client's declared contract.

## 10.6 Protocol v1 Event Types

```text
action
select
submit
key
focus
```

Semantic events are preferred. Raw key events are only sent when the door requests key capture.

Example semantic event:

```json
{
  "kind": "submit",
  "target": "message_form",
  "values": {
    "message": "hello"
  }
}
```

Example key event:

```json
{
  "kind": "key",
  "target": "battlefield",
  "key": "left"
}
```

## 11. Security Model

The security model depends on keeping a hard boundary between trusted client behavior and untrusted node-provided UI.

## 11.1 Allowed Remote Node Behavior

Remote nodes may:

- Describe UI using approved components.
- Receive scoped user events.
- Send notifications within limits.
- Offer files or links with explicit confirmation.
- Request raw key capture for specific doors/components.

## 11.2 Forbidden Remote Node Behavior

Remote nodes may not:

- Execute code on the client.
- Send raw terminal escape sequences.
- Access local files.
- Access clipboard silently.
- Run shell commands.
- Read private keys.
- Hide or spoof trusted client chrome.
- Receive global key input by default.
- Cause arbitrary client-side network requests.

## 11.3 Trusted Client Chrome

The client owns trusted UI zones:

```text
┌──────────────────────────────┐
│ Client-owned trusted header   │
├──────────────────────────────┤
│ Remote door viewport          │
├──────────────────────────────┤
│ Client-owned status/security  │
└──────────────────────────────┘
```

Remote doors cannot draw over trusted zones.

Trusted chrome displays:

- Current node.
- Node key/trust state.
- Connected user identity.
- Security warnings.
- Permission prompts.
- Disconnect / emergency quit hints.

## 11.4 Terminal Escape Safety

The client must never print node-provided strings directly to the terminal.

All remote text is treated as plain text and rendered by the client.

Protocol v1 has no raw ANSI component. Nodes cannot send terminal control
sequences through the trusted renderer.

## 11.5 DoS Protections

The client must enforce hard limits:

- Max WebSocket message size.
- Max UI tree depth.
- Max children per component.
- Max text length per component.
- Max grid dimensions.
- Max renders per second.
- Max notifications per minute.
- Connection idle timeout.
- Schema validation failure threshold.

If a node exceeds limits, the client disconnects safely.

## 11.6 Key Safety

Private keys never leave the client machine.

Nodes receive:

- Public key.
- Typed signatures.
- Optional signed profile documents.

Nodes never receive:

- Private key.
- Seed phrase.
- Identity export.
- Key passphrase.

## 12. Storage Model

Current SQLite tables include:

```sql
CREATE TABLE users (
  public_key TEXT PRIMARY KEY,
  name TEXT,
  role TEXT,
  first_seen TEXT
);

CREATE TABLE door_state (
  door_id TEXT,
  key TEXT,
  value_json TEXT,
  PRIMARY KEY (door_id, key)
);

CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  public_key TEXT,
  door_id TEXT,
  created_at TEXT
);

CREATE TABLE messages (
  id TEXT PRIMARY KEY,
  room TEXT,
  author_key TEXT,
  body TEXT,
  created_at TEXT
);
```

This is intentionally simple. Doors can store JSON values through the SDK.

## 13. Bundled Reference Doors

## 13.1 Lobby

Purpose:

- Demonstrates node connection and door navigation.

Features:

- Node name.
- Connected identity.
- Available doors.
- Basic node status.

## 13.2 Chat

Purpose:

- Demonstrates real-time broadcast and multi-client sessions.

Features:

- Message log.
- Input box.
- Send message.
- Broadcast to connected clients.

## 13.3 Strategy Demo

Purpose:

- Demonstrates custom server-side door logic, grids, key events, and game state.

Features:

- 10x10 grid.
- Movable cursor.
- One or more units.
- Enter to select.
- Move unit.
- End turn action.
- Event log.

This is a compact reference game rather than a full game.

## 14. Example Commands

Initialize a node:

```bash
phosphord init --name localbox
```

Run a node:

```bash
phosphord serve --listen :7707
```

Create a passport:

```bash
phosphor passport create
```

Show public key:

```bash
phosphor passport show
```

Connect directly:

```bash
phosphor connect wss://localhost:7707
```

## 15. Design Mantra

> Computers should be places again.

The architecture stays focused on the core loop:

```text
connect → authenticate → open door → render JSON UI → send event → update state → render again
```

Everything else is secondary until that loop feels magical.
