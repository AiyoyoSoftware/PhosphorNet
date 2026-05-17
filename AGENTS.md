# AGENTS.md

## Purpose

This repository defines **PhosphorNet**, a terminal-native peer/node platform inspired by BBSes, old online services, and personal computer networking.

This file is for coding agents. Its job is to help you:

- find the right source of truth quickly
- stay faithful to current project requirements
- avoid loading unnecessary repo context for routine tasks

If code and docs disagree, prefer the docs unless the user explicitly asks to change project direction.

## Project Mantra

Keep this mental model front and center:

```text
phosphor renders.
phosphord thinks.
doors define behavior.
switchboard helps nodes connect.
Ed25519 identifies.
WebSocket carries the session.
SQLite remembers.
```

Core principle:

```text
Client renders. Node thinks. Doors define behavior. Ed25519 identifies. WebSocket carries the session.
```

## Fast Start For Agents

Before reading broadly, classify the task and load only the minimum relevant context.

Recommended order:

1. Identify the task type.
2. Read the smallest relevant source-of-truth docs.
3. Inspect the likely code entrypoints for that subsystem.
4. Expand context only if the first pass does not answer the question.

Do not load every doc by default.

Do not treat the MVP roadmap as the active backlog.

Check `docs/PhosphorNet_todo.md` before assuming a planned feature is already implemented.

## Source Of Truth By Task

Use these docs first:

- Architecture and product boundaries:
  `docs/PhosphorNet_architecture.md`
  `docs/PhosphorNet_technology_stack.md`
- Current loose ends, partial implementations, and doc drift:
  `docs/PhosphorNet_todo.md`
- Door runtime contract and door authoring:
  `docs/PhosphorNet_runtime_contract.md`
  `docs/PhosphorNet_authoring_doors.md`
- Setup, config, and operational behavior:
  `docs/PhosphorNet_setup.md`
  `docs/PhosphorNet_configuration.md`
- User-visible behavior and copy:
  `docs/PhosphorNet_user_guide.md`
  `README.md`
- Historical planning context only:
  `docs/PhosphorNet_MVP_implementation_roadmap.md`

Treat the roadmap as a planning artifact and architectural reference, not as the single live backlog.

## Repo Navigation Map

Use this map before opening more files:

- Client CLI, session bootstrap, TUI, rendering, input:
  `cmd/phosphor/`
  `internal/client/`
- Node auth, sessions, message routing, door dispatch:
  `cmd/phosphord/`
  `internal/node/`
- Shared protocol and wire/runtime message types:
  `internal/protocol/`
- Persistence and scoped state:
  `internal/storage/`
- Runtime backends, manifests, SDK bridge:
  `internal/runtime/`
- Identity, signatures, known-node trust:
  `internal/identity/`
  `internal/knownnodes/`
- Relay scaffold:
  `cmd/switchboard/`
  `internal/relay/`
- Python door SDK:
  `sdk/python/phosphornet/`
- Bundled doors:
  `doors/`

## Primary Concepts And Naming

Use these terms consistently:

- `station`: the user-facing node/place in the network
- `node`: the technical runtime that hosts doors and accepts connections
- `door`: a hosted app, game, tool, room, or service
- `switchboard`: relay/rendezvous service for reachability
- `passport`: the user's portable Ed25519 identity key

Executable names:

- `phosphor`: main CLI and trusted TUI client
- `phosphord`: node daemon
- `switchboard`: relay/rendezvous service

User-facing copy should prefer **door** over **app** when possible.

## Non-Negotiable Product Boundaries

When implementing or reviewing changes, preserve these rules:

- The client renders approved JSON UI components locally.
- The client must not execute node-provided code.
- The client must not print raw terminal escape sequences from nodes.
- The client owns trusted chrome such as connection status, identity, trust, and permission prompts.
- Doors run server-side under `phosphord`.
- Doors own behavior, not transport, trust, identity, or direct client rendering.
- Door state should flow through the SDK and typed effects, not arbitrary filesystem access.
- Keep the protocol small, typed, and strict. Do not turn it into a mini browser or generic DOM.

## Current MVP Reality Checks

These are easy places to make wrong assumptions. Verify against them before changing behavior:

- Lua is the default door runtime. Python is supported through the generic stdio runtime, but new statements should not describe Python as a privileged runtime model.
- Transport defaults to encrypted `wss://`, but node identity still comes from signed Ed25519 challenge verification plus known-node pinning.
- Manifest `capabilities = [...]` is the enforced capability policy. Legacy `permissions = [...]` is deprecated and only mapped for compatibility.
- `open_door` transitions work. Other declared transition kinds are still reserved future work.
- Presence is live and in-memory, not durable.
- The room model is implicit per door.
- `switchboard` is still scaffold-level, not a complete relay network.
- The file-level MVP roadmap is partly historical. Use `docs/PhosphorNet_todo.md` for the current loose-end backlog.

## Implementation Rules By Task Type

For docs or behavior questions:

- Read the relevant docs first.
- Inspect only the subsystem that owns the behavior before widening the search.
- Do not infer shipped behavior from old roadmap text when current docs or code say otherwise.

For protocol or auth work:

- Start in `internal/protocol/`, `internal/identity/`, and `internal/node/`.
- Prefer explicit typed structs and domain-separated signed payloads over loose maps or opaque blobs.
- Preserve mutual Ed25519 identity proof and known-node trust behavior.

For client work:

- Start in `internal/client/`.
- Preserve trusted chrome boundaries and sanitization limits.
- Do not let remote content bypass local focus, trust, or safety rules.

For node or runtime work:

- Start in `internal/node/`, `internal/runtime/`, and `internal/storage/`.
- Keep runtime behavior inspectable and boring.
- Prefer small typed changes over broad framework-like abstractions.

For door work:

- Read `docs/PhosphorNet_runtime_contract.md` and `docs/PhosphorNet_authoring_doors.md` before reading bundled door code.
- Keep doors lightweight and script-like.
- Do not introduce Flask, FastAPI, per-door HTTP servers, or client-side scripting.

For feature work:

- Check MVP priorities and non-goals before implementing.
- Keep changes aligned with the documented MVP unless the user explicitly asks to change direction.

For bug fixes:

- Reproduce or trace the smallest likely path first.
- Do not read the whole repo to fix a localized issue unless the local path proves insufficient.

## MVP Priorities And Non-Goals

The MVP should prove:

- trusted client rendering
- Ed25519 authentication
- JSON UI protocol
- node-hosted doors
- SQLite-backed state
- at least these doors: `lobby`, `chat`, `strategy_demo`

Avoid pulling the project toward these unless the user explicitly requests them:

- DHT discovery
- full federation
- offline mail
- public door marketplace
- browser client
- mobile client
- arbitrary client-side scripting
- local execution of downloaded doors
- advanced permissions systems
- Cloudflare-first architecture
- WASM-first runtime design
- gRPC-first transport design

## Security Rules

Never weaken these casually:

- no remote code execution on the client
- no raw terminal escape passthrough
- no silent clipboard, shell, or local file access
- no remote overwriting of trusted client chrome
- no automatic global key capture
- no arbitrary client-side network side effects

Also preserve client-side limits:

- message size limits
- UI tree depth limits
- text length limits
- render rate limits
- notification rate limits

## Change Tracking

`changelog.md` is required project documentation.

When making any meaningful repository change, always update `changelog.md` in the same turn.

This includes:

- source code changes
- new files
- deleted files
- changed behavior
- dependency changes
- migrations
- tests
- important generated project artifacts when they are a direct result of implementation or verification work

Do not leave implementation work undocumented.
