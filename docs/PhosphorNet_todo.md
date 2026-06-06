# PhosphorNet Todo

## Purpose

This document gathers the loose ends that are currently scattered across the repository.

It is meant to be the short, inspectable backlog for:

- open MVP follow-ups that are still true today
- documentation and policy drift that should be cleaned up
- explicitly deferred later work that should not be mistaken for shipped behavior

The architecture and stack docs remain the product source of truth. This file is the operational checklist for unfinished edges.

## Current MVP Follow-Ups

| Area | Loose end | Current state | Sources |
|---|---|---|---|
| Transition effects | Transition handling is only partially implemented. | `open_door` transitions are applied by `phosphord`, but the runtime contract still declares `close_door` and `room` transition kinds without matching end-to-end behavior. | `docs/PhosphorNet_runtime_contract.md`, `internal/node/broadcast.go`, `internal/protocol/runtime.go`, `README.md` |
| Admin storage operations | Cross-door state clearing is still intentionally missing. | The Station Admin door can inspect scoped state summaries, but arbitrary cross-door clearing still needs a node-owned admin effect plus confirmation flow. | `docs/PhosphorNet_configuration.md`, `doors/admin/app.lua` |
| Relay / switchboard | `switchboard` is still a scaffold rather than the full reachability layer described by the architecture docs. | The repo has a starter relay/rendezvous executable and setup docs, but not a finished rendezvous and frame-forwarding network. | `README.md`, `docs/PhosphorNet_setup.md`, `docs/PhosphorNet_architecture.md` |
| Presence durability | Presence remains live and in-memory only. | Connected user lists are useful for room and station presence, but they are not durable state and do not survive reconnects or restarts as stored presence records. | `README.md`, `docs/PhosphorNet_authoring_doors.md`, `docs/PhosphorNet_runtime_contract.md`, `docs/PhosphorNet_MVP_implementation_roadmap.md` |
| Room model | Rooms are still implicit per door. | The MVP currently treats each door as one implicit room. A richer room model, durable room metadata, and explicit room lifecycle are still open design and implementation work. | `README.md`, `docs/PhosphorNet_MVP_implementation_roadmap.md` |
| Runtime isolation | The Lua sandbox is still MVP-grade rather than a full host isolation boundary. | Unsafe libraries are blocked and execution is bounded, but the docs still describe the sandbox as early and configurable rather than a complete isolation story. | `README.md`, `docs/PhosphorNet_technology_stack.md` |
| Stdio containment profile | The generic stdio backend now defaults to Podman-wrapped image processes and supports explicit `mode = "host"` trusted opt-out processes, but environment allowlisting and richer named profiles remain open. | Current stdio invocation uses explicit argv or generated Podman argv, stdin/stdout JSON, stderr diagnostics, timeout, output caps, and a harsh default Podman profile. | `docs/PhosphorNet_configuration.md`, `internal/runtime/stdio_invoker.go`, `internal/runtime/container_invoker.go` |
| Secret storage hardening | Passport storage is still file-permissions-first. | The stack leaves room for encrypted passport storage later, but passphrase-based protection is not yet part of the default implementation. | `docs/PhosphorNet_technology_stack.md` |

## Architecture Gap List

These are larger design gaps to settle before PhosphorNet's public-alpha surface becomes hard to change. They are tracked here as todo items, not as shipped behavior.

### Protocol Evolution And Compatibility

Addressed on 2026-05-15 for the current client/render contract. The client now sends a pre-auth `hello` with client version, runtime protocol version, JSON UI schema version, supported components, supported style features, supported event kinds, and render limits. `phosphord` rejects incompatible clients with `client_incompatible` before sending door UI.

Future protocol evolution may still need effect-level negotiation when new effect families become client-visible.

### Message Correlation, Idempotency, And Event Ordering

Addressed on 2026-06-06 for the live client event path. Node render messages
now include `active_door_id` and a monotonically increasing `render_revision` for
the session. Client event messages include `session_id`, `active_door_id`,
`render_revision`, and `event_id`.

`phosphord` rejects mismatched session or active-door metadata, duplicate
`event_id` values within a short live-session window, and stale render revisions
for submit-like events (`action`, `select`, and `submit`). Raw key/focus-style
events still carry render metadata but are not rejected only because a newer
render arrived.

Future reconnect and durable session recovery work may still need broader
message correlation for non-event client requests and node-to-client replies.

### Reconnect And Session Recovery

Addressed on 2026-06-06 for alpha behavior.

Current reconnect semantics are intentionally boring:

- A reconnect creates a new authenticated WebSocket session and a new session ID.
- A short-drop reconnect may reopen the previous active door if it still exists and the new session can still access it.
- If the previous door is not safe to reopen, the session starts in `lobby`.
- Presence remains live-only and in-memory.
- Scroll position, focus, and input drafts are not restored.
- `on_leave` runs after a short disconnect grace timeout instead of immediately.
- Reconnecting inside that grace window cancels the pending `on_leave` and avoids another `on_join` for the recovered door.

Future durable session recovery could still revisit richer resume behavior, but the public-alpha contract is now defined and tested.

### Schema Ownership For The JSON UI Contract

Addressed on 2026-05-14. The JSON UI contract is now owned by `internal/protocol`, with a formal v1 schema and golden-test corpus.

Canonical coverage now lives in:

- `internal/protocol/schema/json-ui-v1.schema.json`
- `internal/protocol/testdata/ui_contract/v1/valid`
- `internal/protocol/testdata/ui_contract/v1/invalid`
- `internal/client/testdata/ui_contract/v1/render`

Keep future component, field, SDK-helper, limit, invalid-case, and client-rendering changes in this corpus before changing door helpers or renderer behavior.

### Error Taxonomy

Addressed on 2026-05-15 for runtime and door-launch failures. Node-to-client `error` messages now carry stable `code` values for `runtime_not_available`, `runtime_image_missing`, `runtime_timeout`, `runtime_bad_output`, `runtime_denied_by_policy`, `runtime_resource_limit`, `manifest_invalid`, and `door_crashed`, plus supporting protocol/auth/storage/client-compatibility codes.

Future work can still refine trust, maintenance-mode, and durable audit presentation.

### Observability And Audit Trail

Addressed on 2026-05-30 for the server-side alpha audit trail. `phosphord` now separates durable audit history from the in-memory runtime event log with an append-only-ish SQLite `audit_events` table, optional JSONL mirroring through `phosphord serve --audit-log-file`, and a shared `--audit-log-max-bytes` limit for SQLite retention plus JSONL rotation.

Current server-side audited events include admin role changes, door enable/disable, door role policy changes, door setting changes, manifest reloads, maintenance changes, event log clearing, moderation operations, failed or denied auth, door access denials, denied privileged effects, and node key changes detected at startup.

Remaining future work: richer Station Admin audit presentation, SQLite retention/export policy, and client-local trust-decision audit records.

### Database Lifecycle

Addressed on 2026-05-15. `docs/PhosphorNet_database_lifecycle.md` documents the station backup bundle, restore procedure, migration expectations, schema version reporting, DB deletion behavior, node-key deletion behavior, and repair/compaction notes. `phosphord serve` logs the absolute database path and SQLite schema version on startup.

### Door Identity, Signing, And Provenance

User and node identity are strong, but door identity is still mostly "manifest in a directory."

For a public ecosystem, answer:

- Who authored this door?
- What version is running?
- Has it been modified?
- Is it trusted by this sysop?
- Can users see that trust level?
- Can bundled doors be checksummed?
- Can installed doors be signed?
- Can sysops pin trusted door keys?

Even before a marketplace, signed bundled doors and local manifest checksums would make the system feel more serious.

### Door Lifecycle Concurrency Semantics

Lifecycle names such as `init`, `view`, `update`, `on_join`, `on_leave`, and `tick` are clear, but scheduling behavior is not yet fully defined.

Define:

- Are updates for the same door/session serialized?
- Are updates for the same room serialized?
- Can `tick` overlap `update`?
- Can broadcast trigger render while `update` is applying state?
- Is `view` pure?
- Can `view` emit effects?
- Are `state_ops` the only committed mutation path?

Atomic state operations are good, but atomic storage is not the same as deterministic lifecycle scheduling.

### Backpressure And Noisy-Door Policy

Limits and DoS protections exist, but doors that constantly broadcast, notify, render huge screens, trigger re-render fanout, or spam state updates need a clearer policy. Current broadcast behavior can become a fanout amplifier.

Define:

- per-door render budget
- broadcast budget
- notify budget
- state update budget
- degraded or paused door state
- operator-visible noisy-door diagnostics

### Privacy And Data Retention

Stations store user keys/fingerprints, handles, posts, chat logs, settings, profiles, and logs. The architecture needs a basic station privacy model.

Document:

- What is public to other users?
- What is visible to admins?
- What is persisted?
- What can be deleted?
- What can be exported?
- What appears in logs?
- What survives a station backup?

This is not legal-document territory yet; it is architecture hygiene.

### Client Local State Management

The client has passport storage and a known-node registry, but local identity lifecycle needs a stronger product story.

Document:

- multiple profiles
- passport import/export
- corrupted known-node entries
- untrust/retrust flows
- key rotation
- new laptop migration
- lost passport consequences
- quick identity versus persistent identity

For a system based on portable identity, recovery and migration are part of the architecture.

### Node Key Rotation And Reinstall Story

The docs correctly warn when a node key changes, but legitimate recovery remains vague.

Questions to answer:

- If a sysop rebuilds a station, how do users verify continuity?
- Can the old node key sign a rotation statement?
- Can invite links include old and new identity?
- Can a station publish a continuity record?
- Can clients distinguish expected rotation from impersonation?

Without this, every reinstall looks like an attack forever.

### Basic Public-Station Moderation Primitives

Addressed as a separate primitive spec on 2026-05-14 in `docs/PhosphorNet_public_station_moderation.md`.

The survival kit is now defined separately from ambitious social systems:

- ban key
- mute key
- delete or hide user content
- freeze door
- maintenance notice
- rate-limit user
- inspect recent abuse-relevant activity
- appeal/unban path later, if needed

Implementation now includes node-owned ban, unban, mute, unmute, moderation-note, and per-user event/open-door rate-limit admin ops, plus Station Admin controls. Durable audit retention and richer user-facing appeal workflows remain future work.

### Test Matrix And Compatibility Promise

The current release-confidence checklist is indexed in `docs/PhosphorNet_test_index.md`.

It maps these promises to the tests that already protect them:

- auth handshake
- node key pinning
- manifest rejection
- UI sanitization
- door effect authorization
- scoped state atomicity
- stdio malformed output handling
- Lua sandbox escape attempts
- broadcast fanout
- admin op authorization
- first-connect trust behavior
- client incompatible UI handling

Keep that index current when adding, moving, or removing tests. Prefer filling the documented gaps over adding duplicate broad integration scaffolding.

### Bubbles-Backed Client Component Model

The protocol already defines semantic UI components like `input`, `textarea`, `list`, `dynamic_list`, `log`, `markdown`, and `screen`. The client should use Bubble Tea Bubbles as the reference implementation for interactive protocol components, without exposing Bubbles-specific concepts in the protocol.

Reference mapping:

- `input` -> `bubbles/textinput`
- `textarea` -> `bubbles/textarea`
- `list` / `dynamic_list` -> `bubbles/list` or a custom list model
- `log` -> `bubbles/viewport`
- `markdown` -> Glamour rendered inside a viewport
- screen viewport -> `bubbles/viewport`
- loading/pending -> `bubbles/spinner`

The important boundary:

- doors emit PhosphorNet UI components
- client implements those components with Bubbles
- protocol never says `bubbles.textarea` or leaks Go implementation details

The missing architecture piece is a client-side reconciliation model:

- component ID to local widget state
- focused component
- cursor position
- input/textarea contents
- scroll offset
- list selection
- viewport position
- bottom-pinned transcript behavior
- submit/select/action event emission
- behavior when a server render replaces or removes a focused component

Without this, richer TUI interaction will either become janky or leak client implementation details into the protocol.

## Documentation And Policy Cleanup

| Area | Loose end | Current state | Sources |
|---|---|---|---|
| Transition docs | The authoring guide still says full transition handling is future work, but `open_door` transitions already work. | The docs should be tightened so they distinguish implemented `open_door` handoff from unimplemented transition kinds. | `docs/PhosphorNet_authoring_doors.md`, `docs/PhosphorNet_runtime_contract.md`, `internal/node/broadcast.go` |
| Role/access vocabulary | Station admission, session roles, door access, and door capabilities need to stay aligned across docs. | The current docs now distinguish `public`/`invite_only` station admission, `member`/`admin`/`sysop` session roles, door access, and door capabilities. Future edits should preserve that split. | `docs/PhosphorNet_architecture.md`, `docs/PhosphorNet_configuration.md`, `docs/PhosphorNet_runtime_contract.md`, `docs/PhosphorNet_authoring_doors.md` |
| Admin op authority | Every node-owned admin op should have an explicit role and capability requirement. | The runtime contract has a current authority table; future admin ops should update it before shipping. `clear_event_log` may deserve a dedicated destructive log capability later. | `docs/PhosphorNet_runtime_contract.md`, `internal/node/admin_ops.go`, `internal/node/capabilities.go` |
| Backlog ownership | The existing MVP roadmap is now historical. | The roadmap now has a top banner pointing to this todo file as the active loose-end backlog, but stale historical sections should still be refreshed or trimmed before being used for planning. | `docs/PhosphorNet_MVP_implementation_roadmap.md`, `README.md`, `changelog.md` |

## Explicitly Deferred Later Work

These are real future directions in the docs, but they should not be treated as current MVP promises.

| Area | Deferred work | Sources |
|---|---|---|
| Network scope | DHT discovery, full federation, offline mail, public node directory, and end-to-end encrypted relayed streams. | `docs/PhosphorNet_architecture.md`, `AGENTS.md` |
| Client platforms | Browser client and mobile client. | `docs/PhosphorNet_architecture.md`, `AGENTS.md` |
| Distribution model | Public door marketplace and local execution of downloaded doors. | `docs/PhosphorNet_architecture.md`, `AGENTS.md` |
| Permission model | Advanced host/container capabilities such as environment allowlists, named resource profiles, mounts, and broader process execution authority beyond the current explicit stdio and Podman profiles. | `docs/PhosphorNet_architecture.md`, `docs/PhosphorNet_MVP_implementation_roadmap.md` |
| Terminal media | Any future ANSI art support would need a deliberately tiny safe subset rather than passthrough rendering. | `docs/PhosphorNet_architecture.md` |
| Social systems | Advanced moderation, reputation, and more ambitious community governance features. | `docs/PhosphorNet_architecture.md` |

## Suggested Next Cleanup Pass

If the goal is to reduce ambiguity before adding more features, the highest-value doc and policy cleanup is:

1. Tighten any remaining transition prose so `open_door` is clearly current behavior and other transition kinds remain reserved.
2. Refresh or archive stale sections of the historical MVP roadmap so completed slices no longer look like active work.
3. Review whether destructive log clearing should split from `admin:read_logs` into a dedicated capability.
4. Define the next stdio containment step: environment allowlist first, then reusable named isolation profiles.
