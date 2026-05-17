# Historical MVP Implementation Roadmap

> This file is retained for design history.
>
> The current loose-end backlog is `docs/PhosphorNet_todo.md`.
> Do not treat this roadmap as the active implementation queue without first checking the todo document and current code.

## Status Note

Several items in this document have already been implemented, while others remain useful as design notes or future follow-up ideas. Treat it as a planning artifact and architectural reference, not as the single live backlog for the repository.

For a tighter current backlog of loose ends, use `docs/PhosphorNet_todo.md`.

## Purpose

This document turns the MVP planning decisions that shaped this repository into a file-level roadmap.

It is meant to guide the next implementation phases for:

- strict cross-interpreter door runtime design
- shared user/room/global state
- multiplayer-capable door runtime primitives
- richer Bubble Tea client rendering
- end-to-end proof doors for `lobby`, `chat`, and `strategy_demo`

This roadmap is implementation-oriented. It assumes the current documentation in:

- `docs/PhosphorNet_architecture.md`
- `docs/PhosphorNet_technology_stack.md`

If this roadmap drifts from those docs, update both intentionally rather than letting them silently diverge.

## Locked Product And Platform Decisions

The current implementation roadmap assumes the following decisions are settled:

- both single-user and multiplayer door experiences must be supported
- the door API must be strict and cross-interpreter
- Python is the first backend, but the runtime contract must be generic enough for future interpreters such as Lua
- the canonical door contract is JSON over stdin/stdout request-response for MVP
- first-class state scopes for the next phase are:
  - `user`
  - `room`
  - `global`
- each door gets one implicit room for MVP simplicity
- doors are isolated from each other
- `global` state writes are sysop/admin only
- the door lifecycle baseline is:
  - `init`
  - `view`
  - `update`
  - `on_join`
  - `on_leave`
  - `tick`
- door responses should support structured side effects rather than hidden runtime mutation
- doors need broadcast capability
- the multiplayer proof target is a turn-based grid tactics demo

## Canonical Runtime Contract Work

### `internal/protocol/types.go`

Expand the shared typed protocol models to support the next runtime phase.

Planned additions:

- canonical runtime request envelope for interpreter lifecycles
- canonical runtime response envelope
- typed lifecycle names:
  - `init`
  - `view`
  - `update`
  - `on_join`
  - `on_leave`
  - `tick`
- typed event kinds:
  - `action`
  - `select`
  - `submit`
  - `key`
  - `focus`
- structured side-effect payloads for:
  - `view`
  - `state_ops`
  - `broadcasts`
  - `notifies`
  - `transitions`
- explicit runtime error payloads instead of loosely shaped failures

This file should stay strongly typed. Avoid falling back to loosely structured `map[string]any` payloads for the main protocol.

### `internal/protocol/runtime.go`

Create this file if the interpreter-facing runtime contract needs to be separated from the client WebSocket protocol.

Reason:

- keep client/node wire messages small and clear
- keep interpreter contract evolution isolated
- avoid overloading `types.go` with unrelated concerns

## Runtime Core Work

### `internal/runtime/invoker.go`

Refactor the runtime core into a contract-first interpreter abstraction.

Planned changes:

- evolve the current context payload into a canonical runtime context
- add explicit scoped state snapshots for:
  - `user`
  - `room`
  - `global`
- add presence snapshot data
- add user/session metadata
- add role/permission metadata
- replace the current narrow `DoorResponse` with the full structured response shape
- keep the invoker registry generic so future runtimes can register cleanly by runtime name

This file should define the minimum interface every interpreter backend must satisfy.

### `internal/runtime/python_invoker.go`

Keep Python as the first concrete backend, but align it exactly to the canonical runtime contract.

Planned changes:

- send the full lifecycle request envelope to Python
- parse the full structured response envelope from Python
- stop treating Python as a special-case protocol
- keep process invocation generic enough that a Lua backend can match the same contract later

### `internal/runtime/session.go`

Create a runtime-facing session model shared by node/runtime logic.

Planned responsibilities:

- active door tracking
- implicit room membership tracking
- client focus metadata
- authenticated user/session metadata

### `internal/runtime/effects.go`

Centralize structured side-effect application logic.

Planned responsibilities:

- interpret `state_ops`
- interpret `broadcasts`
- interpret `notifies`
- interpret `transitions`
- keep side-effect handling out of the main node message loop

### `internal/runtime/presence.go`

Create shared runtime helpers for presence snapshots.

Planned responsibilities:

- connected-user views
- current-door room membership views
- focused-session views
- typed presence shapes shared with the interpreter contract

### `internal/runtime/manifest.go`

Extend only if needed for runtime metadata.

Potential future additions:

- declared runtime capabilities
- interpreter-specific configuration
- optional capability declarations for broadcast or key capture

Do not turn manifests into large framework configuration blobs.

### `internal/runtime/invoke_test.go`

Expand tests to cover:

- `view` lifecycle
- `update` lifecycle
- structured response parsing
- runtime error handling
- generic invoker registry behavior

## Storage Work

### `internal/storage/sqlite.go`

Evolve storage from the current per-user door-state path into explicit scoped state handling.

Planned changes:

- expose first-class storage helpers for:
  - user-scoped door state
  - room-scoped door state
  - global-scoped door state
- support atomic multi-scope updates
- enforce permission checks for global writes
- keep state loading/saving explicit and inspectable

This file should remain the source of truth for persisted state behavior in MVP.

### `internal/storage/state.go`

Create this file for scope models and key formats.

Planned responsibilities:

- typed scope names
- scope-key normalization
- helpers for deriving storage keys from door/session context

### `internal/storage/presence.go`

Only add this if some room or presence metadata needs to survive reconnects.

For MVP, live in-memory presence may remain the default.

### `migrations/001_init.sql`

Keep the current initial migration documented as the starting point.

### `migrations/002_scoped_state.sql`

Add a migration for normalized scoped state if the schema needs to move beyond the current simple door-state table.

Likely responsibilities:

- explicit scope type storage
- normalized scope keys
- timestamps and update metadata

### `migrations/003_rooms.sql`

Add only if implicit per-door rooms still need durable metadata or membership-adjacent records.

## Node Daemon Work

### `internal/node/cli.go`

Reduce this file back toward command bootstrap responsibility.

Current runtime logic should eventually be split out so this file becomes a thin Cobra entrypoint layer.

### `internal/node/server.go`

Move WebSocket server orchestration here.

Planned responsibilities:

- listener setup
- connection lifecycle
- session bootstrap
- integration with the runtime dispatcher

### `internal/node/session.go`

Track node-side live sessions.

Planned responsibilities:

- connected users
- session identifiers
- active door per session
- implicit room membership per door
- focused door/session state

### `internal/node/messages.go`

Centralize client message decoding and routing.

Planned responsibilities:

- typed decode path for incoming client messages
- routing for `open_door`
- routing for `event`
- future `close_door`

### `internal/node/doors.go`

Centralize door lifecycle dispatch.

Planned responsibilities:

- `init`
- `view`
- `update`
- `on_join`
- `on_leave`
- `tick`

### `internal/node/broadcast.go`

Implement node-side broadcast fanout.

Planned responsibilities:

- target resolution for:
  - `room`
  - `door`
  - `user`
- session fanout
- broadcast-safe render refresh paths

### `internal/node/state.go`

Centralize scoped state loading and persistence.

Planned responsibilities:

- populate runtime context with `user`, `room`, and `global`
- apply returned `state_ops`
- keep state access policy out of general message handling

### `internal/node/permissions.go`

Implement sysop/admin gating for privileged state changes.

Planned responsibilities:

- global write checks
- future door capability checks if needed

### `internal/node/tick.go`

Reserve periodic door `tick` orchestration for a separate unit rather than burying it in the connection loop.

## Client Work

### `internal/client/cli.go`

Keep this file focused on command wiring, connect-time configuration, and TUI startup.

### `internal/client/tui.go`

Continue evolving the Bubble Tea shell into the primary trusted local client experience.

Planned work:

- richer local chrome
- clearer door navigation
- stronger focus model
- integration of presence and notifications
- separation between trusted local UI and remote door content

### `internal/client/render.go`

Replace the current flatten-to-text approach with component-aware rendering logic.

Planned work:

- render more semantic component types cleanly
- preserve structured layouts better
- support remote door UIs that are more than simple text panels and first-menu actions

### `internal/client/styles.go`

Create a dedicated Lip Gloss theme/layout file.

Planned responsibilities:

- shared borders
- spacing
- panel styling
- status chrome
- colors and typography choices for the client shell

### `internal/client/components.go`

Create a component-aware render layer.

Planned responsibilities:

- `screen`
- `header`
- `text`
- `panel`
- `menu`
- `list`
- `input`
- `textarea`
- `button`
- `status`
- `log`
- `grid`

### `internal/client/actions.go`

Create a clean mapping layer from local interactions to canonical remote events.

Planned responsibilities:

- action dispatch
- selection dispatch
- submit dispatch
- focus dispatch
- key dispatch when explicitly allowed

### `internal/client/viewport.go`

Add this if the remote render tree needs dedicated scrolling/viewport behavior beyond the current shell model.

### `internal/client/presence.go`

Add optional rendering helpers for presence lists, room membership, and related status chrome.

## Python SDK Work

### `sdk/python/phosphornet/runtime.py`

Make the Python runtime a strict adapter for the canonical contract.

Planned work:

- accept canonical lifecycle request envelopes
- emit canonical structured responses
- keep stdin/stdout protocol behavior explicit and inspectable

### `sdk/python/phosphornet/ctx.py`

Expand the door context API to reflect the canonical runtime contract.

Planned additions:

- scoped state access views for `user`, `room`, `global`
- presence snapshots
- user/session metadata
- role/permission metadata

### `sdk/python/phosphornet/ui.py`

Grow helper coverage only as needed to support the canonical UI component set.

### `sdk/python/phosphornet/effects.py`

Add optional helpers for building structured runtime effects.

Planned helpers:

- broadcasts
- notifications
- transitions
- state operations

### `sdk/python/phosphornet/state.py`

Optional typed helpers for scoped state access while still serializing to the same canonical JSON contract.

### `sdk/python/phosphornet/presence.py`

Optional typed helpers for presence access if `ctx.py` becomes too crowded.

## Proof Doors

### `doors/lobby/app.lua`

Evolve `lobby` into the first full proof of the embedded Lua runtime contract.

It should demonstrate:

- identity and station context
- room presence
- `on_join` and `on_leave`
- room-scoped interaction
- optional read-only global status

### `doors/lobby/manifest.toml`

Only extend if the runtime needs additional declared capabilities or metadata.

### `doors/chat/app.py`

Turn `chat` into the shared-room proof door.

It should demonstrate:

- room-scoped persisted message log
- room presence
- broadcasts to all sessions in that door
- reconnect-safe shared room state

### `doors/chat/manifest.toml`

Only extend if capability declarations are introduced for room state, broadcasting, or key capture.

### `doors/strategy_demo/app.py`

Turn `strategy_demo` into the multiplayer proof door.

It should demonstrate:

- shared room-scoped board state
- 10x10 grid rendering
- player presence
- unit selection
- movement
- end turn
- shared event log
- deterministic update behavior under multiple users

### `doors/strategy_demo/manifest.toml`

Only extend if the runtime introduces declared door capabilities.

## Documentation Work

### `AGENTS.md`

Keep agent guidance aligned with the runtime contract and roadmap as implementation hardens.

### `changelog.md`

Update every time the repository changes in meaningful ways, including planning docs that steer implementation.

### `docs/protocol.md`

Add a focused protocol document for the client/node WebSocket contract.

### `docs/door-runtime-contract.md`

Add a dedicated specification for the interpreter-facing canonical JSON request/response contract.

### `docs/door-authoring-python.md`

Add a practical guide for authoring Python doors once the canonical runtime stabilizes.

### `docs/multiplayer-model.md`

Document user/room/global scopes, implicit rooms, broadcasts, presence, and lifecycle expectations.

## Recommended Build Order

1. Expand the canonical runtime contract in `internal/protocol/`.
2. Normalize scoped storage in `internal/storage/` and migrations.
3. Split node runtime logic out of `internal/node/cli.go`.
4. Align the Python backend with the final runtime contract.
5. Make `doors/lobby/` the first full end-to-end proof door.
6. Finish `doors/chat/` as the first shared-room door proof.
7. Finish `doors/strategy_demo/` as the multiplayer proof.
8. Deepen the Bubble Tea client rendering layer.

## Immediate Next Slice

The highest-leverage next implementation slice is:

1. define the canonical runtime request/response structs
2. define explicit `user` / `room` / `global` state handling
3. split node lifecycle dispatch out of `internal/node/cli.go`
4. align the Python runtime to the contract before adding more door complexity

That sequence should keep the repo inspectable while opening the path to both single-user and multiplayer doors.
